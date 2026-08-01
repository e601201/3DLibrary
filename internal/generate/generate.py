# 3DLibrary 生成スクリプト。
# 1 回の Blender 起動で GLB・サムネイル・抽出メタデータの 3 点を書き出す
# (requirements.md §7 生成)。呼び出し:
#   blender -b <model.blend> --factory-startup --python generate.py -- \
#     --glb <out.glb> --thumb <out.png> --meta <out.json> --size <px>
import json
import os
import struct
import sys

import bpy
import mathutils

# Blender の Material Preview が既定で使うスタジオライト。同じ HDRI を
# world に組むことでビューポートの見た目をヘッドレスで再現する
# (--background では bpy.ops.render.opengl が使えないため)。
STUDIO_HDRI = "forest.exr"

# EEVEE の識別子はバージョンで変わる(4.2〜4.5 系は BLENDER_EEVEE_NEXT)。
EEVEE_ENGINES = ("BLENDER_EEVEE", "BLENDER_EEVEE_NEXT")


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


def frame_camera(scene):
    """全ジオメトリが収まる位置に専用カメラを置く(既存カメラには頼らない)。"""
    corners = []
    for obj in scene.objects:
        if obj.type in {"MESH", "CURVE", "SURFACE", "META", "FONT"}:
            corners.extend(obj.matrix_world @ mathutils.Vector(c) for c in obj.bound_box)
    if corners:
        center = sum(corners, mathutils.Vector()) / len(corners)
        radius = max((c - center).length for c in corners) or 1.0
    else:
        center, radius = mathutils.Vector((0.0, 0.0, 0.0)), 1.0

    cam_data = bpy.data.cameras.new("3dlibrary_thumbnail_camera")
    cam = bpy.data.objects.new("3dlibrary_thumbnail_camera", cam_data)
    scene.collection.objects.link(cam)
    direction = mathutils.Vector((1.0, -1.0, 0.7)).normalized()
    # 既定 FOV(約 40°)で全体が収まるのは半径の約 3 倍だが、大きく表示したいので2.5倍にする
    cam.location = center + direction * radius * 2.5
    cam.rotation_euler = (center - cam.location).to_track_quat("-Z", "Y").to_euler()
    cam_data.clip_start = max(0.001, radius * 0.01)
    cam_data.clip_end = max(100.0, radius * 100)
    scene.camera = cam


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
    """サムネイルを書き出し、使ったシェーディング("material"/"solid")を返す。"""
    scene = bpy.context.scene
    frame_camera(scene)
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
    meta["thumbnailShading"] = render_thumbnail(args["thumb"], int(args["size"]), not missing)
    # 抽出メタデータ JSON は最後に書く(成功マーカーを兼ねる)
    with open(args["meta"], "w", encoding="utf-8") as f:
        json.dump(meta, f)


main()
