import { lazy, Suspense, useEffect, useState } from 'react';
import { assetRawUrl, getAssetDir, type Asset, type BrowseEntry } from './api';
import { formatSize } from './format';

// react-markdown は重いのでノートタブを開いたときだけロードする
const Markdown = lazy(async () => {
  const [{ default: ReactMarkdown }, { default: remarkGfm }] = await Promise.all([
    import('react-markdown'),
    import('remark-gfm'),
  ]);
  return {
    default: ({ text }: { text: string }) => (
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
    ),
  };
});

const IMAGE_DIRS = ['textures', 'references', 'renders'] as const;
type Tab = (typeof IMAGE_DIRS)[number] | 'notes';

// .svg は raw リンク経由で開くとスクリプトが実行されうるため対象外
const IMAGE_EXTENSIONS = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.avif'];

function isBrowserImage(name: string): boolean {
  const lower = name.toLowerCase();
  return IMAGE_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

// 詳細画面のファイルビューア(読み取り専用)。
// textures / references / renders は画像グリッド、notes.md は Markdown。
export default function FileViewerTabs({ asset }: { asset: Asset }) {
  const [tab, setTab] = useState<Tab>('textures');

  return (
    <section className="rounded-lg border border-neutral-300 bg-white dark:border-neutral-700 dark:bg-neutral-800">
      <div className="flex border-b border-neutral-300 dark:border-neutral-700">
        {[...IMAGE_DIRS, 'notes' as const].map((t) => (
          <button
            key={t}
            type="button"
            className={`px-4 py-2 text-sm ${
              tab === t
                ? 'border-b-2 border-blue-600 font-medium text-blue-600 dark:text-blue-400'
                : 'text-neutral-500 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-neutral-100'
            }`}
            onClick={() => setTab(t)}
          >
            {t === 'notes' ? 'ノート' : t}
          </button>
        ))}
      </div>
      <div className="p-4">
        {tab === 'notes' ? <NotesTab asset={asset} /> : <ImageDirTab asset={asset} subdir={tab} />}
      </div>
    </section>
  );
}

function ImageDirTab({ asset, subdir }: { asset: Asset; subdir: string }) {
  const [entries, setEntries] = useState<BrowseEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setEntries(null);
    setError(null);
    getAssetDir(asset.category, asset.title, subdir)
      .then((list) => {
        if (!cancelled) setEntries(list);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [asset.id, asset.category, asset.title, subdir]);

  if (error) return <p className="text-sm text-red-600 dark:text-red-400">{error}</p>;
  if (entries === null) return <p className="text-sm text-neutral-500">読み込み中…</p>;
  if (entries.length === 0) {
    return (
      <p className="py-8 text-center text-sm text-neutral-500 dark:text-neutral-400">
        {subdir} にファイルがありません
      </p>
    );
  }

  const images = entries.filter((e) => isBrowserImage(e.name));
  const others = entries.filter((e) => !isBrowserImage(e.name));

  return (
    <div className="flex flex-col gap-3">
      {images.length > 0 && (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {images.map((e) => (
            <li key={e.name} className="overflow-hidden rounded border border-neutral-200 dark:border-neutral-700">
              <a
                href={assetRawUrl(asset.category, asset.title, `${subdir}/${e.name}`)}
                target="_blank"
                rel="noreferrer"
                title={`${e.name}(${formatSize(e.size)})`}
              >
                <img
                  src={assetRawUrl(asset.category, asset.title, `${subdir}/${e.name}`)}
                  alt={e.name}
                  loading="lazy"
                  className="aspect-square w-full bg-neutral-100 object-cover dark:bg-neutral-900"
                />
              </a>
              <p className="truncate px-2 py-1 text-xs text-neutral-500 dark:text-neutral-400">{e.name}</p>
            </li>
          ))}
        </ul>
      )}
      {others.length > 0 && (
        <ul className="text-sm">
          {others.map((e) => (
            <li key={e.name} className="flex justify-between border-b border-neutral-200 py-1 last:border-0 dark:border-neutral-700">
              <span className="truncate">{e.name}</span>
              <span className="text-neutral-500 dark:text-neutral-400">{formatSize(e.size)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function NotesTab({ asset }: { asset: Asset }) {
  const [notes, setNotes] = useState<string | null>(null);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setNotes(null);
    setMissing(false);
    setError(null);
    fetch(assetRawUrl(asset.category, asset.title, 'notes.md'))
      .then(async (res) => {
        if (cancelled) return;
        if (res.status === 404) {
          setMissing(true); // 無いだけ。エラーとは区別する
          return;
        }
        if (!res.ok) {
          setError(`HTTP ${res.status}`);
          return;
        }
        setNotes(await res.text());
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [asset.id, asset.category, asset.title]);

  if (error) {
    return <p className="text-sm text-red-600 dark:text-red-400">notes.md を読み込めません: {error}</p>;
  }
  if (missing) {
    return (
      <p className="py-8 text-center text-sm text-neutral-500 dark:text-neutral-400">
        notes.md がありません
      </p>
    );
  }
  if (notes === null) return <p className="text-sm text-neutral-500">読み込み中…</p>;
  if (notes.trim() === '') {
    return (
      <p className="py-8 text-center text-sm text-neutral-500 dark:text-neutral-400">
        notes.md は空です
      </p>
    );
  }
  return (
    <div className="prose-notes text-sm leading-relaxed">
      <Suspense fallback={<p className="text-neutral-500">読み込み中…</p>}>
        <Markdown text={notes} />
      </Suspense>
    </div>
  );
}
