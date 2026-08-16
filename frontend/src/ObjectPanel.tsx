// ビューポート左上のオブジェクト一覧(CONTEXT.md「オブジェクト非表示」)。
// GLB ビューワに重なるので、配色は stage-* 固定。
//
// three との受け渡しは GlbViewer の ViewerApi が持ち、こちらは並べて
// 操作を返すだけに徹する(メッシュ実体には触れない)。

import { useEffect, useRef } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { cx } from './ui';

// 一覧に並べるのは描画されるオブジェクトだけ。ボーンや Empty は
// 隠しても何も消えないので、GlbViewer 側で除いてから渡す
export type ViewerObject = {
  id: string; // three の uuid。選択・非表示の突き合わせに使う
  name: string; // 表示名(Blender が付けた元の名前)
};

export function ObjectPanel({
  objects,
  hidden,
  selectedObjectId,
  onSelect,
  onToggle,
  onIsolate,
  onHover,
  onShowAll,
}: {
  objects: ViewerObject[];
  hidden: string[];
  selectedObjectId: string | null;
  onSelect: (objectId: string) => void;
  onToggle: (objectId: string) => void;
  onIsolate: (objectId: string) => void; // これだけ表示(既に単独なら全表示へ戻す)
  onHover: (objectId: string | null) => void;
  onShowAll: () => void;
}) {
  // ビューポートのクリックで選ばれた行が一覧の外にあると探せないので、
  // 選択が変わったらその行まで送る(74 オブジェクトのアセットで効く)
  const rows = useRef(new Map<string, HTMLDivElement>());
  useEffect(() => {
    if (selectedObjectId === null) return;
    rows.current.get(selectedObjectId)?.scrollIntoView({ block: 'nearest' });
  }, [selectedObjectId]);

  return (
    <div
      className="pointer-events-auto flex max-h-[22rem] w-64 flex-col gap-2 overflow-y-auto border border-stage-border bg-stage/90 p-3 backdrop-blur-sm"
      // 行から行へ移る隙間でホバー枠が点滅しないよう、外れる判定はパネル単位で行う
      onPointerLeave={() => onHover(null)}
    >
      <div className="flex items-baseline justify-between gap-2">
        {/* 総数は出さない。サイドバーの OBJECTS は .blend の全オブジェクト数
            (カメラやライトも含む)で、GLB の中身とは数が合わないため */}
        <p className="font-mono text-[10px] leading-none tracking-[1px] text-stage-ink-faint">
          OBJECTS{' '}
          {hidden.length > 0 && <span className="text-stage-accent">{hidden.length} 非表示</span>}
        </p>
        {/* 押しても何も起きないボタンを常設しないよう、隠れているときだけ出す */}
        {hidden.length > 0 && (
          <button
            type="button"
            onClick={onShowAll}
            title="すべて表示 (Alt+H)"
            className="flex shrink-0 items-center gap-1 font-mono text-[10px] leading-none text-stage-ink-muted transition hover:text-stage-ink"
          >
            <Eye size={11} />
            すべて表示
          </button>
        )}
      </div>

      <div className="flex flex-col">
        {objects.map((object) => {
          const isHidden = hidden.includes(object.id);
          const picked = object.id === selectedObjectId;
          return (
            <div
              key={object.id}
              ref={(el) => {
                if (el) rows.current.set(object.id, el);
                else rows.current.delete(object.id);
              }}
              className="flex items-center gap-1"
              // 隠れている行でも枠は出す。名前だけでは戻すべき対象か
              // 判断できないので、位置と大きさを手がかりにする
              onPointerEnter={() => onHover(object.id)}
            >
              <button
                type="button"
                aria-pressed={picked}
                // 隠れているものは「いないもの」として扱うので選択させない
                disabled={isHidden}
                onClick={() => onSelect(object.id)}
                title={
                  isHidden
                    ? '非表示中は選択できません'
                    : picked
                      ? '選択を解除'
                      : `${object.name} を選択`
                }
                className={cx(
                  'min-w-0 flex-1 truncate px-1 py-1.5 text-left font-mono text-[11px] leading-none transition',
                  picked ? 'text-stage-accent' : 'text-stage-ink-muted',
                  isHidden
                    ? 'cursor-not-allowed opacity-40'
                    : !picked && 'hover:text-stage-ink',
                )}
              >
                {object.name}
              </button>
              <button
                type="button"
                aria-pressed={!isHidden}
                aria-label={`${object.name} の表示切替`}
                title="クリック: 表示切替 / Ctrl+クリック: これだけ表示"
                onClick={(e) => (e.ctrlKey || e.metaKey ? onIsolate(object.id) : onToggle(object.id))}
                className={cx(
                  'shrink-0 p-1 transition',
                  isHidden
                    ? 'text-stage-ink-faint hover:text-stage-ink'
                    : 'text-stage-ink-muted hover:text-stage-ink',
                )}
              >
                {isHidden ? <EyeOff size={12} /> : <Eye size={12} />}
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}
