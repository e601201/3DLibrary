import type { Asset } from './api';

export type ViewMode = 'card' | 'list';

type Props = {
  assets: Asset[];
  view: ViewMode;
  filtered: boolean; // 検索・フィルタが効いているか(空表示の文言用)
};

export default function AssetGrid({ assets, view, filtered }: Props) {
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
          <AssetRow key={asset.id} asset={asset} />
        ))}
      </ul>
    );
  }
  return (
    <ul className="grid w-full grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {assets.map((asset) => (
        <AssetCard key={asset.id} asset={asset} />
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

function AssetCard({ asset }: { asset: Asset }) {
  return (
    <li className="overflow-hidden rounded-lg border border-neutral-300 bg-white dark:border-neutral-700 dark:bg-neutral-800">
      <div className="relative flex aspect-square items-center justify-center bg-neutral-100 dark:bg-neutral-900">
        {/* thumbnailPath はサーバー側のファイルパス。画像として表示するには
            配信 API が必要で、それは生成チケット(#6)で入る。それまでは
            常にプレースホルダーを表示する */}
        <Placeholder className="text-4xl" />
        {asset.isIncomplete && <IncompleteBadge className="absolute left-2 top-2" />}
      </div>
      <div className="p-3">
        <p className="truncate text-sm font-medium" title={asset.title}>
          {asset.title}
        </p>
        <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{asset.category}</p>
        <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
          ポリゴン数: {asset.polygonCount ?? '—'}
        </p>
      </div>
    </li>
  );
}

function AssetRow({ asset }: { asset: Asset }) {
  return (
    <li className="flex items-center gap-3 px-4 py-2">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded bg-neutral-100 dark:bg-neutral-900">
        <Placeholder className="text-xl" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" title={asset.title}>
          {asset.title}
        </p>
        <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{asset.category}</p>
      </div>
      {asset.isIncomplete && <IncompleteBadge />}
      <span className="hidden w-24 text-right text-xs text-neutral-500 sm:block dark:text-neutral-400">
        {formatSize(asset.size)}
      </span>
      <span className="hidden w-36 text-right text-xs text-neutral-500 sm:block dark:text-neutral-400">
        {formatDate(asset.updatedAt)}
      </span>
    </li>
  );
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(iso: string): string {
  // 不完全アセットの updatedAt は Go のゼロ値(0001-01-01…)で来る
  if (iso.startsWith('0001-')) return '—';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
