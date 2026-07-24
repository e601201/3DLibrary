import { useEffect, useRef, useState } from 'react';
import type { Material, Mesh } from 'three';

type Props = {
  url: string; // GLB の配信 URL
};

// Three.js で GLB を表示する(回転 = 左ドラッグ、パン = 右ドラッグ、
// ズーム = ホイール)。three は重いので動的 import で分割する。
export default function GlbViewer({ url }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

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

      const controls = new OrbitControls(camera, renderer.domElement);
      controls.enableDamping = true;

      let raf = 0;
      const renderLoop = () => {
        raf = requestAnimationFrame(renderLoop);
        controls.update();
        renderer.render(scene, camera);
      };

      new GLTFLoader().load(
        url,
        (gltf) => {
          if (disposed) return;
          scene.add(gltf.scene);
          // バウンディングボックスに合わせてカメラを配置する
          const box = new THREE.Box3().setFromObject(gltf.scene);
          const center = box.getCenter(new THREE.Vector3());
          const radius = Math.max(box.getSize(new THREE.Vector3()).length() / 2, 0.5);
          camera.position.copy(center.clone().add(new THREE.Vector3(1, 0.7, 1).normalize().multiplyScalar(radius * 2.5)));
          camera.near = radius / 100;
          camera.far = radius * 100;
          camera.updateProjectionMatrix();
          controls.target.copy(center);
          controls.update();
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

      cleanup = () => {
        cancelAnimationFrame(raf);
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

  return (
    <div className="relative h-full w-full">
      <div ref={containerRef} className="h-full w-full" />
      {error && (
        <p className="absolute inset-0 flex items-center justify-center text-sm text-red-600 dark:text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
