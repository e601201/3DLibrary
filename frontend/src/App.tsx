import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ApiError,
  getAssets,
  getCategories,
  getConfig,
  getJobs,
  getTags,
  postJob,
  postScan,
  type Asset,
  type AssetSort,
  type CategoryCount,
  type Config,
  type JobStatus,
  type TagCount,
} from './api';
import { applyTheme } from './theme';
import AssetDetail from './AssetDetail';
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
  const [tags, setTags] = useState<TagCount[]>([]);
  const [config, setConfig] = useState<Config | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [jobs, setJobs] = useState<JobStatus | null>(null);
  const [jobPostError, setJobPostError] = useState<string | null>(null);
  const [selected, setSelected] = useState<{ category: string; title: string } | null>(null);

  // 検索・フィルタ・表示
  const [queryInput, setQueryInput] = useState('');
  const [query, setQuery] = useState(''); // デバウンス済み
  const [category, setCategory] = useState(''); // '' = すべて
  const [tag, setTag] = useState(''); // '' = すべて
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
      const [assets, cats, tagCounts] = await Promise.all([
        getAssets({ q: query, category, tag, sort }),
        getCategories(),
        getTags(),
      ]);
      if (seq !== loadSeq.current) return; // 古いレスポンスは捨てる
      setAssetsState({ kind: 'ready', assets });
      setCategories(cats);
      setTags(tagCounts);
    } catch (err) {
      if (seq !== loadSeq.current) return;
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setAssetsState({ kind: 'error', message: errorMessage(err) });
      }
    }
  }, [query, category, tag, sort]);

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

  const jobsActive = jobs !== null && (jobs.running !== null || jobs.pendingCount > 0);

  // ADR-0002: キューが動いている間だけ 1 秒間隔でポーリングする
  useEffect(() => {
    if (!jobsActive) return;
    const timer = setInterval(() => {
      getJobs()
        .then((s) => {
          setJobs(s);
          // ジョブ完了分を随時カードへ反映する(サーバーはジョブごとに
          // 再スキャン済み)。キューが空になればポーリング自体が止まる
          void loadAssets();
        })
        .catch(() => {
          // 一時的な失敗は次のポーリングに任せる
        });
    }, 1000);
    return () => clearInterval(timer);
  }, [jobsActive, loadAssets]);

  const handleGenerate = useCallback(async (asset: Asset) => {
    setJobPostError(null);
    try {
      setJobs(await postJob({ category: asset.category, title: asset.title }));
    } catch (err) {
      setJobPostError(errorMessage(err));
    }
  }, []);

  useEffect(() => {
    // リロード時に進行中のバッチがあれば表示を復元する
    getJobs()
      .then(setJobs)
      .catch(() => {});
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

  const filtered = query !== '' || category !== '' || tag !== '';
  const totalCount = categories.reduce((sum, c) => sum + c.count, 0);
  const ready = assetsState.kind === 'ready';
  const runningKey = jobs?.running ? `${jobs.running.category}/${jobs.running.title}` : null;

  // 選択中のアセットが一覧から消えたら(削除・フィルタ除外)選択を解除する
  useEffect(() => {
    if (
      selected &&
      assetsState.kind === 'ready' &&
      !assetsState.assets.some(
        (a) => a.category === selected.category && a.title === selected.title,
      )
    ) {
      setSelected(null);
    }
  }, [selected, assetsState]);
  // 再スキャンで id が変わるため、選択は category/title で追いかける
  const selectedAsset =
    selected && ready
      ? assetsState.assets.find(
          (a) => a.category === selected.category && a.title === selected.title,
        )
      : undefined;

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

        {jobsActive && jobs && (
          <p className="rounded border border-blue-300 bg-blue-50 px-4 py-2 text-sm text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300">
            {jobs.running ? '生成中' : '生成準備中'}{' '}
            {Math.min(jobs.batchDone + 1, jobs.batchTotal)}/{jobs.batchTotal}
            {jobs.running && <>: {jobs.running.category}/{jobs.running.title}</>}
            (待機 {jobs.pendingCount} 件)
          </p>
        )}
        {!jobsActive && jobs?.lastError && (
          <ErrorBanner>
            生成に失敗しました({jobs.lastError.category}/{jobs.lastError.title}): {jobs.lastError.message}
          </ErrorBanner>
        )}
        {jobPostError && <ErrorBanner>生成を開始できません: {jobPostError}</ErrorBanner>}
        {scanError && <ErrorBanner>スキャンに失敗しました: {scanError}</ErrorBanner>}

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
        ) : selected && selectedAsset ? (
          <AssetDetail
            asset={selectedAsset}
            generating={runningKey === `${selectedAsset.category}/${selectedAsset.title}`}
            onBack={() => setSelected(null)}
            onGenerate={(a) => void handleGenerate(a)}
            onTagsChanged={(savedTags) => {
              // 絞り込み中のタグを外した場合はフィルタも解除する
              // (詳細画面から突然追い出されないように)
              if (tag && !savedTags.includes(tag)) setTag('');
              void loadAssets();
            }}
          />
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
              {tags.length > 0 && (
                <>
                  <p className="mb-1 mt-4 px-3 text-xs font-semibold text-neutral-500 dark:text-neutral-400">
                    タグ
                  </p>
                  <nav className="flex flex-col gap-1">
                    {tags.map((t) => (
                      <CategoryButton
                        key={t.name}
                        label={`# ${t.name}`}
                        count={t.count}
                        active={tag === t.name}
                        onClick={() => setTag(tag === t.name ? '' : t.name)}
                      />
                    ))}
                  </nav>
                </>
              )}
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
                {tags.length > 0 && (
                  <select
                    className={`${controlClass} lg:hidden`}
                    value={tag}
                    onChange={(e) => setTag(e.target.value)}
                    aria-label="タグ"
                  >
                    <option value="">すべてのタグ</option>
                    {tags.map((t) => (
                      <option key={t.name} value={t.name}>
                        {t.name} ({t.count})
                      </option>
                    ))}
                  </select>
                )}
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
              {ready && (
                <AssetGrid
                  assets={assetsState.assets}
                  view={view}
                  filtered={filtered}
                  onGenerate={(a) => void handleGenerate(a)}
                  onSelect={(a) => setSelected({ category: a.category, title: a.title })}
                  runningKey={runningKey}
                />
              )}
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

function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
      {children}
    </p>
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
