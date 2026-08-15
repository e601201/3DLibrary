// design/Design.pen 画面02 のビューポート。3D 表示の上に
// バッジ(左上)・ツール(右上)・モーションプレビュー(左上)・
// 再生タイムラインと操作ヒント(下中央)を重ねる。

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Camera,
  Film,
  Grid3x3,
  Maximize,
  Move,
  Rotate3d,
  RotateCw,
  SlidersHorizontal,
  ZoomIn,
} from 'lucide-react';
import type {
  AnimationAction,
  AnimationClip,
  AnimationMixer,
  BoxHelper,
  KeyframeTrack,
  Material,
  Mesh,
  Object3D,
  Quaternion,
  Side,
  SkinnedMesh,
  Texture,
  Vector3,
} from 'three';
import { formatSize } from './format';
import {
  MotionPanel,
  TimelineBar,
  timebaseOf,
  type Motion,
  type ShapeKey,
} from './MotionPreview';
import { OverlayChip, cx, type LucideIcon } from './ui';

type Props = {
  url: string; // GLB の配信 URL
  sizeBytes: number | null; // バッジに出す GLB のサイズ
  title: string; // スクリーンショットのファイル名に使う
  // 抽出メタデータのフレームレート。glTF は時間を秒でしか持たないので、
  // フレーム番号を出すにはこれが要る(旧キャッシュには無く、その場合は秒表示)
  frameRate: number | null;
};

// three 側へ命令を送るための最小インターフェース。
// three の初期化は url が変わったときだけ走らせ、
// グリッド切替などの UI 操作でシーンを作り直さない。
// 表示モードは「面の見え方」、ワイヤー重ねは「線を足すか」で軸を分ける。
// 「ワイヤー」は面が消えている状態なので、そこに線を重ねる意味はない
// (重ね設定は無効化するが値は保持し、面のあるモードに戻したら復活させる)
type ShadeMode = 'material' | 'clay' | 'wire';

// 背景: 単色 2 種と RoomEnvironment(背景+IBL)と透明(透過 PNG 用)
type BackgroundMode = 'light' | 'dark' | 'env' | 'transparent';

// クレイはマテリアルの差し替えで表現するので、メッシュごとに両方を控えておく
type ShadedMesh = { mesh: Mesh; original: Material | Material[]; clay: Material | Material[] };

// スライダー 1 本が動かすモーフターゲット。マルチマテリアルのオブジェクトは
// メッシュに分割され、同じシェイプキーがそれぞれに載るので、まとめて動かす
type ShapeKeyTargets = { mesh: Mesh; index: number }[];

// 再生中に three からタイムラインとスライダーへ値を返す間隔。
// 毎フレーム React を更新すると重いので、目に足りる程度に間引く
const MOTION_PUSH_MS = 33;

type ViewerApi = {
  setGrid: (visible: boolean) => void;
  setAutoRotate: (on: boolean) => void;
  setShade: (mode: ShadeMode, wireOverlay: boolean) => void;
  setBackground: (bg: BackgroundMode) => void;
  setExposure: (value: number) => void;
  snapshot: () => void;
  selectObject: (objectId: string | null) => void;
  selectClip: (index: number) => void;
  setPlaying: (on: boolean) => void;
  setLoop: (on: boolean) => void;
  seek: (seconds: number) => void;
  setInfluence: (index: number, value: number) => void;
  resetInfluences: () => void;
  syncInfluences: () => void;
};

