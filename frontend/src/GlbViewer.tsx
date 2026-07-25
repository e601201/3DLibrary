// design/Design.pen 画面02 のビューポート。3D 表示の上に
// バッジ(左上)・ツール(右上)・操作ヒント(下中央)を重ねる。

import { useEffect, useRef, useState } from 'react';
import { Camera, Grid3x3, Maximize, Move, Rotate3d, ZoomIn } from 'lucide-react';
import type { Material, Mesh } from 'three';
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
type ViewerApi = {
  setGrid: (visible: boolean) => void;
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
  const [fps, setFps] = useState<number | null>(null);

  // スクリーンショットのファイル名にしか使わないので、
  // 変わってもシーンを作り直さないよう ref で持つ
  const titleRef = useRef(title);
  titleRef.current = title;

  // 再生成で url が変わるとシーンを作り直すため、そのときも
  // 現在のトグル状態を引き継げるよう ref に写しておく
  const gridRef = useRef(grid);
  gridRef.current = grid;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let disposed = false;
    let cleanup: (() => void) | null = null;

    (async () => {
      const THREE = await import('three');
      const [{ GLTFLoader }, { OrbitControls }] = await Promise.all([
        import('three/addons/loaders/GLTFLoader.js'),
        import('three/addons/controls/OrbitControls.js'),
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
      container.appendChild(renderer.domElement);

      scene.add(new THREE.HemisphereLight(0xffffff, 0x555566, 2.5));
      const sun = new THREE.DirectionalLight(0xffffff, 2);
      sun.position.set(3, 5, 4);
      scene.add(sun);

      // デザインのビューポートはアクセント色のグリッド床が敷かれている
      const gridHelper = new THREE.GridHelper(10, 20, 0xa855f7, 0x3b2a4d);
      gridHelper.visible = gridRef.current;
      scene.add(gridHelper);

      const controls = new OrbitControls(camera, renderer.domElement);
      controls.enableDamping = true;

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

      let raf = 0;
      let frames = 0;
      let lastFpsAt = performance.now();
      const renderLoop = () => {
        raf = requestAnimationFrame(renderLoop);
        controls.update();
        renderer.render(scene, camera);
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
        // preserveDrawingBuffer を有効にしなくて済むよう、描画直後に読み出す
        snapshot: () => {
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
        resizeObserver.disconnect();
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
    <div ref={wrapperRef} className="relative h-full w-full overflow-hidden bg-stage">
      <div ref={containerRef} className="h-full w-full" />

      <div className="pointer-events-none absolute inset-0 flex flex-col justify-between p-4">
        <div className="flex items-start justify-between gap-3">
          <OverlayChip>
            <span className="size-1.5 shrink-0 rounded-full bg-stage-ok" />
            GLB PREVIEW
            {sizeBytes !== null && ` · ${formatSize(sizeBytes)}`}
            {fps !== null && ` · ${fps} FPS`}
          </OverlayChip>
          <div className="pointer-events-auto flex gap-1">
            <ViewportTool
              icon={Grid3x3}
              label="グリッドの表示切替"
              active={grid}
              onClick={() => setGrid((v) => !v)}
            />
            <ViewportTool
              icon={Camera}
              label="スクリーンショットを保存"
              onClick={() => apiRef.current?.snapshot()}
            />
            <ViewportTool icon={Maximize} label="全画面表示" onClick={toggleFullscreen} />
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
