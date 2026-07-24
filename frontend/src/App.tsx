import { useCallback, useEffect, useState } from 'react';
import { ApiError, getAssets, getConfig, postScan, type Asset, type Config } from './api';
import { applyTheme } from './theme';
import AssetGrid from './AssetGrid';
import Settings from './Settings';

type AssetsState =
  | { kind: 'loading' }
  | { kind: 'ready'; assets: Asset[] }
  | { kind: 'notConfigured' }
  | { kind: 'error'; message: string };

function isNotConfigured(err: unknown): boolean {
  return err instanceof ApiError && err.code === 'library_not_configured';
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export default function App() {
  const [assetsState, setAssetsState] = useState<AssetsState>({ kind: 'loading' });
  const [config, setConfig] = useState<Config | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);

  const loadAssets = useCallback(async () => {
    try {
      setAssetsState({ kind: 'ready', assets: await getAssets() });
    } catch (err) {
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setAssetsState({ kind: 'error', message: errorMessage(err) });
      }
    }
  }, []);

  const rescan = useCallback(async () => {
    setScanning(true);
    setScanError(null);
    try {
      await postScan();
      await loadAssets();
    } catch (err) {
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setScanError(errorMessage(err));
      }
    } finally {
      setScanning(false);
    }
  }, [loadAssets]);

  useEffect(() => {
    getConfig()
      .then((c) => {
        setConfig(c);
        applyTheme(c.theme);
      })
      .catch(() => {
        // 設定が読めなくてもアプリ自体は動かす(テーマはデフォルトのまま)
      });
    // 起動時スキャンはバックエンドが済ませているので一覧を取るだけでよい
    void loadAssets();
  }, [loadAssets]);

  const handleSaved = (c: Config) => {
    setConfig(c);
    // ライブラリ変更を一覧に反映する(新規指定ならスキャンも走らせる)
    if (c.libraryDir) {
      void rescan();
    } else {
      void loadAssets();
    }
  };

  return (
    <main className="min-h-screen bg-neutral-50 text-neutral-900 dark:bg-neutral-900 dark:text-neutral-100">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 p-8">
        <header className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">3DLibrary</h1>
            <p className="text-sm text-neutral-500 dark:text-neutral-400">
              Blender アセットのローカル管理
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="rounded border border-neutral-300 px-3 py-2 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-600 dark:hover:bg-neutral-800"
              disabled={scanning || assetsState.kind === 'notConfigured'}
              onClick={() => void rescan()}
            >
              {scanning ? 'スキャン中…' : '↻ 再スキャン'}
            </button>
            <button
              type="button"
              className="rounded border border-neutral-300 px-3 py-2 text-sm hover:bg-neutral-100 dark:border-neutral-600 dark:hover:bg-neutral-800"
              onClick={() => setShowSettings((v) => !v)}
            >
              {showSettings ? '設定を閉じる' : '⚙ 設定'}
            </button>
          </div>
        </header>

        {showSettings && config && <Settings initial={config} onSaved={handleSaved} />}

        {scanError && (
          <p className="rounded border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
            スキャンに失敗しました: {scanError}
          </p>
        )}

        {assetsState.kind === 'loading' && (
          <p className="py-12 text-center text-sm text-neutral-500">読み込み中…</p>
        )}
        {assetsState.kind === 'notConfigured' && (
          <div className="flex flex-col items-center gap-3 py-12">
            <p className="text-sm text-neutral-500 dark:text-neutral-400">
              ライブラリディレクトリが設定されていません。
            </p>
            <button
              type="button"
              className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
              onClick={() => setShowSettings(true)}
            >
              設定を開く
            </button>
          </div>
        )}
        {assetsState.kind === 'error' && (
          <p className="py-12 text-center text-sm text-red-600 dark:text-red-400">
            一覧を取得できません: {assetsState.message}
          </p>
        )}
        {assetsState.kind === 'ready' && <AssetGrid assets={assetsState.assets} />}
      </div>
    </main>
  );
}
