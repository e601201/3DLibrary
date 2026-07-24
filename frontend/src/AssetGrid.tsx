import type { Asset } from './api';

type Props = {
  assets: Asset[];
};

export default function AssetGrid({ assets }: Props) {
  if (assets.length === 0) {
    return (
      <p className="py-12 text-center text-sm text-neutral-500 dark:text-neutral-400">
        アセットがありません。source のカテゴリディレクトリ直下にアセットディレクトリを追加して「再スキャン」してください。
      </p>
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

function AssetCard({ asset }: { asset: Asset }) {
  return (
    <li className="overflow-hidden rounded-lg border border-neutral-300 bg-white dark:border-neutral-700 dark:bg-neutral-800">
      <div className="relative flex aspect-square items-center justify-center bg-neutral-100 dark:bg-neutral-900">
        {/* thumbnailPath はサーバー側のファイルパス。画像として表示するには
            配信 API が必要で、それは生成チケット(#6)で入る。それまでは
            常にプレースホルダーを表示する */}
        <span
          className="select-none text-4xl text-neutral-300 dark:text-neutral-600"
          aria-label="サムネイル未生成"
        >
          ◇
        </span>
        {asset.isIncomplete && (
          <span
            className="absolute left-2 top-2 rounded bg-amber-500/90 px-1.5 py-0.5 text-xs font-medium text-white"
            title="model.blend がありません。プレビュー・生成・Blender 起動はできません"
          >
            ⚠ 不完全
          </span>
        )}
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
