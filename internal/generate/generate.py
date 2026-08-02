# 3DLibrary 生成スクリプト。
# 1 回の Blender 起動で GLB・サムネイル・抽出メタデータ・スプライトの
# 4 点を書き出す(requirements.md §7 生成、ADR-0003)。呼び出し:
#   blender -b <model.blend> --factory-startup --python generate.py -- \
#     --glb <out.glb> --thumb <out.png> --meta <out.json> \
#     --sprite <out.webp> --size <px>
import json
import math
import os
import shutil
import struct
import sys
import tempfile

import bpy
import mathutils
import numpy

# Blender の Material Preview が既定で使うスタジオライト。同じ HDRI を
# world に組むことでビューポートの見た目をヘッドレスで再現する
# (--background では bpy.ops.render.opengl が使えないため)。
STUDIO_HDRI = "forest.exr"

# EEVEE の識別子はバージョンで変わる(4.2〜4.5 系は BLENDER_EEVEE_NEXT)。
EEVEE_ENGINES = ("BLENDER_EEVEE", "BLENDER_EEVEE_NEXT")

# カメラの向き。サムネイルもスプライトもこの向きを基準に据える
# (スプライトはこれをワールド Z 軸まわりに回した各角度)。
CAMERA_DIRECTION = mathutils.Vector((1.0, -1.0, 0.7)).normalized()

# スプライトのシート形状(PRD hover-scrub-preview)。フレーム 0 は
# サムネイルと同一角度で、そこから 7.5° 刻みに一周する。
# フレームサイズは thumbnailSize 設定に連動させず 512px 固定
# (デコード後メモリの上限を守るため)。
SPRITE_FRAMES = 48
SPRITE_COLS = 8
SPRITE_ROWS = 6
SPRITE_FRAME_PX = 512
SPRITE_QUALITY = 80


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :]
    args = {}
    for i in range(0, len(argv), 2):
        args[argv[i].lstrip("-")] = argv[i + 1]
    return args


def extract_metadata():
    images = [i for i in bpy.data.images if i.name not in ("Render Result", "Viewer Node")]
    return {
        "objectCount": len(bpy.data.objects),
        "collectionCount": len(bpy.data.collections),
        "materialCount": len(bpy.data.materials),
        "textureCount": len(images),
        "hasAnimation": bool(bpy.data.actions),
    }


def glb_polygon_count(path):
    """書き出した GLB に入っている三角形の数。.blend のベースメッシュでは
    なくエクスポート結果を数えるので、モディファイアーの適用・三角形分割・
    インスタンスの展開まで含めてビューワーの表示内容と一致する。"""
    with open(path, "rb") as f:
        data = f.read()
    # GLB ヘッダ 12 バイトの直後が JSON チャンク(長さ 4 + 種別 4 + 本体)
    json_length = struct.unpack("<I", data[12:16])[0]
    gltf = json.loads(data[20 : 20 + json_length])
    accessors = gltf.get("accessors", [])

    per_mesh = []
    for mesh in gltf.get("meshes", []):
        triangles = 0
        for prim in mesh.get("primitives", []):
            if prim.get("mode", 4) != 4:  # 4 = TRIANGLES(省略時の既定値)
                continue
            index = prim.get("indices")
            count = accessors[index]["count"] if index is not None else accessors[prim["attributes"]["POSITION"]]["count"]
            triangles += count // 3
        per_mesh.append(triangles)
    # 同じメッシュを複数ノードが参照している(インスタンス)ぶんも数える
    return sum(per_mesh[n["mesh"]] for n in gltf.get("nodes", []) if "mesh" in n)


def unresolved_images():
    """ファイル実体が見つからない画像テクスチャの名前一覧。"""
    names = []
    for img in bpy.data.images:
        if img.source not in {"FILE", "SEQUENCE", "MOVIE"} or img.packed_file:
            continue
        # 誰も参照していない残骸でソリッドに落とさない
        if img.users == 0:
            continue
        path = bpy.path.abspath(img.filepath, library=img.library)
        if not path or not os.path.exists(path):
            names.append(img.name)
    return names


def resolve_missing_textures():
    """未解決テクスチャをアセットディレクトリから探して貼り直し、それでも
    見つからなかったものの名前を返す。未解決のまま EEVEE でレンダーすると
    マゼンタ一色になるので、シェーディング選択の判断材料にする。"""
    if not unresolved_images():
        return []
    asset_dir = os.path.dirname(bpy.data.filepath)
    if asset_dir:
        try:
            # model.blend と同じアセットディレクトリ(textures/ 等)を再帰探索
            bpy.ops.file.find_missing_files(directory=asset_dir)
        except RuntimeError as e:
            print("3dlibrary: find_missing_files failed: %s" % e)
    return unresolved_images()