// 回転 = 左ドラッグ、パン = SHIFT / Ctrl / Cmd + 左ドラッグ / 右ドラッグ、
// ズーム = ホイール / 中ドラッグ。いずれも OrbitControls の既定の割り当て。
// three は重いので動的 import で分割する。
export default function GlbViewer({ url, sizeBytes, title, frameRate }: Props) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const apiRef = useRef<ViewerApi | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [grid, setGrid] = useState(true);
  const [autoRotate, setAutoRotate] = useState(false);
  const [shadeMode, setShadeMode] = useState<ShadeMode>('material');
  const [wireOverlay, setWireOverlay] = useState(false);
  const [background, setBackground] = useState<BackgroundMode>('dark');
  const [exposure, setExposure] = useState(1);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [fps, setFps] = useState<number | null>(null);

  // モーションプレビュー。読み込んだ GLB にシェイプキーもクリップも
  // 無ければ motion は空のまま、UI も一切出さない
  const [motion, setMotion] = useState<Motion | null>(null);
  const [motionOpen, setMotionOpen] = useState(false);
  const [clipIndex, setClipIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [loop, setLoop] = useState(true);
  const [time, setTime] = useState(0);
  const [influences, setInfluences] = useState<number[]>([]);
  // オブジェクト選択。null は絞り込みなしで、モーションプレビューは全表示になる
  const [selectedObject, setSelectedObject] = useState<string | null>(null);

  // スクリーンショットのファイル名にしか使わないので、
  // 変わってもシーンを作り直さないよう ref で持つ
  const titleRef = useRef(title);
  titleRef.current = title;

  // 再生成で url が変わるとシーンを作り直すため、そのときも
  // 現在の表示設定を引き継げるよう ref に写しておく
  const gridRef = useRef(grid);
  gridRef.current = grid;
  const autoRotateRef = useRef(autoRotate);
  autoRotateRef.current = autoRotate;
  const shadeModeRef = useRef(shadeMode);
  shadeModeRef.current = shadeMode;
  const wireOverlayRef = useRef(wireOverlay);
  wireOverlayRef.current = wireOverlay;
  const backgroundRef = useRef(background);
  backgroundRef.current = background;
  const exposureRef = useRef(exposure);
  exposureRef.current = exposure;

  // 描画ループとクリップ生成から読む(表示設定とは違い、こちらは
  // シーンを作り直さずに毎フレーム参照される)
  const playingRef = useRef(playing);
  playingRef.current = playing;
  const loopRef = useRef(loop);
  loopRef.current = loop;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    // 再生成で GLB が入れ替わるとクリップ構成もシェイプキー構成も変わり得る
    // ので、モーションの状態は引き継がない(表示設定は上の ref で引き継ぐ)
    setMotion(null);
    setMotionOpen(false);
    setClipIndex(0);
    setPlaying(false);
    setLoop(true);
    setTime(0);
    setInfluences([]);
    setSelectedObject(null);

    let disposed = false;
    let cleanup: (() => void) | null = null;

    (async () => {
      const THREE = await import('three');
      const [{ GLTFLoader }, { OrbitControls }, { RoomEnvironment }, { ViewHelper }] =
        await Promise.all([
          import('three/addons/loaders/GLTFLoader.js'),
          import('three/addons/controls/OrbitControls.js'),
          import('three/addons/environments/RoomEnvironment.js'),
          import('three/addons/helpers/ViewHelper.js'),
        ]);
      if (disposed) return;

      const scene = new THREE.Scene();
      const camera = new THREE.PerspectiveCamera(
        45,
        container.clientWidth / container.clientHeight,
        0.01,
        1000,
      );
      const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
      renderer.setPixelRatio(window.devicePixelRatio);
      renderer.setSize(container.clientWidth, container.clientHeight);
      // 明るさは露出(トーンマッピング)で変える。Neutral はアセットの色を保つ
      renderer.toneMapping = THREE.NeutralToneMapping;
      renderer.toneMappingExposure = exposureRef.current;
      container.appendChild(renderer.domElement);

      const hemi = new THREE.HemisphereLight(0xffffff, 0x555566, 2.5);
      scene.add(hemi);
      const sun = new THREE.DirectionalLight(0xffffff, 2);
      sun.position.set(3, 5, 4);
      scene.add(sun);

      // 「環境」が初めて選ばれたときに一度だけ RoomEnvironment を焼いて使い回す
      let envMap: Texture | null = null;
      const ensureEnv = () => {
        if (!envMap) {
          const pmrem = new THREE.PMREMGenerator(renderer);
          const room = new RoomEnvironment();
          envMap = pmrem.fromScene(room, 0.04).texture;
          room.dispose();
          pmrem.dispose();
        }
        return envMap;
      };

      // 環境モードは IBL に照明を任せる(固定 2 灯を加算すると白飛びする)
      const applyBackground = (bg: BackgroundMode) => {
        const env = bg === 'env';
        hemi.visible = !env;
        sun.visible = !env;
        scene.environment = env ? ensureEnv() : null;
        scene.background =
          bg === 'light'
            ? new THREE.Color(0xf0f0f0)
            : bg === 'dark'
              ? new THREE.Color(0x0a0a0a)
              : env
                ? ensureEnv()
                : null; // 透明。alpha 付きキャンバスで CSS の市松模様が透ける
      };
      applyBackground(backgroundRef.current);

      // ワイヤーはジオメトリを共有する子メッシュとして重ねる(バッファ複製なし)。
      // 親メッシュを非表示にすると子のワイヤーごと消えるため、
      // 「ワイヤーのみ」では親側マテリアルの visible で面だけを消す
      const wireMaterial = new THREE.MeshBasicMaterial({ wireframe: true, color: 0x39ff14 });
      const baseMaterials: Material[] = [];
      const wireMeshes: Mesh[] = [];

      // クレイ表示はテクスチャもマテリアル色も外し、形だけを単色の陰影で見る。
      // 片面/両面(side)だけは引き継がないと、片面設定の薄い板が裏から抜けて
      // 「壊れたモデル」に見えてしまうため、side ごとに 1 つ作って共有する。
      // アルファ抜きは引き継がない(葉や金網はベタ板になる = Blender のソリッド相当)
      const clayMaterials = new Map<Side, Material>();
      const clayFor = (side: Side) => {
        let clay = clayMaterials.get(side);
        if (!clay) {
          clay = new THREE.MeshStandardMaterial({
            color: 0xb0b0b0, // どの背景でも輪郭が立つ明度の無彩色
            roughness: 0.75,
            metalness: 0,
            side,
            // 元マテリアルと同様、重ねたワイヤーとのチラつきを防ぐ
            polygonOffset: true,
            polygonOffsetFactor: 1,
            polygonOffsetUnits: 1,
          });
          clayMaterials.set(side, clay);
        }
        return clay;
      };

      const shadedMeshes: ShadedMesh[] = [];

      const applyShade = (mode: ShadeMode, wireOverlay: boolean) => {
        for (const e of shadedMeshes) e.mesh.material = mode === 'clay' ? e.clay : e.original;
        // 「ワイヤー」は元マテリアルに戻したうえで面だけを消す
        for (const m of baseMaterials) m.visible = mode !== 'wire';
        for (const w of wireMeshes) w.visible = mode === 'wire' || wireOverlay;
      };

      // デザインのビューポートはアクセント色のグリッド床が敷かれている
      const gridHelper = new THREE.GridHelper(10, 20, 0xa855f7, 0x3b2a4d);
      gridHelper.visible = gridRef.current;
      scene.add(gridHelper);

      const controls = new OrbitControls(camera, renderer.domElement);
      controls.enableDamping = true;
      controls.autoRotate = autoRotateRef.current;

      // 右下の XYZ ギズモ。軸クリックでカメラをその軸方向へスナップさせる
      const viewHelper = new ViewHelper(camera, renderer.domElement);
      viewHelper.center = controls.target; // パン後もスナップが注視点を向くよう参照を共有
      viewHelper.setLabels('X', 'Y', 'Z');

      // ViewHelper の描画サイズ(dim = 128)は閉包定数で変更できないため、
      // 描画時のビューポートとクリック判定座標を写像して 192px(1.5 倍)で運用する
      const GIZMO_DIM = 192;
      const origRender = viewHelper.render.bind(viewHelper);
      viewHelper.render = (r) => {
        const setViewport = r.setViewport.bind(r);
        r.setViewport = ((x: number, y: number, w: number, h: number) => {
          if (w === 128 && h === 128) {
            setViewport(r.domElement.offsetWidth - GIZMO_DIM, 0, GIZMO_DIM, GIZMO_DIM);
          } else {
            setViewport(x, y, w, h);
          }
        }) as typeof r.setViewport;
        origRender(r);
        r.setViewport = setViewport;
      };
      const origHandleClick = viewHelper.handleClick.bind(viewHelper);
      viewHelper.handleClick = (event) => {
        const dom = renderer.domElement;
        const rect = dom.getBoundingClientRect();
        const scale = 128 / GIZMO_DIM;
        const clientX =
          rect.left + dom.offsetWidth - 128 +
          (event.clientX - (rect.left + dom.offsetWidth - GIZMO_DIM)) * scale;
        const clientY =
          rect.top + dom.offsetHeight - 128 +
          (event.clientY - (rect.top + dom.offsetHeight - GIZMO_DIM)) * scale;
        return origHandleClick({ clientX, clientY } as PointerEvent);
      };

      // 自身から根までの並び。「自身か、その子孫か」の判定に何度も要る
      const ancestorsOf = (obj: Object3D) => {
        const chain: Object3D[] = [];
        for (let o: Object3D | null = obj; o; o = o.parent) chain.push(o);
        return chain;
      };

      // --- オブジェクト選択 ---
      // 選択単位は Blender のオブジェクト。glTF ではノードがそれに当たるが、
      // マルチマテリアルのメッシュは複数の子メッシュに分かれるので、レイキャストで
      // 当たったメッシュからノードまで遡ってから選ぶ。実体は読み込み後に埋まる
      let isObjectNode: (obj: Object3D) => boolean = () => false;
      const objectsById = new Map<string, Object3D>();
      let selectionBox: BoxHelper | null = null;
      let selectedId: string | null = null;

      const objectOf = (obj: Object3D): Object3D =>
        // ノードを辿れない GLB でも、選択そのものは成立させる
        ancestorsOf(obj).find(isObjectNode) ?? obj;

      // 選択中のオブジェクトをもう一度選ぶと解除。ビューポートの再クリックと
      // パネル見出しの再クリックを、ここ 1 か所で同じ挙動に揃える
      const select = (object: Object3D | null) => {
        const next = object && object.uuid !== selectedId ? object : null;
        selectedId = next?.uuid ?? null;
        if (selectionBox) {
          if (next) selectionBox.setFromObject(next);
          selectionBox.visible = next !== null;
        }
        setSelectedObject(selectedId);
      };

      const raycaster = new THREE.Raycaster();
      const pointer = new THREE.Vector2();
      // 何も当たらない空クリックは解除
      const pick = (e: PointerEvent) => {
        const rect = renderer.domElement.getBoundingClientRect();
        pointer.set(
          ((e.clientX - rect.left) / rect.width) * 2 - 1,
          -((e.clientY - rect.top) / rect.height) * 2 + 1,
        );
        raycaster.setFromCamera(pointer, camera);
        // 子として重ねたワイヤーは選択対象ではないので、面のメッシュだけを見る
        const hit = raycaster.intersectObjects(
          shadedMeshes.map((s) => s.mesh),
          false,
        )[0];
        select(hit ? objectOf(hit.object) : null);
      };

      // 軌道ドラッグ終了の pointerup で誤スナップ・誤選択しないよう、
      // ほぼ動いていないクリックだけをギズモと選択に渡す
      let downX = 0;
      let downY = 0;
      const onPointerDown = (e: PointerEvent) => {
        downX = e.clientX;
        downY = e.clientY;
      };
      const onPointerUp = (e: PointerEvent) => {
        if (Math.hypot(e.clientX - downX, e.clientY - downY) >= 4) return;
        // ギズモが受け取ったクリックはカメラ操作なので、選択には回さない
        if (viewHelper.handleClick(e)) return;
        if (e.button === 0) pick(e);
      };
      renderer.domElement.addEventListener('pointerdown', onPointerDown);
      renderer.domElement.addEventListener('pointerup', onPointerUp);

      // --- モーションプレビュー ---
      // 実体は読み込み後に埋まる。クリップは初回の再生・シークまで適用しない
      // ので、開いた直後はエクスポート時の姿勢のまま止まっている
      let mixer: AnimationMixer | null = null;
      let clips: AnimationClip[] = [];
      let active: AnimationAction | null = null;
      let activeIndex = -1; // 適用済みのクリップ(未適用は -1)
      let activeDrivesShapeKeys = false;
      let drivenByClip: boolean[][] = [];
      let defaultValues: number[] = [];
      let sliderValues: number[] = []; // スライダーの現在値(three 側の写し)
      // シェイプキーを three の morphTargetInfluences 上の位置へ結びつける
      const bindings: ShapeKeyTargets[] = [];
      const restPose: {
        obj: Object3D;
        position: Vector3;
        quaternion: Quaternion;
        scale: Vector3;
      }[] = [];

      // まとめた先は同じ値で動くので、読むのは代表の 1 つで足りる
      const readInfluences = () =>
        bindings.map(([first]) => first.mesh.morphTargetInfluences?.[first.index] ?? 0);

      const applyInfluence = (targets: ShapeKeyTargets, value: number) => {
        for (const { mesh, index } of targets) {
          if (mesh.morphTargetInfluences) mesh.morphTargetInfluences[index] = value;
        }
      };

      const applySliderValues = () => {
        for (let i = 0; i < bindings.length; i++) applyInfluence(bindings[i], sliderValues[i]);
      };

      // 前のクリップが動かしたボーンが取り残されないよう、読み込み直後の
      // 姿勢に戻してから差し替える
      const restoreRestPose = () => {
        for (const r of restPose) {
          r.obj.position.copy(r.position);
          r.obj.quaternion.copy(r.quaternion);
          r.obj.scale.copy(r.scale);
        }
      };

      // 対象を省くと、いま適用しているクリップ(まだ何も適用していなければ
      // 先頭)を使う。再生・シークはこの既定で足りる
      const ensureAction = (index = Math.max(activeIndex, 0)) => {
        if (!mixer || !clips[index]) return null;
        if (activeIndex === index && active) return active;
        mixer.stopAllAction();
        restoreRestPose();
        applySliderValues();
        active = mixer.clipAction(clips[index]);
        active.reset();
        active.clampWhenFinished = true; // ループ off では末尾の姿勢で止める
        active.loop = loopRef.current ? THREE.LoopRepeat : THREE.LoopOnce;
        active.play();
        active.paused = true;
        activeIndex = index;
        activeDrivesShapeKeys = drivenByClip[index]?.some(Boolean) ?? false;
        return active;
      };

      // ギズモを本体シーンの上に重ねるため、クリアは手動で行う
      renderer.autoClear = false;
      const clock = new THREE.Clock();
      let raf = 0;
      let frames = 0;
      let lastFpsAt = performance.now();
      let lastMotionPushAt = 0;
      const renderLoop = () => {
        raf = requestAnimationFrame(renderLoop);
        const delta = clock.getDelta();
        if (viewHelper.animating) viewHelper.update(delta);
        // 一時停止中は mixer を回さない。回すと、ユーザーがいじった
        // シェイプキーの値をクリップが毎フレーム上書きしてしまう
        if (mixer && playingRef.current) {
          mixer.update(delta);
          const now = performance.now();
          if (now - lastMotionPushAt >= MOTION_PUSH_MS) {
            lastMotionPushAt = now;
            if (active) setTime(active.time);
            // クリップがシェイプキーを動かすときだけ、スライダーを追従させる
            if (activeDrivesShapeKeys) setInfluences(readInfluences());
          }
        }
        // 選択枠は動いた先へ毎フレーム合わせる。止めていてもシークやコマ送りで
        // 姿勢は変わるので、再生中かどうかでは絞らない(枠が元にするのは素の
        // ジオメトリの範囲なので、スキンやモーフの変形までは追わない)
        if (selectionBox?.visible) selectionBox.update();
        controls.update();
        renderer.clear();
        renderer.render(scene, camera);
        viewHelper.render(renderer);
        frames++;
        const now = performance.now();
        if (now - lastFpsAt >= 500) {
          setFps(Math.round((frames * 1000) / (now - lastFpsAt)));
          frames = 0;
          lastFpsAt = now;
        }
      };

      new GLTFLoader().load(
        url,
        (gltf) => {
          if (disposed) return;
          scene.add(gltf.scene);
          // GLTFLoader は生成した three のオブジェクトと glTF 要素の対応を残す。
          // nodes を持つものが Blender のオブジェクトに当たるノード
          const associations = gltf.parser.associations;
          isObjectNode = (obj) => associations.get(obj)?.nodes !== undefined;
          gltf.scene.traverse((obj) => {
            if (isObjectNode(obj)) objectsById.set(obj.uuid, obj);
          });
          // 選択枠。モデルに埋もれると読めないので常に手前へ描く
          selectionBox = new THREE.BoxHelper(gltf.scene, 0xa855f7);
          selectionBox.visible = false;
          selectionBox.material.depthTest = false;
          selectionBox.renderOrder = 1;
          scene.add(selectionBox);
          // クリップ切替時に戻す基準姿勢(ワイヤーを足す前の素の状態)
          gltf.scene.traverse((obj) => {
            restPose.push({
              obj,
              position: obj.position.clone(),
              quaternion: obj.quaternion.clone(),
              scale: obj.scale.clone(),
            });
          });
          // バウンディングボックスに合わせてカメラとグリッドを配置する
          const box = new THREE.Box3().setFromObject(gltf.scene);
          const center = box.getCenter(new THREE.Vector3());
          const size = box.getSize(new THREE.Vector3());
          const radius = Math.max(size.length() / 2, 0.5);
          camera.position.copy(
            center.clone().add(new THREE.Vector3(1, 0.7, 1).normalize().multiplyScalar(radius * 2.5)),
          );
          camera.near = radius / 100;
          camera.far = radius * 100;
          camera.updateProjectionMatrix();
          controls.target.copy(center);
          controls.update();
          gridHelper.scale.setScalar((radius * 2.5) / 5);
          gridHelper.position.set(center.x, box.min.y, center.z);

          const meshes: Mesh[] = [];
          gltf.scene.traverse((obj) => {
            if ((obj as Mesh).isMesh) meshes.push(obj as Mesh);
          });
          for (const mesh of meshes) {
            const materials = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
            for (const m of materials) {
              // 重ねたワイヤーが面とチラつかないよう、面をわずかに奥へずらす
              m.polygonOffset = true;
              m.polygonOffsetFactor = 1;
              m.polygonOffsetUnits = 1;
              baseMaterials.push(m);
            }
            shadedMeshes.push({
              mesh,
              original: mesh.material,
              clay: Array.isArray(mesh.material)
                ? mesh.material.map((m) => clayFor(m.side))
                : clayFor(mesh.material.side),
            });
            // シェイプキーとスキンの変形にワイヤーも追従させる。モーフは
            // 影響値の配列をそのまま共有し、スキンは同じスケルトンに束ねる
            const skinned = mesh as SkinnedMesh;
            let wire: Mesh;
            if (skinned.isSkinnedMesh) {
              const skinnedWire = new THREE.SkinnedMesh(mesh.geometry, wireMaterial);
              skinnedWire.bind(skinned.skeleton, skinned.bindMatrix);
              wire = skinnedWire;
            } else {
              wire = new THREE.Mesh(mesh.geometry, wireMaterial);
            }
            wire.morphTargetInfluences = mesh.morphTargetInfluences;
            wire.morphTargetDictionary = mesh.morphTargetDictionary;
            mesh.add(wire);
            wireMeshes.push(wire);
          }
          applyShade(shadeModeRef.current, wireOverlayRef.current);

          // シェイプキー(GLB ではモーフターゲット)を、メッシュと添字に
          // 結びつけたうえで平らに並べる。マルチマテリアルのオブジェクトは
          // 分割後のメッシュそれぞれに同じキーが載るので、オブジェクトとキー名で
          // 1 本にまとめる(でないと同名のスライダーが並び、片方しか動かない)
          const shapeKeys: ShapeKey[] = [];
          const slots = new Map<string, number>(); // オブジェクトとキー名 -> shapeKeys 上の位置
          for (const mesh of meshes) {
            const values = mesh.morphTargetInfluences;
            const dictionary = mesh.morphTargetDictionary;
            if (!values || !dictionary) continue;
            const names: string[] = [];
            for (const [name, i] of Object.entries(dictionary)) names[i] = name;
            // 見出しはオブジェクト単位。GLTFLoader が名前から記号を落とすので、
            // 表示だけは Blender が付けた元の名前に戻す
            const object = objectOf(mesh);
            objectsById.set(object.uuid, object);
            const objectName =
              typeof object.userData.name === 'string'
                ? object.userData.name
                : object.name || 'Object';
            for (let i = 0; i < values.length; i++) {
              const name = names[i] ?? `Key ${i}`;
              const slot = `${object.uuid}\n${name}`;
              const at = slots.get(slot);
              if (at !== undefined) {
                bindings[at].push({ mesh, index: i });
                continue;
              }
              slots.set(slot, shapeKeys.length);
              bindings.push([{ mesh, index: i }]);
              shapeKeys.push({
                objectId: object.uuid,
                objectName,
                name,
                defaultValue: values[i],
              });
            }
          }
          defaultValues = shapeKeys.map((k) => k.defaultValue);
          sliderValues = [...defaultValues];

          clips = gltf.animations;
          if (clips.length > 0) {
            mixer = new THREE.AnimationMixer(gltf.scene);
            // ループ off でクリップが終わったら、再生ボタンを再生状態に戻す
            mixer.addEventListener('finished', () => {
              setPlaying(false);
              if (active) setTime(active.time);
            });
          }
          // トラック名は "<ノード名>.<プロパティ>" で、ノードは名前
          // (無名なら uuid)で指される。実体を引けるようにしておく
          const nodeByName = new Map<string, Object3D>();
          gltf.scene.traverse((obj) => nodeByName.set(obj.name || obj.uuid, obj));
          const parseTrack = (track: KeyframeTrack) => {
            const parsed = THREE.PropertyBinding.parseTrackName(track.name);
            return { node: nodeByName.get(parsed.nodeName) ?? null, property: parsed.propertyName };
          };

          // シェイプキーを動かすトラックは "<ノード名>.morphTargetInfluences"
          // で、1 本がそのノードのシェイプキーをまとめて駆動する
          drivenByClip = clips.map((clip) => {
            const nodes = new Set<Object3D>();
            for (const track of clip.tracks) {
              const { node, property } = parseTrack(track);
              if (node && property === 'morphTargetInfluences') nodes.add(node);
            }
            if (nodes.size === 0) return bindings.map(() => false);
            return bindings.map((targets) =>
              targets.some(({ mesh }) => ancestorsOf(mesh).some((o) => nodes.has(o))),
            );
          });

          // ボーンを動かすトラックは、そのスケルトンにスキンされたメッシュにも効く
          const skinnedByBone = new Map<Object3D, Mesh[]>();
          for (const mesh of meshes) {
            const skinned = mesh as SkinnedMesh;
            if (!skinned.isSkinnedMesh) continue;
            for (const bone of skinned.skeleton.bones) {
              const bound = skinnedByBone.get(bone);
              if (bound) bound.push(mesh);
              else skinnedByBone.set(bone, [mesh]);
            }
          }

          // オブジェクトに紐づくクリップ = そのオブジェクト自身・子孫・スキン先
          // スケルトンのボーンのいずれかを動かすクリップ。トラックの対象から
          // 祖先へさかのぼって印を付けると、子孫ぶんもまとめて拾える
          const clipsByObject: Record<string, boolean[]> = {};
          for (const id of objectsById.keys()) clipsByObject[id] = clips.map(() => false);
          clips.forEach((clip, clipIndex) => {
            for (const track of clip.tracks) {
              const { node } = parseTrack(track);
              if (!node) continue;
              for (const moved of [node, ...(skinnedByBone.get(node) ?? [])]) {
                for (const o of ancestorsOf(moved)) {
                  const related = clipsByObject[o.uuid];
                  if (related) related[clipIndex] = true;
                }
              }
            }
          });

          setMotion({
            clips: clips.map((c) => ({ name: c.name, duration: c.duration })),
            shapeKeys,
            drivenByClip,
            clipsByObject,
          });
          setInfluences([...defaultValues]);
        },
        undefined,
        () => setError('GLB を読み込めませんでした'),
      );

      const onResize = () => {
        camera.aspect = container.clientWidth / container.clientHeight;
        camera.updateProjectionMatrix();
        renderer.setSize(container.clientWidth, container.clientHeight);
      };
      const resizeObserver = new ResizeObserver(onResize);
      resizeObserver.observe(container);

      renderLoop();

      apiRef.current = {
        setGrid: (visible) => {
          gridHelper.visible = visible;
        },
        setAutoRotate: (on) => {
          controls.autoRotate = on;
        },
        setShade: applyShade,
        setBackground: applyBackground,
        setExposure: (value) => {
          renderer.toneMappingExposure = value;
        },
        // preserveDrawingBuffer を有効にしなくて済むよう、描画直後に読み出す。
        // ギズモ抜きで本体シーンだけを描き直してから読むので、PNG にギズモは写らない
        // 選択枠は画面上の目印でしかないので、撮る間だけ隠す
        snapshot: () => {
          const framed = selectionBox?.visible ?? false;
          if (selectionBox) selectionBox.visible = false;
          renderer.clear();
          renderer.render(scene, camera);
          const link = document.createElement('a');
          link.href = renderer.domElement.toDataURL('image/png');
          link.download = `${titleRef.current}.png`;
          link.click();
          if (selectionBox) selectionBox.visible = framed;
        },
        selectObject: (objectId) => {
          select(objectId === null ? null : (objectsById.get(objectId) ?? null));
        },
        selectClip: (index) => {
          const action = ensureAction(index);
          if (!action || !mixer) return;
          action.time = 0;
          mixer.update(0);
          setInfluences(readInfluences());
        },
        setPlaying: (on) => {
          const action = ensureAction();
          if (!action) return;
          // 末尾で止まっているところから再生するときは頭に戻す
          if (on && action.time >= action.getClip().duration - 1e-4) action.time = 0;
          action.paused = !on;
          if (!on) setInfluences(readInfluences());
        },
        setLoop: (on) => {
          if (active) active.loop = on ? THREE.LoopRepeat : THREE.LoopOnce;
        },
        seek: (seconds) => {
          const action = ensureAction();
          if (!action || !mixer) return;
          action.time = Math.max(0, Math.min(seconds, action.getClip().duration));
          // 再生中なら次のフレームで反映されるので、ここで回すのは停止中だけ
          if (action.paused) {
            mixer.update(0);
            setInfluences(readInfluences());
          }
        },
        setInfluence: (index, value) => {
          sliderValues[index] = value;
          applyInfluence(bindings[index] ?? [], value);
        },
        resetInfluences: () => {
          sliderValues = [...defaultValues];
          applySliderValues();
        },
        syncInfluences: () => setInfluences(readInfluences()),
      };

      cleanup = () => {
        apiRef.current = null;
        cancelAnimationFrame(raf);
        mixer?.stopAllAction();
        renderer.domElement.removeEventListener('pointerdown', onPointerDown);
        renderer.domElement.removeEventListener('pointerup', onPointerUp);
        resizeObserver.disconnect();
        viewHelper.dispose();
        controls.dispose();
        // GLTF のジオメトリ・マテリアルも解放する(GPU メモリリーク防止)
        scene.traverse((obj) => {
          const mesh = obj as Mesh;
          if (mesh.geometry) mesh.geometry.dispose();
          const materials: (Material | undefined)[] = Array.isArray(mesh.material)
            ? mesh.material
            : [mesh.material];
          for (const m of materials) m?.dispose();
        });
        // クレイ表示のままアンマウントすると上の traverse は差し替え後のマテリアルしか
        // 見ないので、元マテリアルとクレイの両方を明示的に解放する
        for (const m of baseMaterials) m.dispose();
        for (const clay of clayMaterials.values()) clay.dispose();
        envMap?.dispose();
        renderer.dispose();
        container.removeChild(renderer.domElement);
      };
    })().catch(() => setError('ビューアを初期化できませんでした'));

    return () => {
      disposed = true;
      cleanup?.();
    };
  }, [url]);

  useEffect(() => {
    apiRef.current?.setGrid(grid);
  }, [grid]);

  useEffect(() => {
    apiRef.current?.setAutoRotate(autoRotate);
  }, [autoRotate]);

  useEffect(() => {
    apiRef.current?.setShade(shadeMode, wireOverlay);
  }, [shadeMode, wireOverlay]);

  useEffect(() => {
    apiRef.current?.setBackground(background);
  }, [background]);

  useEffect(() => {
    apiRef.current?.setExposure(exposure);
  }, [exposure]);

  useEffect(() => {
    apiRef.current?.setLoop(loop);
  }, [loop]);

  // 閉じている間に再生で動いたぶんを取り込んでから見せる
  useEffect(() => {
    if (motionOpen) apiRef.current?.syncInfluences();
  }, [motionOpen]);

  // ESC でも絞り込みを解除できるようにする。ただし全画面中の ESC は全画面を
  // 閉じる操作なので、そちらに譲って選択は残す
  useEffect(() => {
    if (selectedObject === null) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.fullscreenElement) apiRef.current?.selectObject(null);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selectedObject]);

  const timebase = useMemo(() => timebaseOf(frameRate), [frameRate]);
  const activeClip = motion?.clips[clipIndex] ?? null;
  const duration = activeClip?.duration ?? 0;
  const hasMotion = motion !== null && (motion.clips.length > 0 || motion.shapeKeys.length > 0);

  const togglePlay = () => {
    const next = !playing;
    setPlaying(next);
    apiRef.current?.setPlaying(next);
  };

  const selectClip = (index: number) => {
    if (index === clipIndex) {
      togglePlay();
      return;
    }
    setClipIndex(index);
    setTime(0);
    setPlaying(true);
    apiRef.current?.selectClip(index);
    apiRef.current?.setPlaying(true);
  };

  const seek = (seconds: number) => {
    setTime(seconds);
    apiRef.current?.seek(seconds);
  };

  const stepFrame = (frames: number) => {
    setPlaying(false);
    apiRef.current?.setPlaying(false);
    seek(Math.max(0, Math.min(timebase.step(time, frames), duration)));
  };

  const changeInfluence = (index: number, value: number) => {
    setInfluences((prev) => {
      const next = [...prev];
      next[index] = value;
      return next;
    });
    apiRef.current?.setInfluence(index, value);
  };

  const resetInfluences = () => {
    if (!motion) return;
    setInfluences(motion.shapeKeys.map((k) => k.defaultValue));
    apiRef.current?.resetInfluences();
  };

  const toggleFullscreen = () => {
    const wrapper = wrapperRef.current;
    if (!wrapper) return;
    if (document.fullscreenElement) {
      void document.exitFullscreen();
    } else {
      void wrapper.requestFullscreen();
    }
  };

  return (
    <div
      ref={wrapperRef}
      className="relative h-full w-full overflow-hidden bg-stage"
      // 透明背景では画像編集ソフト式の市松模様を敷き、透過 PNG になることを示す
      style={
        background === 'transparent'
          ? {
              backgroundImage: 'repeating-conic-gradient(#1e1e22 0% 25%, #2d2d33 0% 50%)',
              backgroundSize: '16px 16px',
            }
          : undefined
      }
    >
      <div ref={containerRef} className="h-full w-full" />

      <div className="pointer-events-none absolute inset-0 flex flex-col justify-between gap-3 p-4">
        <div className="flex min-h-0 flex-col items-start gap-2">
          <div className="flex w-full items-start justify-between gap-3">
            <OverlayChip>
              <span className="size-1.5 shrink-0 rounded-full bg-stage-ok" />
              GLB PREVIEW
              {sizeBytes !== null && ` · ${formatSize(sizeBytes)}`}
              {fps !== null && ` · ${fps} FPS`}
            </OverlayChip>
            <div className="pointer-events-auto relative">
              <div className="flex gap-1">
                <ViewportTool
                  icon={Grid3x3}
                  label="グリッドの表示切替"
                  active={grid}
                  onClick={() => setGrid((v) => !v)}
                />
                <ViewportTool
                  icon={RotateCw}
                  label="自動回転の切替"
                  active={autoRotate}
                  onClick={() => setAutoRotate((v) => !v)}
                />
                {/* シェイプキーもクリップも無い GLB では出さない */}
                {hasMotion && (
                  <ViewportTool
                    icon={Film}
                    label="モーションプレビュー"
                    active={motionOpen}
                    onClick={() => setMotionOpen((v) => !v)}
                  />
                )}
                <ViewportTool
                  icon={SlidersHorizontal}
                  label="表示設定"
                  active={settingsOpen}
                  onClick={() => setSettingsOpen((v) => !v)}
                />
                <ViewportTool
                  icon={Camera}
                  label="スクリーンショットを保存"
                  onClick={() => apiRef.current?.snapshot()}
                />
                <ViewportTool icon={Maximize} label="全画面表示" onClick={toggleFullscreen} />
              </div>
              {settingsOpen && (
                <ViewerSettings
                  shadeMode={shadeMode}
                  wireOverlay={wireOverlay}
                  background={background}
                  exposure={exposure}
                  onShadeMode={setShadeMode}
                  onWireOverlay={setWireOverlay}
                  onBackground={setBackground}
                  onExposure={setExposure}
                />
              )}
            </div>
          </div>
          {motionOpen && motion && (
            <MotionPanel
              motion={motion}
              influences={influences}
              driven={motion.drivenByClip[clipIndex] ?? []}
              clipIndex={clipIndex}
              playing={playing}
              selectedObjectId={selectedObject}
              onSelectObject={(objectId) => apiRef.current?.selectObject(objectId)}
              onSelectClip={selectClip}
              onInfluence={changeInfluence}
              onReset={resetInfluences}
            />
          )}
        </div>

        <div className="flex flex-col items-center gap-2">
          {activeClip && (
            <TimelineBar
              clipName={activeClip.name}
              time={time}
              duration={duration}
              playing={playing}
              loop={loop}
              timebase={timebase}
              onTogglePlay={togglePlay}
              onRewind={() => seek(0)}
              onStep={stepFrame}
              onSeek={seek}
              onToggleLoop={() => setLoop((v) => !v)}
            />
          )}
          <div className="flex justify-center gap-2">
            <OverlayChip>
              <Rotate3d size={13} className="text-stage-accent" />
              回転 · ドラッグ
            </OverlayChip>
            <OverlayChip>
              <Move size={13} className="text-stage-accent" />
              パン · SHIFT+ドラッグ
            </OverlayChip>
            <OverlayChip>
              <ZoomIn size={13} className="text-stage-accent" />
              ズーム · スクロール
            </OverlayChip>
          </div>
        </div>
      </div>

      {error && (
        <p className="absolute inset-0 flex items-center justify-center text-[13px] text-stage-danger">
          {error}
        </p>
      )}
    </div>
  );
}

