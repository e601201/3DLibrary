// design/Design.pen 画面02 のビューポート。3D 表示の上に
// バッジ(左上)・ツール(右上)・操作ヒント(下中央)を重ねる。

import { useEffect, useRef, useState } from 'react';
import {
  Camera,
  Grid3x3,
  Maximize,
  Move,
  Rotate3d,
  RotateCw,
  SlidersHorizontal,
  ZoomIn,
} from 'lucide-react';
import type { Material, Mesh, Texture } from 'three';
import { formatSize } from './format';
import { OverlayChip, cx, type LucideIcon } from './ui';

type Props = {
  url: string; // GLB の配信 URL
  sizeBytes: number | null; // バッジに出す GLB のサイズ
  title: string; // スクリーンショットのファイル名に使う
};

// three 側へ命令を送るための最小インターフェース。
// three の初期化は url が変わったときだけ走らせ、
// グリッド切替などの UI 操作でシーンを作り直さない。
// 表示モードは「面の見え方」、ワイヤー重ねは「線を足すか」で軸を分ける。
// 「ワイヤー」は面が消えている状態なので、そこに線を重ねる意味はない
// (重ね設定は無効化するが値は保持し、面のあるモードに戻したら復活させる)
type ShadeMode = 'material' | 'wire';

// 背景: 単色 2 種と RoomEnvironment(背景+IBL)と透明(透過 PNG 用)
type BackgroundMode = 'light' | 'dark' | 'env' | 'transparent';

type ViewerApi = {
  setGrid: (visible: boolean) => void;
  setAutoRotate: (on: boolean) => void;
  setShade: (mode: ShadeMode, wireOverlay: boolean) => void;
  setBackground: (bg: BackgroundMode) => void;
  setExposure: (value: number) => void;
  snapshot: () => void;
};

// 回転 = 左ドラッグ、パン = SHIFT+ドラッグ / 右ドラッグ、ズーム = ホイール。
// three は重いので動的 import で分割する。
export default function GlbViewer({ url, sizeBytes, title }: Props) {
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

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

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
      const applyShade = (mode: ShadeMode, wireOverlay: boolean) => {
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

      // 軌道ドラッグ終了の pointerup で誤スナップしないよう、
      // ほぼ動いていないクリックだけをギズモに渡す
      let downX = 0;
      let downY = 0;
      const onPointerDown = (e: PointerEvent) => {
        downX = e.clientX;
        downY = e.clientY;
      };
      const onPointerUp = (e: PointerEvent) => {
        if (Math.hypot(e.clientX - downX, e.clientY - downY) < 4) viewHelper.handleClick(e);
      };
      renderer.domElement.addEventListener('pointerdown', onPointerDown);
      renderer.domElement.addEventListener('pointerup', onPointerUp);

      // ヒントの「パン · SHIFT+ドラッグ」を成立させる
      const rotateButtons = { ...controls.mouseButtons };
      const onShift = (e: KeyboardEvent) => {
        controls.mouseButtons = {
          ...rotateButtons,
          LEFT: e.shiftKey ? THREE.MOUSE.PAN : THREE.MOUSE.ROTATE,
        };
      };
      window.addEventListener('keydown', onShift);
      window.addEventListener('keyup', onShift);

      // ギズモを本体シーンの上に重ねるため、クリアは手動で行う
      renderer.autoClear = false;
      const clock = new THREE.Clock();
      let raf = 0;
      let frames = 0;
      let lastFpsAt = performance.now();
      const renderLoop = () => {
        raf = requestAnimationFrame(renderLoop);
        const delta = clock.getDelta();
        if (viewHelper.animating) viewHelper.update(delta);
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
            const wire = new THREE.Mesh(mesh.geometry, wireMaterial);
            mesh.add(wire);
            wireMeshes.push(wire);
          }
          applyShade(shadeModeRef.current, wireOverlayRef.current);
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
        snapshot: () => {
          renderer.clear();
          renderer.render(scene, camera);
          const link = document.createElement('a');
          link.href = renderer.domElement.toDataURL('image/png');
          link.download = `${titleRef.current}.png`;
          link.click();
        },
      };

      cleanup = () => {
        apiRef.current = null;
        cancelAnimationFrame(raf);
        window.removeEventListener('keydown', onShift);
        window.removeEventListener('keyup', onShift);
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

      <div className="pointer-events-none absolute inset-0 flex flex-col justify-between p-4">
        <div className="flex items-start justify-between gap-3">
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
