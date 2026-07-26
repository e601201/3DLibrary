import { useCallback, useEffect, useRef, useState } from 'react';
import { ChevronDown, LayoutGrid, List, Loader, Plus, Search } from 'lucide-react';
import {
  ApiError,
  getAssets,
  getCacheInfo,
  getCategories,
  getConfig,
  getJobs,
  getTags,
  needsGeneration,
  postBulkJobs,
  postJob,
  postScan,
  type Asset,
  type AssetSort,
  type CategoryCount,
  type CacheInfo,
  type Config,
  type JobStatus,
  type TagCount,
} from './api';
import { applyTheme } from './theme';
import AssetDetail from './AssetDetail';
import AssetGrid, { type ViewMode } from './AssetGrid';
import NewAssetModal from './NewAssetModal';
import Settings from './Settings';
import Sidebar from './Sidebar';
import { Banner, Button, Centered, cx } from './ui';

type AssetsState =
  | { kind: 'loading' }
  | { kind: 'ready'; assets: Asset[] }
  | { kind: 'notConfigured' }
  | { kind: 'error'; message: string };

type Page = 'library' | 'settings';

function isNotConfigured(err: unknown): boolean {
  return err instanceof ApiError && err.code === 'library_not_configured';
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

const SORT_LABELS: Record<AssetSort, string> = {
  title: 'カテゴリ・タイトル順',
  updated_desc: '更新日: 新しい順',
  updated_asc: '更新日: 古い順',
};

// デザイン 01 のヘッダー右端はライブラリの source パス。Windows の
// バックスラッシュ区切りに合わせて連結する。
function sourcePath(libraryDir: string): string {
  if (libraryDir === '') return '';
  return libraryDir.includes('\\') ? `${libraryDir}\\source` : `${libraryDir}/source`;
}

export default function App() {
  const [assetsState, setAssetsState] = useState<AssetsState>({ kind: 'loading' });
  const [categories, setCategories] = useState<CategoryCount[]>([]);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [cache, setCache] = useState<CacheInfo | null>(null);
  const [missingCount, setMissingCount] = useState<number | null>(null);
  const [config, setConfig] = useState<Config | null>(null);
  const [page, setPage] = useState<Page>('library');
  const [showCreate, setShowCreate] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [jobs, setJobs] = useState<JobStatus | null>(null);
  const [jobPostError, setJobPostError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
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

  const filtered = query !== '' || category !== '' || tag !== '';
  // ポーリング中に依存が変わって interval が張り直されないよう ref で読む
  const filteredRef = useRef(filtered);
  filteredRef.current = filtered;

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
      // 絞り込みが無ければこの結果が全件なので、そのまま「不足分」を数える
      if (!filteredRef.current) setMissingCount(assets.filter(needsGeneration).length);
    } catch (err) {
      if (seq !== loadSeq.current) return;
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setAssetsState({ kind: 'error', message: errorMessage(err) });
      }
    }
  }, [query, category, tag, sort]);

  // キャッシュ容量は検索条件と無関係なので loadAssets(キーストローク毎)
  // には含めず、変化しうるタイミングでだけ取り直す
  const loadStats = useCallback(() => {
    getCacheInfo()
      .then(setCache)
      .catch(() => {});
    // 絞り込み中は一覧が全件ではないので、サイドバーの「(3)」だけ別に数える
    if (!filteredRef.current) return;
    getAssets({})
      .then((all) => setMissingCount(all.filter(needsGeneration).length))
      .catch(() => setMissingCount(null));
  }, []);

  const rescan = useCallback(async () => {
    setScanning(true);
    setScanError(null);
    setNotice(null);
    try {
      await postScan();
      await loadAssets();
      loadStats();
    } catch (err) {
      if (isNotConfigured(err)) {
        setAssetsState({ kind: 'notConfigured' });
      } else {
        setScanError(errorMessage(err));
      }
    } finally {
      setScanning(false);
    }
  }, [loadAssets, loadStats]);

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
          loadStats(); // 生成でキャッシュ容量と不足件数が変わる
        })
        .catch(() => {
          // 一時的な失敗は次のポーリングに任せる
        });
    }, 1000);
    return () => clearInterval(timer);
  }, [jobsActive, loadAssets, loadStats]);

  const handleGenerate = useCallback(async (asset: Asset) => {
    setJobPostError(null);
    setNotice(null);
    try {
      setJobs(await postJob({ category: asset.category, title: asset.title }));
    } catch (err) {
      setJobPostError(errorMessage(err));
    }
  }, []);

  const handleBulkGenerate = useCallback(async () => {
    setJobPostError(null);
    setNotice(null);
    try {
      const res = await postBulkJobs();
      setJobs(res.status);
      if (res.enqueued === 0) {
        setNotice('生成が必要なアセットはありません(すべて最新です)');
      }
    } catch (err) {
      setJobPostError(errorMessage(err));
    }
  }, []);

  useEffect(() => {
    // リロード時に進行中のバッチがあれば表示を復元する
    getJobs()
      .then(setJobs)
      .catch(() => {});
    loadStats();
    getConfig()
      .then((c) => {
        setConfig(c);
        applyTheme(c.theme);
      })
      .catch(() => {
        // 設定が読めなくてもアプリ自体は動かす(テーマはデフォルトのまま)
      });
  }, [loadStats]);

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

  const totalCount = categories.reduce((sum, c) => sum + c.count, 0);
  const ready = assetsState.kind === 'ready';
  const notConfigured = assetsState.kind === 'notConfigured';
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

  // 生成の失敗は一覧でも詳細でも同じ文言で出す(詳細は全画面で
  // 一覧のバナーが描画されないため、ここで組み立てて渡す)
  const jobError = jobPostError
    ? `生成を開始できません: ${jobPostError}`
    : !jobsActive && jobs?.lastError
      ? `生成に失敗しました(${jobs.lastError.category}/${jobs.lastError.title}): ${jobs.lastError.message}`
      : null;

  // 詳細はデザイン 02 のとおりサイドバーを持たない全画面レイアウト
  if (selected && selectedAsset) {
    return (
      <AssetDetail
        asset={selectedAsset}
        allTags={tags}
        generating={runningKey === `${selectedAsset.category}/${selectedAsset.title}`}
        jobError={jobError}
        onBack={() => setSelected(null)}
        onGenerate={(a) => void handleGenerate(a)}
        onTagsChanged={(savedTags) => {
          // 絞り込み中のタグを外した場合はフィルタも解除する
          // (詳細画面から突然追い出されないように)
          if (tag && !savedTags.includes(tag)) setTag('');
          void loadAssets();
        }}
      />
    );
  }

  return (
    <div className="flex h-screen overflow-hidden bg-bg font-sans text-ink">
      <Sidebar
        page={page}
        categories={categories}
        tags={tags}
        category={category}
        tag={tag}
        totalCount={totalCount}
        cache={cache}
        missingCount={missingCount}
        scanning={scanning}
        jobsActive={jobsActive}
        disabled={notConfigured}
        onCategory={setCategory}
        onTag={setTag}
        onRescan={() => void rescan()}
        onBulkGenerate={() => void handleBulkGenerate()}
        onOpenLibrary={() => setPage('library')}
        onOpenSettings={() => setPage('settings')}
      />

      <main className="flex min-w-0 flex-1 flex-col">
        {page === 'settings' ? (
          config ? (
            <Settings
              initial={config}
              cache={cache}
              onSaved={handleSaved}
              onCacheCleared={() => {
                void loadAssets();
                loadStats();
              }}
            />
          ) : (
            <Centered>
              <p className="text-[13px] text-ink-faint">設定を読み込んでいます…</p>
            </Centered>
          )
        ) : (
          <>
            <div className="flex shrink-0 items-center gap-3 border-b border-border px-6 py-3">
              <div className="flex w-80 shrink-0 items-center gap-2 border border-border bg-surface-2 px-3 py-[9px] focus-within:border-accent">
                <Search size={14} className="shrink-0 text-ink-faint" />
                <input
                  type="search"
                  className="w-full min-w-0 bg-transparent text-[13px] text-ink placeholder:text-ink-faint focus:outline-none"
                  placeholder="タイトル・タグで検索..."
                  value={queryInput}
                  aria-label="検索"
                  onChange={(e) => setQueryInput(e.target.value)}
                />
              </div>

              <FilterSelect
                label="カテゴリ"
                value={category}
                onChange={setCategory}
                options={[
                  { value: '', label: 'カテゴリ: すべて' },
                  ...categories.map((c) => ({ value: c.name, label: `カテゴリ: ${c.name}` })),
                ]}
              />
              <FilterSelect
                label="並び順"
                value={sort}
                onChange={(v) => setSort(v as AssetSort)}
                options={(Object.keys(SORT_LABELS) as AssetSort[]).map((s) => ({
                  value: s,
                  label: SORT_LABELS[s],
                }))}
              />

              <div className="flex-1" />

              {jobsActive && jobs && (
                <div className="flex shrink-0 items-center gap-1.5 border border-border bg-surface-2 px-3 py-2">
                  <Loader size={13} className="animate-spin text-accent" />
                  <span className="font-mono text-[11px] whitespace-nowrap text-ink-muted">
                    {jobs.running ? '生成中' : '生成準備中'}{' '}
                    {Math.min(jobs.batchDone + 1, jobs.batchTotal)}/{jobs.batchTotal}
                  </span>
                </div>
              )}

              <div className="flex shrink-0 border border-border">
                <ViewButton
                  icon={LayoutGrid}
                  label="カード表示"
                  active={view === 'card'}
                  onClick={() => setView('card')}
                />
                <ViewButton
                  icon={List}
                  label="リスト表示"
                  active={view === 'list'}
                  onClick={() => setView('list')}
                />
              </div>

              <Button
                variant="primary"
                icon={Plus}
                disabled={notConfigured}
                onClick={() => setShowCreate(true)}
              >
                新規アセット
              </Button>
            </div>

            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-6">
              <div className="flex items-end justify-between gap-4">
                <div className="flex min-w-0 items-end gap-2.5">
                  <h1 className="font-heading text-xl leading-none font-bold text-ink">
                    {category === '' ? 'すべてのアセット' : category}
                  </h1>
                  <span className="font-mono text-[11px] leading-none whitespace-nowrap text-ink-faint">
                    {ready ? assetsState.assets.length : '—'} ASSETS
                  </span>
                  {tag !== '' && (
                    <button
                      type="button"
                      className="font-mono text-[11px] leading-none text-accent hover:underline"
                      title="タグの絞り込みを解除"
                      onClick={() => setTag('')}
                    >
                      #{tag} ×
                    </button>
                  )}
                </div>
                {config?.libraryDir && (
                  <span className="truncate font-mono text-[10px] text-ink-faint">
                    {sourcePath(config.libraryDir)}
                  </span>
                )}
              </div>

              {jobError && <Banner tone="error">{jobError}</Banner>}
              {scanError && <Banner tone="error">スキャンに失敗しました: {scanError}</Banner>}
              {notice && <Banner tone="info">{notice}</Banner>}

              {notConfigured && (
                <Centered>
                  <p className="text-[13px] text-ink-muted">
                    ライブラリディレクトリが設定されていません。
                  </p>
                  <Button variant="primary" onClick={() => setPage('settings')}>
                    設定を開く
                  </Button>
                </Centered>
              )}
              {assetsState.kind === 'loading' && (
                <Centered>
                  <p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>
                </Centered>
              )}
              {assetsState.kind === 'error' && (
                <Centered>
                  <p className="text-[13px] text-danger">一覧を取得できません: {assetsState.message}</p>
                </Centered>
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
            </div>
          </>
        )}
      </main>

      {showCreate && (
        <NewAssetModal
          categories={categories.map((c) => c.name)}
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            void loadAssets();
            loadStats();
          }}
        />
      )}
    </div>
  );
}

// デザイン 01 のトップバーのフィルタ(枠 + ラベル + chevron)
function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="relative shrink-0">
      <select
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="max-w-52 appearance-none truncate border border-border bg-transparent py-2 pr-8 pl-3 text-xs text-ink-muted focus:border-accent focus:outline-none"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value} className="bg-surface text-ink">
            {o.label}
          </option>
        ))}
      </select>
      <ChevronDown
        size={13}
        className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-ink-faint"
      />
    </div>
  );
}

function ViewButton({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: typeof LayoutGrid;
  label: string;
  active: boolean;
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
        'p-2 transition',
        active ? 'bg-accent-soft text-accent' : 'text-ink-faint hover:text-ink',
      )}
    >
      <Icon size={15} />
    </button>
  );
}
