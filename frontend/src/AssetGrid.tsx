import { thumbnailUrl, type Asset } from './api';
import { formatDate, formatSize } from './format';

export type ViewMode = 'card' | 'list';

type Props = {
  assets: Asset[];
  view: ViewMode;
  filtered: boolean; // 検索・フィルタが効いているか(空表示の文言用)
  onGenerate: (asset: Asset) => void;
  onSelect: (asset: Asset) => void;
  runningKey: string | null; // 生成実行中のアセット(category/title)
};

export default function AssetGrid({ assets, view, filtered, onGenerate, onSelect, runningKey }: Props) {
  if (assets.length === 0) {
    return (
      <p className="py-12 text-center text-sm text-neutral-500 dark:text-neutral-400">
        {filtered
          ? '条件に一致するアセットがありません。'
          : 'アセットがありません。source のカテゴリディレクトリ直下にアセットディレクトリを追加して「再スキャン」してください。'}
      </p>
    );
  }
  if (view === 'list') {
    return (
      <ul className="divide-y divide-neutral-200 rounded-lg border border-neutral-300 bg-white dark:divide-neutral-700 dark:border-neutral-700 dark:bg-neutral-800">
        {assets.map((asset) => (
          <AssetRow key={asset.id} asset={asset} onSelect={onSelect} />
        ))}
      </ul>
    );
  }
  return (
    <ul className="grid w-full grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {assets.map((asset) => (
        <AssetCard
          key={asset.id}
          asset={asset}
          onGenerate={onGenerate}
          onSelect={onSelect}
          generating={runningKey === `${asset.category}/${asset.title}`}
        />
      ))}
    </ul>
  );
}

function IncompleteBadge({ className = '' }: { className?: string }) {
  return (
    <span
      className={`rounded bg-amber-500/90 px-1.5 py-0.5 text-xs font-medium text-white ${className}`}
      title="model.blend がありません。プレビュー・生成・Blender 起動はできません"
    >
      ⚠ 不完全
    </span>
  );
}

export function StaleBadge({ className = '' }: { className?: string }) {
  return (
    <span
      className={`rounded bg-red-500/90 px-1.5 py-0.5 text-xs font-medium text-white ${className}`}
      title="model.blend がキャッシュより新しいか、サムネイルサイズが変更されています。生成で最新化できます"
    >
      要更新
    </span>
  );
}

function Placeholder({ className = '' }: { className?: string }) {
  return (
    <span
      className={`select-none text-neutral-300 dark:text-neutral-600 ${className}`}
      aria-label="サムネイル未生成"
    >
      ◇
    </span>
  );
}

function AssetCard({
  asset,
  onGenerate,
  onSelect,
  generating,
}: {
  asset: Asset;
  onGenerate: (asset: Asset) => void;
  onSelect: (asset: Asset) => void;
  generating: boolean;
}) {
  const thumb = thumbnailUrl(asset);
  return (
    <li
      className="cursor-pointer overflow-hidden rounded-lg border border-neutral-300 bg-white transition-shadow hover:shadow-md dark:border-neutral-700 dark:bg-neutral-800"
      onClick={() => onSelect(asset)}
    >
      <div className="relative flex aspect-square items-center justify-center bg-neutral-100 dark:bg-neutral-900">
        {thumb ? (
          <img src={thumb} alt={asset.title} className="h-full w-full object-contain" loading="lazy" />
        ) : (
          <Placeholder className="text-4xl" />
        )}
        {asset.isIncomplete && <IncompleteBadge className="absolute left-2 top-2" />}
        {asset.isStale && <StaleBadge className="absolute right-2 top-2" />}
      </div>
      <div className="p-3">
        <p className="truncate text-sm font-medium" title={asset.title}>
          {asset.title}
        </p>
        <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{asset.category}</p>
        <div className="mt-1 flex items-center justify-between">
          <span className="text-xs text-neutral-500 dark:text-neutral-400">
            ポリゴン数: {asset.polygonCount ?? '—'}
          </span>
          <button
            type="button"
            className="rounded border border-neutral-300 px-2 py-1 text-xs hover:bg-neutral-100 disabled:cursor-not-allowed disabled:opacity-40 dark:border-neutral-600 dark:hover:bg-neutral-700"
            disabled={asset.isIncomplete || generating}
            title={asset.isIncomplete ? 'model.blend が無いため生成できません' : 'GLB・サムネイル・抽出メタデータを生成'}
            onClick={(e) => {
              e.stopPropagation(); // カードクリック(詳細遷移)と干渉させない
              onGenerate(asset);
            }}
          >
            {generating ? '生成中…' : '生成'}
          </button>
        </div>
      </div>
    </li>
  );
}

function AssetRow({ asset, onSelect }: { asset: Asset; onSelect: (asset: Asset) => void }) {
  return (
    <li
      className="flex cursor-pointer items-center gap-3 px-4 py-2 hover:bg-neutral-50 dark:hover:bg-neutral-700/50"
      onClick={() => onSelect(asset)}
    >
      <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded bg-neutral-100 dark:bg-neutral-900">
        {thumbnailUrl(asset) ? (
          <img src={thumbnailUrl(asset)!} alt="" className="h-full w-full object-contain" loading="lazy" />
        ) : (
          <Placeholder className="text-xl" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" title={asset.title}>
          {asset.title}
        </p>
        <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{asset.category}</p>
      </div>
      {asset.isIncomplete && <IncompleteBadge />}
      {asset.isStale && <StaleBadge />}
      <span className="hidden w-24 text-right text-xs text-neutral-500 sm:block dark:text-neutral-400">
        {formatSize(asset.size, true)}
      </span>
      <span className="hidden w-36 text-right text-xs text-neutral-500 sm:block dark:text-neutral-400">
        {formatDate(asset.updatedAt)}
      </span>
    </li>
  );
}