def apply_render_modifier_settings():
    """モディファイアーをレンダー時の設定に揃える。glTF エクスポータは
    ビューポート側の評価結果を書き出すため、表示を軽くするために落として
    ある設定(Subdivision のビューポートレベル等)がそのまま GLB に出て
    しまう。ビューポートでのみ切ってあるモディファイアーも有効に戻す。"""
    for obj in bpy.data.objects:
        for mod in obj.modifiers:
            mod.show_viewport = mod.show_render
            if mod.type in {"SUBSURF", "MULTIRES"}:
                mod.levels = mod.render_levels


def scene_bounds(scene):
    """全ジオメトリを包む球(バウンディング中心と半径)。"""
    corners = []
    for obj in scene.objects:
        if obj.type in {"MESH", "CURVE", "SURFACE", "META", "FONT"}:
            corners.extend(obj.matrix_world @ mathutils.Vector(c) for c in obj.bound_box)
    if not corners:
        return mathutils.Vector((0.0, 0.0, 0.0)), 1.0
    center = sum(corners, mathutils.Vector()) / len(corners)
    return center, max((c - center).length for c in corners) or 1.0


def frame_camera(scene, center, radius):
    """全ジオメトリが収まる位置に専用カメラを置く(既存カメラには頼らない)。
    角度違いのレンダーはこのカメラを aim_camera で回して撮る。"""
    cam_data = bpy.data.cameras.new("3dlibrary_thumbnail_camera")
    cam = bpy.data.objects.new("3dlibrary_thumbnail_camera", cam_data)
    scene.collection.objects.link(cam)
    cam_data.clip_start = max(0.001, radius * 0.01)
    cam_data.clip_end = max(100.0, radius * 100)
    scene.camera = cam
    aim_camera(cam, center, radius, 0.0)
    return cam


def aim_camera(cam, center, radius, angle):
    """バウンディング中心を注視する位置へカメラを置く。angle はワールド Z 軸
    まわりの回転(ラジアン)で、0 がサムネイルの角度。仰角と距離は角度に
    よらず一定なので、一周させても構図が揺れない。"""
    direction = mathutils.Matrix.Rotation(angle, 3, "Z") @ CAMERA_DIRECTION
    # 既定 FOV(約 40°)で全体が収まるのは半径の約 3 倍だが、大きく表示したいので2.5倍にする
    cam.location = center + direction * radius * 2.5
    cam.rotation_euler = (center - cam.location).to_track_quat("-Z", "Y").to_euler()


def use_eevee(scene):
    for engine in EEVEE_ENGINES:
        try:
            scene.render.engine = engine
            return
        except TypeError:
            continue
    raise RuntimeError("no EEVEE engine in this Blender build")


def setup_material_preview(scene):
    """Material Preview 相当のシェーディングを組む。.blend 側の world や
    ライトには依存しない(ライトの無い .blend でも暗くならない)。"""
    hdri = os.path.join(bpy.utils.system_resource("DATAFILES"), "studiolights", "world", STUDIO_HDRI)
    world = bpy.data.worlds.new("3dlibrary_studio")
    world.use_nodes = True
    nodes, links = world.node_tree.nodes, world.node_tree.links
    nodes.clear()
    env = nodes.new("ShaderNodeTexEnvironment")
    env.image = bpy.data.images.load(hdri)
    background = nodes.new("ShaderNodeBackground")
    output = nodes.new("ShaderNodeOutputWorld")
    links.new(env.outputs["Color"], background.inputs["Color"])
    links.new(background.outputs["Background"], output.inputs["Surface"])
    scene.world = world

    use_eevee(scene)
    scene.eevee.taa_render_samples = 16
    # ライブラリ内で明るさ・彩度を揃える(.blend ごとの設定に左右されない)
    scene.view_settings.view_transform = "AgX"


def render_thumbnail(path, size, allow_material):
    """サムネイルを書き出し、使ったシェーディング("material"/"solid")を返す。
    ここで決まったシェーディングはスプライトのレンダーにも引き継がれる。"""
    scene = bpy.context.scene
    scene.render.resolution_x = size
    scene.render.resolution_y = size
    scene.render.resolution_percentage = 100
    scene.render.film_transparent = True
    scene.render.image_settings.file_format = "PNG"
    scene.render.filepath = path

    if allow_material:
        try:
            setup_material_preview(scene)
            bpy.ops.render.render(write_still=True)
            return "material"
        except Exception as e:
            # GPU 無し・EEVEE 非対応ビルド等。原因を問わずソリッドで確実に出す
            print("3dlibrary: material preview failed (%s), falling back to solid" % e)

    # WORKBENCH はライト不要・高速で、ライトの無い .blend でも確実に映る
    scene.render.engine = "BLENDER_WORKBENCH"
    bpy.ops.render.render(write_still=True)
    return "solid"


