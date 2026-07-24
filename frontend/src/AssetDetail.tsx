import { useEffect, useState } from 'react';
import {
  getAssetFiles,
  getExtractedMetadata,
  glbUrl,
  postOpenBlender,
  putTags,
  type Asset,
  type AssetFiles,
  type ExtractedMetadata,
} from './api';
import { formatSize } from './format';
import GlbViewer from './GlbViewer';

type Props = {
  asset: Asset;
  generating: boolean; // このアセットのジョブが実行中か
  onBack: () => void;
  onGenerate: (asset: Asset) => void;
  onTagsChanged: (savedTags: string[]) => void; // タグ保存後に一覧・サイドバーを更新する
};

export default function AssetDetail({ asset, generating, onBack, onGenerate, onTagsChanged }: Props) {
  const [metadata, setMetadata] = useState<ExtractedMetadata | null>(null);
  const [files, setFiles] = useState<AssetFiles | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

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

  const glb = glbUrl(asset);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3">
        <button
          type="button"
          className="rounded border border-neutral-300 px-3 py-2 text-sm hover:bg-neutral-100 dark:border-neutral-600 dark:hover:bg-neutral-800"
          onClick={onBack}
        >
          ← 一覧へ
        </button>
        <div className="min-w-0">
          <h2 className="truncate text-xl font-bold">{asset.title}</h2>
          <p className="text-sm text-neutral-500 dark:text-neutral-400">{asset.category}</p>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            className="rounded border border-neutral-300 px-3 py-2 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-600 dark:hover:bg-neutral-800"
            disabled={asset.isIncomplete || generating}
            onClick={() => onGenerate(asset)}
          >
            {generating ? '生成中…' : glb ? '再生成' : '生成'}
          </button>
          <button
            type="button"
            className="rounded bg-orange-600 px-3 py-2 text-sm font-medium text-white hover:bg-orange-500 disabled:opacity-50"
            disabled={asset.isIncomplete}
            onClick={() => void handleOpenBlender()}
          >
            Blenderで開く
          </button>
        </div>
      </div>

      {loadError && (
        <p className="rounded border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
          詳細情報を取得できません: {loadError}
        </p>
      )}
      {actionError && (
        <p className="rounded border border-red-300 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
          {actionError}
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        {/* プレビュー */}
        <div className="h-[420px] overflow-hidden rounded-lg border border-neutral-300 bg-neutral-100 dark:border-neutral-700 dark:bg-neutral-950">
          {glb ? (
            <GlbViewer url={glb} />
          ) : (
            <div className="flex h-full flex-col items-center justify-center gap-3">
              <p className="text-sm text-neutral-500 dark:text-neutral-400">
                {asset.isIncomplete
                  ? 'model.blend が無いためプレビューできません'
                  : 'GLB が未生成です'}
              </p>
              {!asset.isIncomplete && (
                <button
                  type="button"
                  className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50"
                  disabled={generating}
                  onClick={() => onGenerate(asset)}
                >
                  {generating ? '生成中…' : '生成する'}
                </button>
              )}
            </div>
          )}
        </div>

        <div className="flex flex-col gap-4">
          {/* タグ(meta.json に保存) */}
          <TagEditor asset={asset} onChanged={onTagsChanged} onError={setActionError} />

          {/* 抽出メタデータ */}
          <section className="rounded-lg border border-neutral-300 bg-white p-4 dark:border-neutral-700 dark:bg-neutral-800">
            <h3 className="mb-2 text-sm font-semibold">抽出メタデータ</h3>
            <table className="w-full text-sm">
              <tbody>
                <MetaRow label="Object 数" value={metadata?.objectCount} />
                <MetaRow label="Collection 数" value={metadata?.collectionCount} />
                <MetaRow label="Material 数" value={metadata?.materialCount} />
                <MetaRow label="Polygon 数" value={metadata?.polygonCount} />
                <MetaRow label="Texture 数" value={metadata?.textureCount} />
                <MetaRow
                  label="Animation"
                  value={metadata ? (metadata.hasAnimation ? 'あり' : 'なし') : undefined}
                />
              </tbody>
            </table>
          </section>

          {/* ファイル一覧 */}
          <section className="rounded-lg border border-neutral-300 bg-white p-4 dark:border-neutral-700 dark:bg-neutral-800">
            <h3 className="mb-2 text-sm font-semibold">ファイル</h3>
            {files === null ? (
              <p className="text-sm text-neutral-500">読み込み中…</p>
            ) : (
              <table className="w-full text-sm">
                <tbody>
                  {files.entries.map((e) => (
                    <tr key={e.name} className="border-b border-neutral-200 last:border-0 dark:border-neutral-700">
                      <td className="py-1 pr-2">
                        {e.name}
                        {e.isDir && (
                          <span className="ml-1 text-xs text-neutral-500 dark:text-neutral-400">
                            ({e.fileCount} ファイル)
                          </span>
                        )}
                      </td>
                      <td className="py-1 text-right text-neutral-500 dark:text-neutral-400">
                        {formatSize(e.size)}
                      </td>
                    </tr>
                  ))}
                  {files.glbSize !== null && (
                    <tr>
                      <td className="py-1 pr-2">GLB(キャッシュ)</td>
                      <td className="py-1 text-right text-neutral-500 dark:text-neutral-400">
                        {formatSize(files.glbSize)}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

function TagEditor({
  asset,
  onChanged,
  onError,
}: {
  asset: Asset;
  onChanged: (savedTags: string[]) => void;
  onError: (message: string) => void;
}) {
  // 表示はローカル state を正とし、保存レスポンス(正規化済み)で即時更新する
  // (プロップの asset.tags は一覧の再読込まで古いままのため)
  const [tags, setTags] = useState(asset.tags);
  const [input, setInput] = useState('');
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

  const addTag = () => {
    const tag = input.trim();
    if (tag === '' || tags.includes(tag)) {
      setInput('');
      return;
    }
    setInput('');
    void save([...tags, tag]);
  };

  return (
    <section className="rounded-lg border border-neutral-300 bg-white p-4 dark:border-neutral-700 dark:bg-neutral-800">
      <h3 className="mb-2 text-sm font-semibold">タグ</h3>
      <div className="flex flex-wrap gap-1.5">
        {tags.map((tag) => (
          <span
            key={tag}
            className="inline-flex items-center gap-1 rounded-full bg-neutral-200 px-2.5 py-0.5 text-xs dark:bg-neutral-700"
          >
            {tag}
            <button
              type="button"
              className="text-neutral-500 hover:text-red-600 dark:text-neutral-400 dark:hover:text-red-400"
              aria-label={`タグ ${tag} を削除`}
              disabled={saving}
              onClick={() => void save(tags.filter((t) => t !== tag))}
            >
              ×
            </button>
          </span>
        ))}
        {tags.length === 0 && (
          <span className="text-xs text-neutral-500 dark:text-neutral-400">タグはありません</span>
        )}
      </div>
      <div className="mt-2 flex gap-2">
        <input
          type="text"
          className="w-full rounded border border-neutral-300 bg-white px-2 py-1 text-sm dark:border-neutral-600 dark:bg-neutral-900"
          placeholder="タグを追加…"
          value={input}
          disabled={saving}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            // IME の変換確定 Enter では追加しない
            if (e.key === 'Enter' && !e.nativeEvent.isComposing) addTag();
          }}
        />
        <button
          type="button"
          className="rounded border border-neutral-300 px-3 py-1 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-600 dark:hover:bg-neutral-700"
          disabled={saving || input.trim() === ''}
          onClick={addTag}
        >
          追加
        </button>
      </div>
    </section>
  );
}

function MetaRow({ label, value }: { label: string; value: number | string | undefined }) {
  return (
    <tr className="border-b border-neutral-200 last:border-0 dark:border-neutral-700">
      <td className="py-1 pr-2 text-neutral-500 dark:text-neutral-400">{label}</td>
      <td className="py-1 text-right">{value ?? '—'}</td>
    </tr>
  );
}

