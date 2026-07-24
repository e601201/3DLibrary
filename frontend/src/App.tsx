import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ApiError,
  getAssets,
  getCategories,
  getConfig,
  postScan,
  type Asset,
  type AssetSort,
  type CategoryCount,
  type Config,
} from './api';
import { applyTheme } from './theme';
import AssetGrid, { type ViewMode } from './AssetGrid';
import NewAssetModal from './NewAssetModal';
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

const controlClass =
  'rounded border border-neutral-300 bg-white px-3 py-2 text-sm ' +
  'dark:border-neutral-600 dark:bg-neutral-800';

export default function App() {
  const [assetsState, setAssetsState] = useState<AssetsState>({ kind: 'loading' });
  const [categories, setCategories] = useState<CategoryCount[]>([]);
  const [config, setConfig] = useState<Config | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);

  // 検索・フィルタ・表示
  const [queryInput, setQueryInput] = useState('');
  const [query, setQuery] = useState(''); // デバウンス済み
  const [category, setCategory] = useState(''); // '' = すべて
  const [sort, setSort] = useState<AssetSort>('title');
  const [view, setView] = useState<ViewMode>('card');

  // 入力から 200ms 落ち着いたら検索する
  useEffect(() => {
    const timer = setTimeout(() => setQuery(queryInput.trim()), 200);
    return () => clearTimeout(timer);
  }, [queryInput]);

  // フィルタ変更が連続したとき、遅れて返ってきた古い結果で
  // 新しい結果を上書きしないための連番
  const loadSeq = useRef(0);

  const loadAssets = useCallback(async () => {
    const seq = ++loadSeq.current;
    try {
      const [assets, cats] = await Promise.all([
        getAssets({ q: query, category, sort }),
        getCategories(),
      ]);
      if (seq !== loadSeq.current) return; // 古いレスポンスは捨てる
      setAssetsState({ kind: 'ready', assets });
      setCategories(cats);
    } catch (err) {
      if (seq !== loadSeq.current) return;
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setAssetsState({ kind: 'error', message: errorMessage(err) });
      }
    }
  }, [query, category, sort]);

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
  }, []);

  // 起動時スキャンはバックエンドが済ませている。フィルタ変更時も再取得
  useEffect(() => {
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

  const filtered = query !== '' || category !== '';
  const totalCount = categories.reduce((sum, c) => sum + c.count, 0);
  const ready = assetsState.kind === 'ready';

  return (
    <main className="min-h-screen bg-neutral-50 text-neutral-900 dark:bg-neutral-900 dark:text-neutral-100">
      <div className="mx-auto flex max-w-7xl flex-col gap-6 p-8">
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
              className="rounded bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50"
              disabled={assetsState.kind === 'notConfigured'}
              onClick={() => setShowCreate(true)}
            >
              ＋ 新規作成
            </button>
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

        {assetsState.kind === 'notConfigured' ? (
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
        ) : (
          <div className="flex gap-6">
            {/* サイドバー: 動的カテゴリ一覧(件数付き) */}
            <aside className="hidden w-56 shrink-0 lg:block">
              <nav className="flex flex-col gap-1">
                <CategoryButton
                  label="すべて"
                  count={totalCount}
                  active={category === ''}
                  onClick={() => setCategory('')}
                />
                {categories.map((c) => (
                  <CategoryButton
                    key={c.name}
                    label={c.name}
                    count={c.count}
                    active={category === c.name}
                    onClick={() => setCategory(c.name)}
                  />
                ))}
              </nav>
            </aside>

            <section className="flex min-w-0 flex-1 flex-col gap-4">
              {/* ツールバー: 検索・カテゴリ(小画面)・ソート・表示切替 */}
              <div className="flex flex-wrap items-center gap-2">
                <input
                  type="search"
                  className={`${controlClass} min-w-0 flex-1`}
                  placeholder="タイトルで検索…"
                  value={queryInput}
                  onChange={(e) => setQueryInput(e.target.value)}
                />
                <select
                  className={`${controlClass} lg:hidden`}
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                  aria-label="カテゴリ"
                >
                  <option value="">すべてのカテゴリ</option>
                  {categories.map((c) => (
                    <option key={c.name} value={c.name}>
                      {c.name} ({c.count})
                    </option>
                  ))}
                </select>
                <select
                  className={controlClass}
                  value={sort}
                  onChange={(e) => setSort(e.target.value as AssetSort)}
                  aria-label="並び順"
                >
                  <option value="title">カテゴリ・タイトル順</option>
                  <option value="updated_desc">更新日(新しい順)</option>
                  <option value="updated_asc">更新日(古い順)</option>
                </select>
                <div className="flex overflow-hidden rounded border border-neutral-300 dark:border-neutral-600">
                  <ViewButton label="カード" active={view === 'card'} onClick={() => setView('card')} />
                  <ViewButton label="リスト" active={view === 'list'} onClick={() => setView('list')} />
                </div>
              </div>

              {assetsState.kind === 'loading' && (
                <p className="py-12 text-center text-sm text-neutral-500">読み込み中…</p>
              )}
              {assetsState.kind === 'error' && (
                <p className="py-12 text-center text-sm text-red-600 dark:text-red-400">
                  一覧を取得できません: {assetsState.message}
                </p>
              )}
              {ready && <AssetGrid assets={assetsState.assets} view={view} filtered={filtered} />}
            </section>
          </div>
        )}
      </div>

      {showCreate && (
        <NewAssetModal
          categories={categories.map((c) => c.name)}
          onClose={() => setShowCreate(false)}
          onCreated={() => void loadAssets()}
        />
      )}
    </main>
  );
}

function CategoryButton({
  label,
  count,
  active,
  onClick,
}: {
  label: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`flex items-center justify-between rounded px-3 py-2 text-left text-sm ${
        active
          ? 'bg-blue-600 text-white'
          : 'hover:bg-neutral-100 dark:hover:bg-neutral-800'
      }`}
      onClick={onClick}
    >
      <span className="truncate">{label}</span>
      <span className={active ? 'text-blue-100' : 'text-neutral-500 dark:text-neutral-400'}>
        {count}
      </span>
    </button>
  );
}

function ViewButton({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`px-3 py-2 text-sm ${
        active
          ? 'bg-blue-600 text-white'
          : 'bg-white hover:bg-neutral-100 dark:bg-neutral-800 dark:hover:bg-neutral-700'
      }`}
      onClick={onClick}
    >
      {label}
    </button>
  );
}
