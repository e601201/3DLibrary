// design/Design.pen 画面01 のカードグリッドとリスト表示。
// カードはサムネイル(3:2)+ 情報部(padding 12-14 / gap 8)の 2 段構成。

import { ImageOff, RefreshCw, TriangleAlert, Zap } from 'lucide-react';
import { useRef, useState } from 'react';
import { spriteUrl, thumbnailUrl, type Asset } from './api';
import { formatDate, formatDay, formatPolygons, formatSize } from './format';
import {
  canHoverScrub,
  frameAtX,
  framePosition,
  scrubRatio,
  SPRITE_BACKGROUND_SIZE,
} from './sprite';
import { Centered, cx } from './ui';

export type ViewMode = 'card' | 'list';

type Props = {
  assets: Asset[];
  view: ViewMode;
  filtered: boolean; // 検索・フィルタが効いているか(空表示の文言用)
  onGenerate: (asset: Asset) => void;
  onSelect: (asset: Asset) => void;
  runningKey: string | null; // 生成実行中のアセット(category/title)
};

export default function AssetGrid({
  assets,
  view,
  filtered,
  onGenerate,
  onSelect,
  runningKey,
}: Props) {
  if (assets.length === 0) {
    return (
      <Centered>
        <ImageOff size={32} className="text-ink-faint" />
        <p className="text-sm font-semibold text-ink">
          {filtered ? '条件に一致するアセットがありません' : 'アセットがありません'}
        </p>
        {!filtered && (
          <p className="max-w-md text-xs text-ink-faint">
            source のカテゴリディレクトリ直下にアセットディレクトリを追加して「再スキャン」してください。
          </p>
        )}
      </Centered>
    );
  }

  if (view === 'list') {
    return (
      <ul className="border border-border bg-surface">
        {assets.map((asset) => (
          <AssetRow key={asset.id} asset={asset} onSelect={onSelect} />
        ))}
      </ul>
    );
  }

  return (
    <ul className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-5">
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

// 「要更新」バッジ。サムネイル上(onStage)はデザインどおり
// fill #0A0A0ACC + 暗色前提の配色、それ以外の面ではテーマ追従の配色にする。
export function StaleBadge({ onStage = false }: { onStage?: boolean }) {
  return (
    <span
      className={cx(
        'flex shrink-0 items-center gap-1 border px-2 py-1 font-mono text-[9px] leading-none',
        onStage ? 'border-stage-warn bg-stage/80 text-stage-warn' : 'border-warn text-warn',
      )}
      title="model.blend がキャッシュより新しいか、サムネイルサイズが変更されています。生成で最新化できます"
    >
      <RefreshCw size={10} />
      要更新
    </span>
  );
}

// ホバー位置スクラブ(PRD hover-scrub-preview)。ホバー中だけスプライトを
// 取得し、デコードが済んで初めてカーソル位置のフレームを返す。それまで
// (と非対応環境・スプライト未生成)は null で、カードは静止サムネイルのまま。
function useSpriteScrub(url: string | null) {
  const [position, setPosition] = useState<{ frame: number; ratio: number } | null>(null);
  const [decoded, setDecoded] = useState(false);
  const [hoverCapable] = useState(canHoverScrub);
  // 取得中・取得済みのスプライト。ホバー終了で捨てる(JS 側で溜めない)
  const sprite = useRef<HTMLImageElement | null>(null);

  const track = (e: React.MouseEvent<HTMLElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    setPosition({ frame: frameAtX(x, rect.width), ratio: scrubRatio(x, rect.width) });
  };

  if (url === null || !hoverCapable) {
    return { frame: null, ratio: 0, handlers: {} };
  }
  return {
    frame: decoded && position ? position.frame : null,
    ratio: position ? position.ratio : 0,
    handlers: {
      onMouseEnter: (e: React.MouseEvent<HTMLElement>) => {
        track(e);
        // 一覧の初期表示では取得しない。ここが唯一の取得点
        const image = new Image();
        sprite.current = image;
        image.src = url;
        // デコードが済むまではサムネイルを出し続ける(スピナーは出さない)。
        // 失敗したらそのまま静止サムネイルに留まる
        image.decode().then(
          () => sprite.current === image && setDecoded(true),
          () => {},
        );
      },
      onMouseMove: track,
      onMouseLeave: () => {
        sprite.current = null;
        setDecoded(false);
        setPosition(null);
      },
    },
  };
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
  const sprite = spriteUrl(asset);
  // スクラブはサムネイルの差し替えなので、サムネイルのあるカードだけが対象
  const scrub = useSpriteScrub(!asset.isIncomplete && thumb ? sprite : null);
  return (
    <li
      className="group flex cursor-pointer flex-col border border-border bg-surface transition hover:border-ink-faint"
      onClick={() => onSelect(asset)}
    >
      <div
        className={cx(
          'relative flex aspect-[3/2] items-center justify-center overflow-hidden bg-surface-2',
          // 画像は縁まで使う。プレースホルダだけデザインどおり内側に余白を取る
          !asset.isIncomplete && thumb ? '' : 'p-2',
        )}
        {...scrub.handlers}
      >
        {asset.isIncomplete ? (
          <div className="flex flex-col items-center gap-2 text-center">
            <TriangleAlert size={22} className="text-warn" />
            <span className="font-mono text-[10px] text-warn">model.blend がありません</span>
          </div>
        ) : thumb ? (
          <img
            src={thumb}
            alt={asset.title}
            className={cx(
              'h-full w-full object-contain',
              // スプライトは背景透過なので、下にサムネイルを残すと
              // 別角度の像が透けて二重に見える。スクラブ中は隠す
              // (フレーム 0 = サムネイルなので切り替わりは見えない)
              scrub.frame !== null && 'invisible',
            )}
            loading="lazy"
          />
        ) : (
          <div className="flex flex-col items-center gap-2 text-center">
            <ImageOff size={22} className="text-ink-faint" />
            <span className="font-mono text-[10px] text-ink-faint">サムネイル未生成</span>
            <GenerateButton asset={asset} generating={generating} onGenerate={onGenerate} />
          </div>
        )}

        {scrub.frame !== null && sprite && (
          <>
            {/* デコード済みシートの background-position を差し替えるだけで
                フレームを切り替える(再デコード・再レイアウトを起こさない)。
                正方形のフレームを高さ基準で中央に置き、サムネイルの
                object-contain と同じ収まりにする */}
            <div className="pointer-events-none absolute inset-0 flex justify-center">
              <div
                className="aspect-square h-full"
                style={{
                  backgroundImage: `url(${sprite})`,
                  backgroundSize: SPRITE_BACKGROUND_SIZE,
                  backgroundPosition: framePosition(scrub.frame),
                  backgroundRepeat: 'no-repeat',
                }}
              />
            </div>
            {/* 操作を持たない下縁のポジションバー(カーソル位置の目印) */}
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-[3px] bg-border">
              <div className="h-full bg-accent" style={{ width: `${scrub.ratio * 100}%` }} />
            </div>
          </>
        )}

        {asset.isStale && (
          <div className="absolute top-2 right-2">
            <StaleBadge onStage />
          </div>
        )}

        {/* サムネイルがある場合の再生成はホバーでだけ出す(デザインの
            静止状態を保ちつつ、要更新アセットを個別に直せるように) */}
        {!asset.isIncomplete && thumb && (
          <div className="absolute right-2 bottom-2 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100">
            <GenerateButton
              asset={asset}
              generating={generating}
              onGenerate={onGenerate}
              label={asset.isStale ? '更新' : '再生成'}
            />
          </div>
        )}
      </div>

      <div className="flex flex-col gap-2 px-3.5 py-3">
        <p className="truncate text-sm font-semibold text-ink" title={asset.title}>
          {asset.title}
        </p>
        <div className="flex items-center justify-between gap-2">
          <span className="truncate font-mono text-[10px] tracking-[0.5px] text-accent uppercase">
            {asset.category}
          </span>
          <span className="shrink-0 font-mono text-[10px] whitespace-nowrap text-ink-faint">
            {formatPolygons(asset.polygonCount)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-2 font-mono text-[10px] text-ink-faint">
          <span>{formatDay(asset.updatedAt)}</span>
          <span>{formatSize(asset.size, true)}</span>
        </div>
      </div>
    </li>
  );
}

// サムネイル未生成カードの小さな生成ボタン(padding 5-10 / gap 5)
function GenerateButton({
  asset,
  generating,
  onGenerate,
  label = '生成',
}: {
  asset: Asset;
  generating: boolean;
  onGenerate: (asset: Asset) => void;
  label?: string;
}) {
  return (
    <button
      type="button"
      disabled={generating}
      title="GLB・サムネイル・抽出メタデータを生成"
      className="flex items-center gap-1.5 border border-border bg-surface px-2.5 py-[5px] text-[11px] leading-none text-ink-muted transition hover:border-accent hover:text-ink disabled:cursor-not-allowed disabled:opacity-50"
      onClick={(e) => {
        e.stopPropagation(); // カードクリック(詳細遷移)と干渉させない
        onGenerate(asset);
      }}
    >
      <Zap size={11} className="text-accent" />
      {generating ? '生成中…' : label}
    </button>
  );
}

function AssetRow({ asset, onSelect }: { asset: Asset; onSelect: (asset: Asset) => void }) {
  const thumb = thumbnailUrl(asset);
  return (
    <li
      className="flex cursor-pointer items-center gap-3 border-b border-border px-4 py-2 transition last:border-b-0 hover:bg-surface-2"
      onClick={() => onSelect(asset)}
    >
      <div className="flex size-10 shrink-0 items-center justify-center overflow-hidden bg-surface-2">
        {thumb ? (
          <img src={thumb} alt="" className="h-full w-full object-contain" loading="lazy" />
        ) : (
          <ImageOff size={14} className={cx(asset.isIncomplete ? 'text-warn' : 'text-ink-faint')} />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-[13px] font-medium text-ink" title={asset.title}>
          {asset.title}
        </p>
        <p className="truncate font-mono text-[10px] tracking-[0.5px] text-accent uppercase">
          {asset.category}
        </p>
      </div>
      {asset.isIncomplete && (
        <span
          className="flex shrink-0 items-center gap-1 border border-warn px-2 py-1 font-mono text-[9px] leading-none text-warn"
          title="model.blend がありません。プレビュー・生成・Blender 起動はできません"
        >
          <TriangleAlert size={10} />
          不完全
        </span>
      )}
      {asset.isStale && <StaleBadge />}
      <span className="hidden w-28 shrink-0 text-right font-mono text-[10px] text-ink-faint sm:block">
        {formatPolygons(asset.polygonCount)}
      </span>
      <span className="hidden w-20 shrink-0 text-right font-mono text-[10px] text-ink-faint sm:block">
        {formatSize(asset.size, true)}
      </span>
      <span className="hidden w-36 shrink-0 text-right font-mono text-[10px] text-ink-faint md:block">
        {formatDate(asset.updatedAt)}
      </span>
    </li>
  );
}