// ビューポート右上のツールボタン(padding 7 / fill #0A0A0ACC + border)
function ViewportTool({
  icon: Icon,
  label,
  active = false,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      className={cx(
        'border bg-stage/80 p-[7px] backdrop-blur-sm transition',
        active
          ? 'border-stage-accent text-stage-accent'
          : 'border-stage-border text-stage-ink-muted hover:border-stage-ink-faint hover:text-stage-ink',
      )}
    >
      <Icon size={14} />
    </button>
  );
}

// 表示設定のポップオーバー。ビューポートに重なるので配色は stage-* 固定
function ViewerSettings({
  shadeMode,
  wireOverlay,
  background,
  exposure,
  onShadeMode,
  onWireOverlay,
  onBackground,
  onExposure,
}: {
  shadeMode: ShadeMode;
  wireOverlay: boolean;
  background: BackgroundMode;
  exposure: number;
  onShadeMode: (mode: ShadeMode) => void;
  onWireOverlay: (on: boolean) => void;
  onBackground: (bg: BackgroundMode) => void;
  onExposure: (value: number) => void;
}) {
  return (
    <div className="absolute right-0 top-full mt-1 flex w-64 flex-col gap-3 border border-stage-border bg-stage/90 p-3 backdrop-blur-sm">
      <div className="flex flex-col gap-1.5">
        <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
          表示モード
        </p>
        <StageSegmented
          value={shadeMode}
          onChange={onShadeMode}
          label="表示モード"
          options={[
            { value: 'material', label: 'マテリアル' },
            { value: 'clay', label: 'クレイ' },
            { value: 'wire', label: 'ワイヤー' },
          ]}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
          ワイヤー重ね
        </p>
        <StageSegmented
          value={wireOverlay ? 'on' : 'off'}
          onChange={(v) => onWireOverlay(v === 'on')}
          label="ワイヤー重ね"
          // 面が消えている「ワイヤー」では重ねる対象がないので選ばせない
          disabled={shadeMode === 'wire'}
          options={[
            { value: 'off', label: 'オフ' },
            { value: 'on', label: 'オン' },
          ]}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
          背景
        </p>
        <StageSegmented
          value={background}
          onChange={onBackground}
          label="背景"
          options={[
            { value: 'light', label: 'ライト' },
            { value: 'dark', label: 'ダーク' },
            { value: 'env', label: '環境' },
            { value: 'transparent', label: '透明' },
          ]}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <div className="flex items-baseline justify-between">
          <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
            明るさ
          </p>
          <p className="font-mono text-[10px] leading-none text-stage-ink-muted">
            {exposure.toFixed(2)}
          </p>
        </div>
        <input
          type="range"
          min={0}
          max={2}
          step={0.05}
          value={exposure}
          aria-label="明るさ"
          onChange={(e) => onExposure(Number(e.target.value))}
          className="w-full accent-stage-accent"
        />
      </div>
    </div>
  );
}

// ui.tsx の Segmented のビューポート用(stage-* 配色・小型)
function StageSegmented<T extends string>({
  value,
  options,
  onChange,
  label,
  disabled = false,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <div
      // 無効時も選択中の値は薄く見せ、戻したときに何が復活するか分かるようにする
      className={cx('flex border border-stage-border', disabled && 'opacity-40')}
      role="group"
      aria-label={label}
    >
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            disabled={disabled}
            onClick={() => onChange(o.value)}
            className={cx(
              'flex-1 px-1 py-[6px] text-[11px] leading-none transition',
              active
                ? 'bg-stage-accent/15 font-semibold text-stage-accent'
                : 'text-stage-ink-muted',
              disabled ? 'cursor-not-allowed' : !active && 'hover:text-stage-ink',
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
