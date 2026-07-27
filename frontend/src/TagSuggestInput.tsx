// タグ入力のサジェスト付き入力欄。既存タグを使用回数の多い順に出し、部分一致
// (大文字小文字無視)で絞り込む。保存の方針は持たず、確定したタグ名を onAdd で
// 親へ渡すだけの表示部品にしてある(詳細画面は 1 追加 = 1 保存、新規作成フォームは
// 送信までローカルに溜める、という違いを親側に置くため)。

import { useId, useRef, useState } from 'react';
import { X } from 'lucide-react';
import type { TagCount } from './api';
import { TEXT_INPUT_CLASS, cx } from './ui';

// chip は既存タグの並びに混ざる小さな入力(詳細画面)、field はフォームの
// 1 項目としての横幅いっぱいの入力(新規作成フォーム)。
type Variant = 'chip' | 'field';

const VARIANT: Record<Variant, { wrapper: string; input: string; dropdown: string }> = {
  chip: {
    wrapper: 'relative',
    input:
      'w-28 border border-accent bg-surface-2 px-2.5 py-1 text-ink placeholder:text-ink-faint focus:outline-none',
    dropdown: 'w-48',
  },
  field: {
    wrapper: 'relative w-full',
    input: TEXT_INPUT_CLASS,
    dropdown: 'w-full',
  },
};

// 入力テキストから「追加すべきタグ」を決める。空白のみ・付与済みは追加しない。
// 入力欄での確定と、送信時の打ちかけテキストの確定で同じ規則を使うため関数にしてある。
export function pendingTagOf(value: string, exclude: string[]): string | null {
  const tag = value.trim();
  if (tag === '' || exclude.includes(tag)) return null;
  return tag;
}

