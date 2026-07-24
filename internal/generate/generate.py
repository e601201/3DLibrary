# 3DLibrary 生成スクリプト。
# 1 回の Blender 起動で GLB・サムネイル・抽出メタデータの 3 点を書き出す
# (requirements.md §7 生成)。呼び出し:
#   blender -b <model.blend> --factory-startup --python generate.py -- \
#     --glb <out.glb> --thumb <out.png> --meta <out.json> --size <px>
import json
import sys

import bpy
import mathutils


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :]
    args = {}
    for i in range(0, len(argv), 2):
        args[argv[i].lstrip("-")] = argv[i + 1]
    return args


def extract_metadata():
    polygons = sum(len(m.polygons) for m in bpy.data.meshes)
    images = [i for i in bpy.data.images if i.name not in ("Render Result", "Viewer Node")]
    return {
        "objectCount": len(bpy.data.objects),
        "collectionCount": len(bpy.data.collections),
        "materialCount": len(bpy.data.materials),
        "polygonCount": polygons,
        "textureCount": len(images),
        "hasAnimation": bool(bpy.data.actions),
    }


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
    # 既定 FOV(約 40°)で全体が収まるのは半径の約 3 倍から
    cam.location = center + direction * radius * 3.0
    cam.rotation_euler = (center - cam.location).to_track_quat("-Z", "Y").to_euler()
    cam_data.clip_start = max(0.001, radius * 0.01)
    cam_data.clip_end = max(100.0, radius * 100)
    scene.camera = cam


def render_thumbnail(path, size):
    scene = bpy.context.scene
    frame_camera(scene)
    # WORKBENCH はライト不要・高速で、ライトの無い .blend でも確実に映る
    scene.render.engine = "BLENDER_WORKBENCH"
    scene.render.resolution_x = size
    scene.render.resolution_y = size
    scene.render.resolution_percentage = 100
    scene.render.film_transparent = True
    scene.render.image_settings.file_format = "PNG"
    scene.render.filepath = path
    bpy.ops.render.render(write_still=True)


def main():
    args = parse_args()
    meta = extract_metadata()
    bpy.ops.export_scene.gltf(filepath=args["glb"], export_format="GLB")
    render_thumbnail(args["thumb"], int(args["size"]))
    # 抽出メタデータ JSON は最後に書く(成功マーカーを兼ねる)
    with open(args["meta"], "w", encoding="utf-8") as f:
        json.dump(meta, f)


main()