def render_frames(cam, center, radius, out_dir):
    """全周 48 フレームを 512px の PNG として out_dir へ書き出し、パスを順に返す。
    Render Result は画素を直接読めないので、いったんファイルへ落としてから
    読み直す(48 枚 × 512px なので一時ファイルの負担は小さい)。"""
    scene = bpy.context.scene
    scene.render.resolution_x = SPRITE_FRAME_PX
    scene.render.resolution_y = SPRITE_FRAME_PX
    scene.render.resolution_percentage = 100
    scene.render.film_transparent = True
    scene.render.use_file_extension = True
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"

    paths = []
    for i in range(SPRITE_FRAMES):
        aim_camera(cam, center, radius, i * 2.0 * math.pi / SPRITE_FRAMES)
        path = os.path.join(out_dir, "frame%02d.png" % i)
        scene.render.filepath = path
        bpy.ops.render.render(write_still=True)
        paths.append(path)
    return paths


def compose_sheet(paths):
    """フレーム画像を 8 列 × 6 行・行優先の 1 枚へ敷き詰めた画素配列を返す。
    合成は Blender 同梱の numpy だけで行う(Pillow 等の外部依存は増やさない)。"""
    width = SPRITE_COLS * SPRITE_FRAME_PX
    height = SPRITE_ROWS * SPRITE_FRAME_PX
    sheet = numpy.zeros((height, width, 4), dtype=numpy.float32)
    frame = numpy.empty(SPRITE_FRAME_PX * SPRITE_FRAME_PX * 4, dtype=numpy.float32)

    for i, path in enumerate(paths):
        image = bpy.data.images.load(path)
        try:
            image.pixels.foreach_get(frame)
        finally:
            bpy.data.images.remove(image)
        row, col = divmod(i, SPRITE_COLS)
        # Blender の画素は下から上へ並ぶ。左上を frame 0 にするため、
        # 上から row 番目は下から数えて (ROWS-1-row) 番目のブロックへ置く
        y = (SPRITE_ROWS - 1 - row) * SPRITE_FRAME_PX
        x = col * SPRITE_FRAME_PX
        sheet[y : y + SPRITE_FRAME_PX, x : x + SPRITE_FRAME_PX] = frame.reshape(
            SPRITE_FRAME_PX, SPRITE_FRAME_PX, 4
        )
    return sheet


def save_webp(sheet, path):
    """合成済みの画素配列を背景透過の WebP として書き出す。
    8bit バッファの画像に載せて保存するので、フレーム PNG の見た目
    (AgX 適用済み)がそのまま保たれる。"""
    scene = bpy.context.scene
    scene.render.image_settings.file_format = "WEBP"
    scene.render.image_settings.color_mode = "RGBA"
    scene.render.image_settings.quality = SPRITE_QUALITY

    height, width, _ = sheet.shape
    image = bpy.data.images.new("3dlibrary_sprite", width=width, height=height, alpha=True)
    try:
        image.pixels.foreach_set(sheet.ravel())
        image.save_render(filepath=path, scene=scene)
    finally:
        bpy.data.images.remove(image)


def render_sprite(path, cam, center, radius):
    """スプライト(全周 48 フレームを 1 枚に敷き詰めた WebP)を書き出す。
    シェーディングはサムネイルの続きをそのまま使うので、フレーム 0 は
    サムネイルと同一の画になる(ホバー開始時に画像が飛ばないための要)。"""
    out_dir = tempfile.mkdtemp(prefix="3dlibrary-sprite-")
    try:
        paths = render_frames(cam, center, radius, out_dir)
        sheet = compose_sheet(paths)
    finally:
        shutil.rmtree(out_dir, ignore_errors=True)
    save_webp(sheet, path)


def main():
    args = parse_args()
    meta = extract_metadata()
    # テクスチャの貼り直しは GLB エクスポートより前に(GLB の埋め込みにも効く)
    missing = resolve_missing_textures()
    if missing:
        print("3dlibrary: unresolved textures: %s" % ", ".join(missing))
    # モディファイアーを適用した結果を書き出す(Armature は骨として残るよう
    # エクスポータが自動で除外する)。シェイプキーは適用結果と両立しないため
    # 出力されない
    apply_render_modifier_settings()
    bpy.ops.export_scene.gltf(filepath=args["glb"], export_format="GLB", export_apply=True)
    # ポリゴン数は書き出した GLB から数える(モディファイアー適用後の実体)
    meta["polygonCount"] = glb_polygon_count(args["glb"])

    scene = bpy.context.scene
    center, radius = scene_bounds(scene)
    cam = frame_camera(scene, center, radius)
    meta["thumbnailShading"] = render_thumbnail(args["thumb"], int(args["size"]), not missing)
    render_sprite(args["sprite"], cam, center, radius)
    # 抽出メタデータ JSON は最後に書く(成功マーカーを兼ねる)
    with open(args["meta"], "w", encoding="utf-8") as f:
        json.dump(meta, f)


main()
