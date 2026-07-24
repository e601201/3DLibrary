import { useEffect, useState } from 'react';
import { getConfig, getHealth, type Config } from './api';
import { applyTheme } from './theme';
import Settings from './Settings';

type HealthState =
  | { kind: 'loading' }
  | { kind: 'ok'; status: string }
  | { kind: 'error'; message: string };

export default function App() {
  const [health, setHealth] = useState<HealthState>({ kind: 'loading' });
  const [config, setConfig] = useState<Config | null>(null);
  const [showSettings, setShowSettings] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getHealth()
      .then((body) => {
        if (!cancelled) setHealth({ kind: 'ok', status: body.status });
      })
      .catch((err: unknown) => {
        if (!cancelled)
          setHealth({ kind: 'error', message: err instanceof Error ? err.message : String(err) });
      });
    getConfig()
      .then((c) => {
        if (cancelled) return;
        setConfig(c);
        applyTheme(c.theme);
      })
      .catch(() => {
        // 設定が読めなくてもアプリ自体は動かす(テーマはデフォルトのまま)
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="flex min-h-screen flex-col items-center gap-6 bg-neutral-50 p-8 text-neutral-900 dark:bg-neutral-900 dark:text-neutral-100">
      <header className="flex w-full max-w-lg items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">3DLibrary</h1>
          <p className="text-sm text-neutral-500 dark:text-neutral-400">
            Blender アセットのローカル管理
          </p>
        </div>
        <button
          type="button"
          className="rounded border border-neutral-300 px-3 py-2 text-sm hover:bg-neutral-100 dark:border-neutral-600 dark:hover:bg-neutral-800"
          onClick={() => setShowSettings((v) => !v)}
        >
          {showSettings ? '設定を閉じる' : '⚙ 設定'}
        </button>
      </header>

      <div className="w-full max-w-lg rounded-lg border border-neutral-300 bg-white px-6 py-4 dark:border-neutral-700 dark:bg-neutral-800">
        {health.kind === 'loading' && <p>ヘルスチェック中…</p>}
        {health.kind === 'ok' && (
          <p>
            バックエンド: <span className="font-mono text-green-600 dark:text-green-400">{health.status}</span>
          </p>
        )}
        {health.kind === 'error' && (
          <p>
            バックエンドに接続できません:{' '}
            <span className="font-mono text-red-600 dark:text-red-400">{health.message}</span>
          </p>
        )}
      </div>

      {showSettings && config && (
        <Settings initial={config} onSaved={(c) => setConfig(c)} />
      )}
    </main>
  );
}
