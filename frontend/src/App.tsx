import { useEffect, useState } from 'react';

type HealthState =
  | { kind: 'loading' }
  | { kind: 'ok'; status: string }
  | { kind: 'error'; message: string };

export default function App() {
  const [health, setHealth] = useState<HealthState>({ kind: 'loading' });

  useEffect(() => {
    let cancelled = false;
    fetch('/api/health')
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const body: { status: string } = await res.json();
        if (!cancelled) setHealth({ kind: 'ok', status: body.status });
      })
      .catch((err: unknown) => {
        if (!cancelled)
          setHealth({ kind: 'error', message: err instanceof Error ? err.message : String(err) });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 bg-neutral-900 text-neutral-100">
      <h1 className="text-3xl font-bold">3DLibrary</h1>
      <p className="text-neutral-400">Blender アセットのローカル管理</p>
      <div className="rounded-lg border border-neutral-700 bg-neutral-800 px-6 py-4">
        {health.kind === 'loading' && <p>ヘルスチェック中…</p>}
        {health.kind === 'ok' && (
          <p>
            バックエンド: <span className="font-mono text-green-400">{health.status}</span>
          </p>
        )}
        {health.kind === 'error' && (
          <p>
            バックエンドに接続できません:{' '}
            <span className="font-mono text-red-400">{health.message}</span>
          </p>
        )}
      </div>
    </main>
  );
}
