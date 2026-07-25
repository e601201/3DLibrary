// design/Design.pen 画面05-08 のファイルビューア(読み取り専用)。
// 詳細画面のビューポート領域を textures / references / renders / notes.md
// に差し替える。切り替えはサイドパネルのファイル一覧から行う。

import { lazy, Suspense, useEffect, useState } from 'react';
import { Download, FolderOpen } from 'lucide-react';
import { assetRawUrl, getAssetDir, type Asset, type BrowseEntry } from './api';
import { formatDate, formatSize } from './format';
import { Button, Centered, OverlayChip, cx } from './ui';

// react-markdown は重いのでノートを開いたときだけロードする
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

export type FileView = 'textures' | 'references' | 'renders' | 'notes';

// .svg は raw リンク経由で開くとスクリプトが実行されうるため対象外
const IMAGE_EXTENSIONS = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.avif'];

function isBrowserImage(name: string): boolean {
  const lower = name.toLowerCase();
  return IMAGE_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

// デザイン 05 のテクスチャカードは種別チップを持つ。ファイル名から推測する。
const TEXTURE_KINDS: [RegExp, string][] = [
  [/(albedo|base[_-]?color|diffuse|(^|[_\-.])col([_\-.]|$))/i, 'ALBEDO'],
  [/(normal|(^|[_\-.])n(rm|or)([_\-.]|$))/i, 'NORMAL'],
  [/rough/i, 'ROUGHNESS'],
  [/metal/i, 'METALLIC'],
  [/(occlusion|ambient|(^|[_\-.])ao([_\-.]|$))/i, 'AO'],
  [/(height|displac|(^|[_\-.])disp([_\-.]|$))/i, 'HEIGHT'],
  [/emiss/i, 'EMISSIVE'],
  [/(opacity|alpha|mask)/i, 'ALPHA'],
  [/spec/i, 'SPECULAR'],
];

function textureKind(name: string): string {
  const stem = name.replace(/\.[^.]+$/, '');
  for (const [pattern, label] of TEXTURE_KINDS) {
    if (pattern.test(stem)) return label;
  }
  const ext = name.slice(name.lastIndexOf('.') + 1);
  return ext.toUpperCase();
}

type Props = {
  asset: Asset;
  view: FileView;
  onReveal: () => void;
};

export default function FileViewer({ asset, view, onReveal }: Props) {
  if (view === 'notes') return <NotesView asset={asset} onReveal={onReveal} />;
  return <ImageDirView asset={asset} view={view} onReveal={onReveal} />;
}

// 各ビューの上部ツールバー(左: ディレクトリ名 + 集計 / 右: 操作)
function Toolbar({
  title,
  meta,
  children,
}: {
  title: string;
  meta: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-between gap-3">
      <div className="flex items-end gap-2.5">
        <h2 className="font-mono text-[15px] leading-none font-semibold text-ink">{title}</h2>
        <span className="font-mono text-[10px] leading-none text-ink-faint">{meta}</span>
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}

function useDirEntries(asset: Asset, subdir: string) {
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

  return { entries, error };
}

function ImageDirView({
  asset,
  view,
  onReveal,
}: {
  asset: Asset;
  view: 'textures' | 'references' | 'renders';
  onReveal: () => void;
}) {
  const { entries, error } = useDirEntries(asset, view);
  // renders は「大きな 1 枚 + サムネイル」。選択はツールバーの
  // ダウンロードからも参照するのでここで持つ。
  const [current, setCurrent] = useState(0);
  useEffect(() => {
    setCurrent(0);
  }, [asset.id, view]);

  if (error) {
    return (
      <Frame>
        <Centered>
          <p className="text-[13px] text-danger">{error}</p>
        </Centered>
      </Frame>
    );
  }
  if (entries === null) {
    return (
      <Frame>
        <Centered>
          <p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>
        </Centered>
      </Frame>
    );
  }

  const images = entries.filter((e) => isBrowserImage(e.name));
  const others = entries.filter((e) => !isBrowserImage(e.name));
  const totalSize = entries.reduce((sum, e) => sum + e.size, 0);
  const unit = view === 'textures' ? 'FILES' : 'IMAGES';
  const meta = `${entries.length} ${unit} · ${formatSize(totalSize)}`;
  const selected = images[Math.min(current, images.length - 1)];

  return (
    <Frame>
      <Toolbar title={`${view}/`} meta={meta}>
        <Button icon={FolderOpen} onClick={onReveal}>
          Finderで表示
        </Button>
        {view === 'renders' && selected && (
          <a
            href={assetRawUrl(asset.category, asset.title, `${view}/${selected.name}`)}
            download={selected.name}
            title={`${selected.name} を保存`}
            className="inline-flex shrink-0 items-center gap-2 border border-border bg-surface-2 px-4 py-2.5 text-[13px] leading-none font-medium text-ink transition hover:border-ink-faint"
          >
            <Download size={15} className="text-ink-muted" />
            ダウンロード
          </a>
        )}
      </Toolbar>

      {entries.length === 0 ? (
        <Centered>
          <p className="font-mono text-[11px] text-ink-faint">{view}/ にファイルがありません</p>
        </Centered>
      ) : view === 'textures' ? (
        <TextureGrid asset={asset} images={images} others={others} />
      ) : view === 'references' ? (
        <ReferenceBoard asset={asset} images={images} others={others} />
      ) : (
        <RenderStage
          asset={asset}
          images={images}
          others={others}
          current={current}
          onSelect={setCurrent}
        />
      )}
    </Frame>
  );
}

// 画面05-08 共通の外枠(padding 20 / gap 16 / 背景 bg)
function Frame({ children }: { children: React.ReactNode }) {
  return <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto bg-bg p-5">{children}</div>;
}

function OtherFiles({ entries }: { entries: BrowseEntry[] }) {
  if (entries.length === 0) return null;
  return (
    <ul className="flex flex-col gap-2 border border-border bg-surface p-3">
      {entries.map((e) => (
        <li key={e.name} className="flex items-center justify-between gap-3 font-mono text-[11px]">
          <span className="truncate text-ink-muted">{e.name}</span>
          <span className="shrink-0 text-[10px] text-ink-faint">{formatSize(e.size)}</span>
        </li>
      ))}
    </ul>
  );
}

// 画面05: 4 列のテクスチャカード(種別チップ + 解像度 + サイズ)
function TextureGrid({
  asset,
  images,
  others,
}: {
  asset: Asset;
  images: BrowseEntry[];
  others: BrowseEntry[];
}) {
  const [sizes, setSizes] = useState<Record<string, string>>({});

  return (
    <div className="flex flex-col gap-4">
      <ul className="grid grid-cols-2 gap-4 lg:grid-cols-3 xl:grid-cols-4">
        {images.map((e) => {
          const url = assetRawUrl(asset.category, asset.title, `textures/${e.name}`);
          return (
            <li key={e.name} className="flex flex-col border border-border bg-surface">
              <a href={url} target="_blank" rel="noreferrer" className="block aspect-square bg-surface-2">
                <img
                  src={url}
                  alt={e.name}
                  loading="lazy"
                  className="h-full w-full object-cover"
                  onLoad={(ev) => {
                    const img = ev.currentTarget;
                    setSizes((s) =>
                      s[e.name] ? s : { ...s, [e.name]: `${img.naturalWidth}×${img.naturalHeight}` },
                    );
                  }}
                />
              </a>
              <div className="flex flex-col gap-1.5 px-3 py-2.5">
                <p className="truncate font-mono text-[11px] text-ink" title={e.name}>
                  {e.name}
                </p>
                <div className="flex items-center justify-between gap-2">
                  <span className="shrink-0 bg-accent-soft px-1.5 py-0.5 font-mono text-[9px] leading-none tracking-[0.5px] text-accent">
                    {textureKind(e.name)}
                  </span>
                  <span className="truncate font-mono text-[9px] text-ink-faint">
                    {sizes[e.name] ? `${sizes[e.name]} · ` : ''}
                    {formatSize(e.size)}
                  </span>
                </div>
              </div>
            </li>
          );
        })}
      </ul>
      <OtherFiles entries={others} />
    </div>
  );
}

// 画面06: 高さの揃わない参考画像を 3 列のボードに流し込む
function ReferenceBoard({
  asset,
  images,
  others,
}: {
  asset: Asset;
  images: BrowseEntry[];
  others: BrowseEntry[];
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="columns-1 gap-4 sm:columns-2 xl:columns-3">
        {images.map((e) => {
          const url = assetRawUrl(asset.category, asset.title, `references/${e.name}`);
          return (
            <div key={e.name} className="mb-4 break-inside-avoid border border-border bg-surface">
              <a href={url} target="_blank" rel="noreferrer">
                <img src={url} alt={e.name} loading="lazy" className="w-full" />
              </a>
              <div className="flex items-center justify-between gap-2 px-3 py-2">
                <span className="truncate font-mono text-[10px] text-ink-muted" title={e.name}>
                  {e.name}
                </span>
                <span className="shrink-0 font-mono text-[9px] text-ink-faint">
                  {formatSize(e.size)}
                </span>
              </div>
            </div>
          );
        })}
      </div>
      <OtherFiles entries={others} />
    </div>
  );
}

// 画面07: 大きな 1 枚 + 下部のサムネイルストリップ
function RenderStage({
  asset,
  images,
  others,
  current,
  onSelect,
}: {
  asset: Asset;
  images: BrowseEntry[];
  others: BrowseEntry[];
  current: number;
  onSelect: (index: number) => void;
}) {
  const [dimensions, setDimensions] = useState<string | null>(null);

  const selected = images[Math.min(current, images.length - 1)];
  if (!selected) return <OtherFiles entries={others} />;

  const url = assetRawUrl(asset.category, asset.title, `renders/${selected.name}`);
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="relative flex min-h-72 flex-1 items-center justify-center overflow-hidden border border-border bg-stage p-3">
        <img
          key={url}
          src={url}
          alt={selected.name}
          className="max-h-full max-w-full object-contain"
          onLoad={(e) =>
            setDimensions(`${e.currentTarget.naturalWidth}×${e.currentTarget.naturalHeight}`)
          }
        />
        <div className="absolute bottom-3 left-3">
          <OverlayChip>
            {selected.name}
            {dimensions && ` · ${dimensions}`} · {formatSize(selected.size)}
          </OverlayChip>
        </div>
      </div>

      <ul className="flex shrink-0 flex-wrap gap-3">
        {images.map((e, i) => (
          <li key={e.name}>
            <button
              type="button"
              onClick={() => {
                onSelect(i);
                setDimensions(null);
              }}
              className="flex flex-col items-start gap-1.5"
            >
              <span
                className={cx(
                  'block h-[84px] w-[148px] overflow-hidden border bg-stage',
                  i === current ? 'border-2 border-accent' : 'border-border',
                )}
              >
                <img
                  src={assetRawUrl(asset.category, asset.title, `renders/${e.name}`)}
                  alt=""
                  loading="lazy"
                  className="h-full w-full object-cover"
                />
              </span>
              <span
                className={cx(
                  'max-w-[148px] truncate font-mono text-[9px]',
                  i === current ? 'text-accent' : 'text-ink-faint',
                )}
              >
                {e.name}
              </span>
            </button>
          </li>
        ))}
      </ul>

      <OtherFiles entries={others} />
    </div>
  );
}

// 画面08: notes.md を 760px 幅のドキュメントとして表示する
function NotesView({ asset, onReveal }: { asset: Asset; onReveal: () => void }) {
  const [notes, setNotes] = useState<string | null>(null);
  const [missing, setMissing] = useState(false);
  const [modified, setModified] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setNotes(null);
    setMissing(false);
    setModified(null);
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
        const lastModified = res.headers.get('last-modified');
        if (lastModified) setModified(new Date(lastModified).toISOString());
        setNotes(await res.text());
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [asset.id, asset.category, asset.title]);

  const bytes = notes === null ? null : new Blob([notes]).size;
  const meta = [
    bytes === null ? null : formatSize(bytes),
    modified === null ? null : `更新 ${formatDate(modified)}`,
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <Frame>
      <Toolbar title="notes.md" meta={meta}>
        <Button icon={FolderOpen} onClick={onReveal}>
          Finderで表示
        </Button>
      </Toolbar>

      {error && (
        <Centered>
          <p className="text-[13px] text-danger">notes.md を読み込めません: {error}</p>
        </Centered>
      )}
      {missing && (
        <Centered>
          <p className="font-mono text-[11px] text-ink-faint">notes.md がありません</p>
        </Centered>
      )}
      {!error && !missing && notes === null && (
        <Centered>
          <p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>
        </Centered>
      )}
      {notes !== null &&
        (notes.trim() === '' ? (
          <Centered>
            <p className="font-mono text-[11px] text-ink-faint">notes.md は空です</p>
          </Centered>
        ) : (
          <div className="flex justify-center">
            <article className="prose-notes w-full max-w-[760px] border border-border bg-surface px-12 py-10">
              <Suspense
                fallback={<p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>}
              >
                <Markdown text={notes} />
              </Suspense>
            </article>
          </div>
        ))}
    </Frame>
  );
}