export default function TagSuggestInput({
  allTags,
  exclude,
  value,
  onValueChange,
  onAdd,
  variant = 'chip',
  saving = false,
  autoFocus = false,
  onDismiss,
  onComposingChange,
  placeholder = 'タグ名',
  ariaLabel,
}: {
  allTags: TagCount[];
  exclude: string[]; // 付与済みタグ(候補から外す)
  value: string;
  onValueChange: (value: string) => void;
  onAdd: (tag: string) => void;
  variant?: Variant;
  saving?: boolean; // 保存中。入力は生かしたまま確定だけ無視する
  autoFocus?: boolean;
  onDismiss?: () => void; // 入力を終えた合図(空 Enter・欄外クリック・何も開いていない Escape)
  onComposingChange?: (composing: boolean) => void; // IME 変換中か(親が送信時の確定を抑えるため)
  placeholder?: string;
  ariaLabel?: string;
}) {
  // 候補の開閉。入力欄そのものの有無とは別で、Escape はまずこちらを閉じる
  const [open, setOpen] = useState(false);
  // キーボード選択位置。null は非選択で、Enter は入力テキストを確定する
  const [highlighted, setHighlighted] = useState<number | null>(null);
  // blur の時点で参照するため state ではなく ref(再描画を待たずに読む)
  const composing = useRef(false);
  const listboxId = useId();

  // タグの同一性は大文字小文字を区別したまま(絞り込みだけ無視する)
  const query = value.trim().toLowerCase();
  const suggestions = allTags
    .filter((t) => !exclude.includes(t.name) && t.name.toLowerCase().includes(query))
    .sort((a, b) => b.count - a.count);
  // 入力の変化で候補が縮んだとき、範囲外の選択位置は非選択に戻す
  const activeIndex = highlighted !== null && highlighted < suggestions.length ? highlighted : null;
  const showSuggestions = open && suggestions.length > 0;

  const setComposing = (next: boolean) => {
    composing.current = next;
    onComposingChange?.(next);
  };

  const commit = (name: string) => {
    // 保存中の確定は無視する(入力テキストは保持され、再度 Enter できる)
    if (saving) return;
    const tag = pendingTagOf(name, exclude);
    onValueChange('');
    setHighlighted(null);
    if (tag !== null) onAdd(tag);
  };

  return (
    <div className={VARIANT[variant].wrapper}>
      <input
        type="text"
        autoFocus={autoFocus}
        value={value}
        placeholder={placeholder}
        aria-label={ariaLabel}
        className={cx('font-mono text-[11px]', VARIANT[variant].input)}
        role="combobox"
        aria-expanded={showSuggestions}
        aria-controls={listboxId}
        onFocus={() => setOpen(true)}
        onCompositionStart={() => setComposing(true)}
        onCompositionEnd={() => setComposing(false)}
        onChange={(e) => {
          onValueChange(e.target.value);
          setHighlighted(null);
          setOpen(true);
        }}
        onBlur={() => {
          setOpen(false);
          setHighlighted(null);
          // 変換途中の未確定文字はタグにしない
          if (saving || composing.current) return;
          commit(value);
          onDismiss?.();
        }}
        onKeyDown={(e) => {
          // IME の変換確定 Enter では追加しない
          if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
            if (activeIndex !== null) commit(suggestions[activeIndex].name);
            else if (value.trim() === '') onDismiss?.();
            else commit(value);
          }
          if (e.key === 'Escape') {
            // 重なっているものを内側から順に 1 つずつ閉じる。何かを閉じた Escape は
            // 外側(モーダル)へ伝えない。何も開いていなければ伝わり、外側が閉じる
            if (showSuggestions) {
              e.stopPropagation();
              setOpen(false);
              setHighlighted(null);
            } else if (value !== '') {
              e.stopPropagation();
              onValueChange('');
              setHighlighted(null);
            } else {
              onDismiss?.();
            }
          }
          // IME 変換中の矢印は候補選択ではなく変換操作
          if (showSuggestions && !e.nativeEvent.isComposing) {
            if (e.key === 'ArrowDown') {
              e.preventDefault();
              setHighlighted((h) => (h === null ? 0 : Math.min(h + 1, suggestions.length - 1)));
            }
            if (e.key === 'ArrowUp') {
              e.preventDefault();
              setHighlighted((h) => (h === null || h === 0 ? null : h - 1));
            }
          }
        }}
      />
      {showSuggestions && (
        <div
          id={listboxId}
          role="listbox"
          className={cx(
            'absolute top-full left-0 z-10 mt-1 border border-border bg-surface shadow-lg',
            VARIANT[variant].dropdown,
          )}
        >
          <div className="max-h-44 overflow-y-auto">
            {suggestions.map((t, i) => (
              <button
                key={t.name}
                type="button"
                role="option"
                aria-selected={i === activeIndex}
                disabled={saving}
                className={cx(
                  'flex w-full items-center justify-between gap-2 px-2.5 py-1.5 text-left font-mono text-[11px]',
                  i === activeIndex
                    ? 'bg-accent-soft font-semibold text-accent'
                    : 'text-ink-muted hover:bg-surface-2',
                )}
                // 先に blur が走って入力途中のテキストが確定しないようにする
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => commit(t.name)}
              >
                {t.name}
                <span
                  className={cx('text-[10px]', i === activeIndex ? 'text-accent' : 'text-ink-faint')}
                >
                  {t.count}
                </span>
              </button>
            ))}
          </div>
          <p className="border-t border-border px-2.5 py-1.5 font-mono text-[9px] text-ink-faint">
            候補にない名前は Enter で新規作成
          </p>
        </div>
      )}
    </div>
  );
}

// 付与済みタグのチップ。× で外す(何が起きるかは親の onRemove 次第で、
// 詳細画面は即保存、新規作成フォームは送信前のローカル削除)。
export function TagChip({
  name,
  onRemove,
  disabled = false,
}: {
  name: string;
  onRemove: () => void;
  disabled?: boolean;
}) {
  return (
    <span className="group inline-flex items-center gap-1.5 border border-border px-2.5 py-1 font-mono text-[11px] leading-none text-ink-muted">
      {name}
      <button
        type="button"
        className="text-ink-faint opacity-0 transition group-hover:opacity-100 hover:text-danger focus-visible:opacity-100"
        aria-label={`タグ ${name} を削除`}
        disabled={disabled}
        onClick={onRemove}
      >
        <X size={11} />
      </button>
    </span>
  );
}
