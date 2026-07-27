// design/Design.pen 画面02・05-09 のアセット詳細。サイドバーを持たない
// 全画面レイアウトで、ヘッダー + (ビューポート | ファイルビューア) + 右サイドパネル。

import { useEffect, useState } from 'react';
import {
  ArrowLeft,
  Aperture,
  Box,
  ExternalLink,
  FileBox,
  FileText,
  FolderOpen,
  Image as ImageIcon,
  Images,
  Plus,
  RefreshCw,
  TriangleAlert,
  Zap,
} from 'lucide-react';
import {
  getAssetFiles,
  getExtractedMetadata,
  glbUrl,
  postOpenBlender,
  postReveal,
  putTags,
  type Asset,
  type AssetFiles,
  type ExtractedMetadata,
  type FileEntry,
  type TagCount,
} from './api';
import { formatDate, formatNumber, formatSize } from './format';
import { StaleBadge } from './AssetGrid';
import FileViewer, { type FileView } from './FileViewer';
import GlbViewer from './GlbViewer';
import TagSuggestInput, { TagChip } from './TagSuggestInput';
import {
  Banner,
  Button,
  Chip,
  IconButton,
  SectionLabel,
  cx,
  type LucideIcon,
} from './ui';

type Props = {
  asset: Asset;
  allTags: TagCount[]; // タグサジェストの候補(ライブラリで使用中のタグ、件数付き)
  generating: boolean; // このアセットのジョブが実行中か
  jobError: string | null; // 生成キューの失敗(一覧と同じ内容をここでも出す)
  onBack: () => void;
  onGenerate: (asset: Asset) => void;
  onTagsChanged: (savedTags: string[]) => void; // タグ保存後に一覧・サイドバーを更新する
};

type View = 'preview' | FileView;

// パンくずの末尾(デザイン: SOURCE / PROPS / WOODEN CHAIR / TEXTURES)
const VIEW_CRUMB: Record<FileView, string> = {
  textures: 'TEXTURES',
  references: 'REFERENCES',
  renders: 'RENDERS',
  notes: 'NOTES.MD',
};

// ファイル一覧の行から開けるビュー。ディレクトリ名は library の作成規約に対応する。
const FILE_VIEWS: Record<string, FileView> = {
  textures: 'textures',
  references: 'references',
  renders: 'renders',
  'notes.md': 'notes',
};

const FILE_ICONS: Record<string, LucideIcon> = {
  'model.blend': Box,
  textures: ImageIcon,
  references: Images,
  renders: Aperture,
  'notes.md': FileText,
};

// デザインのファイル一覧の並び。ここに無い名前は後ろへ名前順で続く。
const FILE_ORDER = ['model.blend', 'textures', 'references', 'renders', 'notes.md'];

type Row = { entry: FileEntry; icon?: LucideIcon; view?: View };

// デザインでは model.blend の直後に GLB キャッシュが並ぶ
function fileRows(files: AssetFiles, title: string): Row[] {
  const rank = (name: string) => {
    const i = FILE_ORDER.indexOf(name);
    return i === -1 ? FILE_ORDER.length : i;
  };
  const rows: Row[] = [...files.entries]
    .sort((a, b) => rank(a.name) - rank(b.name) || a.name.localeCompare(b.name))
    .map((entry) => ({ entry, view: FILE_VIEWS[entry.name] }));
  if (files.glbSize === null) return rows;
  const glbRow: Row = {
    entry: { name: `cache/glb/${title}.glb`, size: files.glbSize, isDir: false, fileCount: 0 },
    icon: FileBox,
    view: 'preview',
  };
  const after = rows.findIndex((r) => r.entry.name === 'model.blend');
  rows.splice(after === -1 ? 0 : after + 1, 0, glbRow);
  return rows;
}

export default function AssetDetail({
  asset,
  allTags,
  generating,
  jobError,
  onBack,
  onGenerate,
  onTagsChanged,
}: Props) {
  const [metadata, setMetadata] = useState<ExtractedMetadata | null>(null);
  const [files, setFiles] = useState<AssetFiles | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [view, setView] = useState<View>('preview');

  // asset.id は再スキャンごとに変わるので、生成完了後に自動で取り直される
  useEffect(() => {
    let cancelled = false;
    setLoadError(null);
    getExtractedMetadata(asset.category, asset.title)
      .then((m) => {
        if (!cancelled) setMetadata(m);
      })
      .catch((err: unknown) => {
        // 未生成(404)は getExtractedMetadata が null にするので、ここは実エラー
        if (!cancelled) setLoadError(err instanceof Error ? err.message : String(err));
      });
    getAssetFiles(asset.category, asset.title)
      .then((f) => {
        if (!cancelled) setFiles(f);
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [asset.id, asset.category, asset.title]);

  const handleOpenBlender = async () => {
    setActionError(null);
    try {
      await postOpenBlender(asset.category, asset.title);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleReveal = async () => {
    setActionError(null);
    try {
      await postReveal(asset.category, asset.title);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    }
  };

  const glb = glbUrl(asset);
  const crumb = [
    'SOURCE',
    asset.category.toUpperCase(),
    asset.title.toUpperCase(),
    ...(view === 'preview' ? [] : [VIEW_CRUMB[view]]),
  ].join(' / ');

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-bg font-sans text-ink">
      <header className="flex shrink-0 items-center gap-3 border-b border-border bg-surface px-5 py-3">
        <IconButton icon={ArrowLeft} label="一覧へ戻る" onClick={onBack} />
        <div className="flex min-w-0 flex-col gap-[3px]">
          <p className="truncate font-mono text-[10px] leading-none text-ink-faint">{crumb}</p>
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="truncate font-heading text-lg leading-none font-bold text-ink">
              {asset.title}
            </h1>
            {asset.isStale && <StaleBadge />}
            {asset.isIncomplete && (
              <span
                className="flex shrink-0 items-center gap-1 border border-warn px-2 py-1 font-mono text-[9px] leading-none text-warn"
                title="model.blend がありません。プレビュー・生成・Blender 起動はできません"
              >
                <TriangleAlert size={10} />
                不完全
              </span>
            )}
          </div>
        </div>

        <div className="flex-1" />

        <IconButton
          icon={FolderOpen}
          label="Finderで表示"
          onClick={() => void handleReveal()}
        />
        <Button
          icon={RefreshCw}
          disabled={asset.isIncomplete || generating}
          onClick={() => onGenerate(asset)}
        >
          {generating ? '生成中…' : glb ? '再生成' : '生成'}
        </Button>
        <Button
          variant="primary"
          icon={ExternalLink}
          disabled={asset.isIncomplete}
          onClick={() => void handleOpenBlender()}
        >
          Blenderで開く
        </Button>
      </header>

      {(loadError || actionError || jobError) && (
        <div className="flex shrink-0 flex-col gap-2 border-b border-border px-5 py-2">
          {jobError && <Banner tone="error">{jobError}</Banner>}
          {loadError && <Banner tone="error">詳細情報を取得できません: {loadError}</Banner>}
          {actionError && <Banner tone="error">{actionError}</Banner>}
        </div>
      )}

      <div className="flex min-h-0 flex-1">
        <section className="flex min-w-0 flex-1 flex-col">
          {view === 'preview' ? (
            glb ? (
              <GlbViewer url={glb} sizeBytes={files?.glbSize ?? null} title={asset.title} />
            ) : (
              <EmptyViewport asset={asset} generating={generating} onGenerate={onGenerate} />
            )
          ) : (
            <FileViewer asset={asset} view={view} onReveal={() => void handleReveal()} />
          )}
        </section>

        <aside className="flex w-80 shrink-0 flex-col gap-5 overflow-y-auto border-l border-border bg-surface p-5">
          <section className="flex flex-col gap-2.5">
            <SectionLabel>メタデータ</SectionLabel>
            <div className="flex flex-col border border-border">
              <MetaRow index={0} label="OBJECTS" value={metadata?.objectCount} />
              <MetaRow index={1} label="COLLECTIONS" value={metadata?.collectionCount} />
              <MetaRow index={2} label="MATERIALS" value={metadata?.materialCount} />
              <MetaRow index={3} label="POLYGONS" value={metadata?.polygonCount} />
              <MetaRow index={4} label="TEXTURES" value={metadata?.textureCount} />
              <MetaRow
                index={5}
                label="ANIMATION"
                value={metadata ? (metadata.hasAnimation ? 'あり' : 'なし') : undefined}
              />
            </div>
          </section>

          <TagEditor
            asset={asset}
            allTags={allTags}
            onChanged={onTagsChanged}
            onError={setActionError}
          />

          <section className="flex flex-col gap-2.5">
            <SectionLabel>ファイル</SectionLabel>
            {files === null ? (
              <p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>
            ) : (
              <div className="flex flex-col gap-2">
                {fileRows(files, asset.title).map(({ entry, icon, view: rowView }) => (
                  <FileRow
                    key={entry.name}
                    entry={entry}
                    icon={icon}
                    active={rowView !== undefined && rowView === view}
                    onOpen={rowView === undefined ? undefined : () => setView(rowView)}
                  />
                ))}
              </div>
            )}
            <div className="flex flex-col gap-1 py-2 font-mono text-[10px] text-ink-faint">
              <span>更新: {formatDate(asset.updatedAt)}</span>
              <span>作成: {formatDate(asset.createdAt)}</span>
            </div>
          </section>
        </aside>
      </div>
    </div>
  );
}

// 画面09: GLB 未生成のビューポート
function EmptyViewport({
  asset,
  generating,
  onGenerate,
}: {
  asset: Asset;
  generating: boolean;
  onGenerate: (asset: Asset) => void;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2.5 bg-stage px-6 text-center">
      <Box size={32} className="text-stage-ink-faint" />
      <p className="text-sm font-semibold text-stage-ink">
        {asset.isIncomplete ? 'model.blend がありません' : 'GLBが未生成です'}
      </p>
      <p className="text-xs text-stage-ink-faint">
        {asset.isIncomplete
          ? 'model.blend を置いて再スキャンするとプレビューできます'
          : '生成するとThree.jsプレビューが表示されます'}
      </p>
      {!asset.isIncomplete && (
        <div className="mt-1.5">
          <Button variant="primary" icon={Zap} disabled={generating} onClick={() => onGenerate(asset)}>
            {generating ? '生成中…' : '生成'}
          </Button>
        </div>
      )}
    </div>
  );
}

// メタデータ表の 1 行(奇数行 surface-2 / 偶数行 surface のストライプ)
function MetaRow({
  index,
  label,
  value,
}: {
  index: number;
  label: string;
  value: number | string | undefined;
}) {
  return (
    <div
      className={cx(
        'flex items-center justify-between gap-3 px-3 py-2',
        index % 2 === 0 ? 'bg-surface-2' : 'bg-surface',
      )}
    >
      <span className="font-mono text-[10px] tracking-[0.5px] text-ink-faint">{label}</span>
      <span className="font-mono text-[11px] text-ink">
        {value === undefined ? '—' : typeof value === 'number' ? formatNumber(value) : value}
      </span>
    </div>
  );
}

function FileRow({
  entry,
  icon,
  active,
  onOpen,
}: {
  entry: FileEntry;
  icon?: LucideIcon;
  active: boolean;
  onOpen?: () => void;
}) {
  const Icon = icon ?? FILE_ICONS[entry.name] ?? FileText;
  const label = entry.isDir ? `${entry.name}/ (${entry.fileCount})` : entry.name;
  const content = (
    <>
      <span className="flex min-w-0 items-center gap-2">
        <Icon size={13} className={cx('shrink-0', active ? 'text-accent' : 'text-ink-muted')} />
        <span
          className={cx('truncate font-mono text-[11px]', active ? 'text-accent' : 'text-ink-muted')}
        >
          {label}
        </span>
      </span>
      <span className="shrink-0 font-mono text-[10px] text-ink-faint">
        {formatSize(entry.size)}
      </span>
    </>
  );

  if (!onOpen) {
    return <div className="flex items-center justify-between gap-2">{content}</div>;
  }
  return (
    <button
      type="button"
      onClick={onOpen}
      title={`${label} を開く`}
      className="flex items-center justify-between gap-2 text-left transition hover:opacity-80"
    >
      {content}
    </button>
  );
}

function TagEditor({
  asset,
  allTags,
  onChanged,
  onError,
}: {
  asset: Asset;
  allTags: TagCount[];
  onChanged: (savedTags: string[]) => void;
  onError: (message: string) => void;
}) {
  // 表示はローカル state を正とし、保存レスポンス(正規化済み)で即時更新する
  // (プロップの asset.tags は一覧の再読込まで古いままのため)
  const [tags, setTags] = useState(asset.tags);
  const [input, setInput] = useState('');
  const [adding, setAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setTags(asset.tags);
  }, [asset.id, asset.tags]);

  const save = async (next: string[]) => {
    setSaving(true);
    try {
      const res = await putTags(asset.category, asset.title, next);
      setTags(res.tags);
      onChanged(res.tags);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="flex flex-col gap-2.5">
      <SectionLabel>タグ</SectionLabel>
      <div className="flex flex-wrap items-center gap-1.5">
        {tags.map((tag) => (
          <TagChip
            key={tag}
            name={tag}
            disabled={saving}
            onRemove={() => void save(tags.filter((t) => t !== tag))}
          />
        ))}
        {adding ? (
          <TagSuggestInput
            allTags={allTags}
            exclude={tags}
            value={input}
            onValueChange={setInput}
            onAdd={(tag) => void save([...tags, tag])}
            frozen={saving}
            autoFocus
            // 連続追加のため追加後も開いたままにし、Escape・欄外クリック・空 Enter で終える
            onDone={() => setAdding(false)}
          />
        ) : (
          <Chip title="タグを追加" onClick={() => setAdding(true)}>
            <Plus size={11} />
          </Chip>
        )}
      </div>
    </section>
  );
}
