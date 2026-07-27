// design/Design.pen 画面03 の新規アセット作成モーダル(幅 560)。
// タイトル / カテゴリ(チップ)/ テンプレート(カード)/ 生成されるフォルダ構成。

import { useEffect, useState } from 'react';
import { Car, Check, File as FileIcon, Plus, Trees, User, X } from 'lucide-react';
import { createAsset, getTemplates, type TagCount } from './api';
import TagSuggestInput, { TagChip } from './TagSuggestInput';
import { Banner, Button, SectionLabel, TextInput, cx, type LucideIcon } from './ui';

// ドット始まりの名前はカテゴリとして拒否・スキャン対象外なので、
// 実在のカテゴリ名と衝突しない番兵値になる
const NEW_CATEGORY = '.new-category';

// 同梱テンプレートは empty.blend のみ(internal/library)。
// 説明文が分かるものだけ添える。
const TEMPLATE_META: Record<string, { icon: LucideIcon; description: string }> = {
  'empty.blend': { icon: FileIcon, description: '空のシーン' },
  'character.blend': { icon: User, description: '' },
  'environment.blend': { icon: Trees, description: '' },
  'vehicle.blend': { icon: Car, description: '' },
};

type Props = {
  categories: string[]; // 既存カテゴリ(現在の一覧から)
  allTags: TagCount[]; // サジェストの母集団(使用中のタグ)
  onClose: () => void;
  onCreated: () => void;
};

export default function NewAssetModal({ categories, allTags, onClose, onCreated }: Props) {
  const [title, setTitle] = useState('');
  const [categoryChoice, setCategoryChoice] = useState(categories[0] ?? NEW_CATEGORY);
  const [newCategory, setNewCategory] = useState('');
  const [templates, setTemplates] = useState<string[] | null>(null);
  const [template, setTemplate] = useState('');
  // タグは作成リクエストまでローカルに溜める(詳細画面の「1 追加 = 1 保存」と違い、
  // 保存先のアセットがまだ無いため)
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getTemplates()
      .then((list) => {
        setTemplates(list);
        setTemplate((cur) => cur || (list[0] ?? ''));
      })
      .catch((err: unknown) => {
        setTemplates([]);
        setError(err instanceof Error ? err.message : String(err));
      });
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const category = categoryChoice === NEW_CATEGORY ? newCategory.trim() : categoryChoice;
  const canSubmit = title.trim() !== '' && category !== '' && template !== '' && !submitting;

  // 打ちかけのタグは確定扱いで送る。取り違えたタグは詳細画面で外せるが、
  // 黙って捨てられたタグは気づけないため
  const pendingTag = tagInput.trim();
  const submittedTags =
    pendingTag !== '' && !tags.includes(pendingTag) ? [...tags, pendingTag] : tags;

  const handleCreate = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await createAsset({ title: title.trim(), category, template, tags: submittedTags });
      onCreated();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSubmitting(false);
    }
  };

  const previewCategory = category || 'カテゴリ';
  const previewTitle = title.trim() || 'タイトル';

  return (
    <div
      className="fixed inset-0 z-10 flex items-center justify-center bg-[var(--c-scrim)] p-6"
      onClick={onClose}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-label="新規アセット作成"
        className="flex max-h-full w-full max-w-[560px] flex-col border border-border bg-surface shadow-[0_16px_48px_#00000080]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex shrink-0 items-start justify-between gap-4 border-b border-border px-6 py-[18px]">
          <div className="flex flex-col gap-1">
            <h2 className="font-heading text-[17px] leading-none font-bold text-ink">
              新規アセット作成
            </h2>
            <p className="text-xs text-ink-faint">source/ 配下にアセットフォルダを自動生成します</p>
          </div>
          <button
            type="button"
            aria-label="閉じる"
            className="text-ink-faint transition hover:text-ink"
            onClick={onClose}
          >
            <X size={16} />
          </button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-6">
          <div className="flex flex-col gap-2">
            <SectionLabel>タイトル</SectionLabel>
            <TextInput
              value={title}
              onChange={setTitle}
              autoFocus
              ariaLabel="タイトル"
              placeholder="Wooden Chair"
            />
          </div>

          <div className="flex flex-col gap-2">
            <SectionLabel>カテゴリ</SectionLabel>
            <div className="flex flex-wrap gap-1.5">
              {categories.map((c) => (
                <CategoryChoice
                  key={c}
                  label={c}
                  active={categoryChoice === c}
                  onClick={() => setCategoryChoice(c)}
                />
              ))}
              <CategoryChoice
                label="新規カテゴリ"
                icon={Plus}
                active={categoryChoice === NEW_CATEGORY}
                onClick={() => setCategoryChoice(NEW_CATEGORY)}
              />
            </div>
            {categoryChoice === NEW_CATEGORY && (
              <TextInput
                value={newCategory}
                onChange={setNewCategory}
                ariaLabel="新規カテゴリ名"
                placeholder="Props"
              />
            )}
          </div>

          <div className="flex flex-col gap-2">
            <SectionLabel>テンプレート</SectionLabel>
            <p className="text-[10px] text-ink-faint">
              templates/ 直下の .blend ファイルが一覧に表示されます
            </p>
            {templates === null ? (
              <p className="font-mono text-[11px] text-ink-faint">読み込み中…</p>
            ) : templates.length === 0 ? (
              <p className="text-[11px] text-ink-faint">
                templates/ に .blend がありません。ファイルを置くとここに現れます
              </p>
            ) : (
              <div className="grid grid-cols-2 gap-2">
                {templates.map((name) => {
                  const meta = TEMPLATE_META[name];
                  const Icon = meta?.icon ?? FileIcon;
                  const active = template === name;
                  return (
                    <button
                      key={name}
                      type="button"
                      onClick={() => setTemplate(name)}
                      className={cx(
                        'flex items-center gap-2.5 border p-3 text-left transition',
                        active
                          ? 'border-accent bg-accent-soft'
                          : 'border-border bg-surface-2 hover:border-ink-faint',
                      )}
                    >
                      <Icon size={18} className={cx('shrink-0', active ? 'text-accent' : 'text-ink-muted')} />
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span
                          className={cx(
                            'truncate font-mono text-[11px]',
                            active ? 'font-semibold text-ink' : 'text-ink-muted',
                          )}
                        >
                          {name}
                        </span>
                        {meta?.description && (
                          <span className="truncate text-[10px] text-ink-faint">
                            {meta.description}
                          </span>
                        )}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          <div className="flex flex-col gap-2">
            <SectionLabel>タグ</SectionLabel>
            <p className="text-[10px] text-ink-faint">
              Enter または候補のクリックで追加します(あとから詳細画面でも編集できます)
            </p>
            {tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {tags.map((tag) => (
                  <TagChip
                    key={tag}
                    name={tag}
                    disabled={submitting}
                    onRemove={() => setTags((cur) => cur.filter((t) => t !== tag))}
                  />
                ))}
              </div>
            )}
            <TagSuggestInput
              variant="field"
              allTags={allTags}
              exclude={tags}
              value={tagInput}
              onValueChange={setTagInput}
              onAdd={(tag) => setTags((cur) => [...cur, tag])}
              frozen={submitting}
              ariaLabel="タグ"
            />
          </div>

          <div className="flex flex-col gap-[3px] border border-border bg-bg px-3.5 py-3 font-mono text-[10px]">
            <span className="truncate text-accent">
              source/{previewCategory}/{previewTitle}/
            </span>
            <span className="truncate text-ink-faint">
              ├── model.blend{template && `  ← ${template} をコピー`}
            </span>
            <span className="text-ink-faint">├── meta.json</span>
            <span className="text-ink-faint">├── textures/</span>
            <span className="text-ink-faint">├── references/</span>
            <span className="text-ink-faint">├── renders/</span>
            <span className="text-ink-faint">└── notes.md</span>
          </div>

          {error && <Banner tone="error">{error}</Banner>}
        </div>

        <div className="flex shrink-0 justify-end gap-2 border-t border-border px-6 py-4">
          <Button onClick={onClose}>キャンセル</Button>
          <Button
            variant="primary"
            icon={Check}
            disabled={!canSubmit}
            onClick={() => void handleCreate()}
          >
            {submitting ? '作成中…' : '作成する'}
          </Button>
        </div>
      </section>
    </div>
  );
}

// カテゴリ選択チップ(padding 9 / 中央揃え / 選択時はアクセント)
function CategoryChoice({
  label,
  icon: Icon,
  active,
  onClick,
}: {
  label: string;
  icon?: LucideIcon;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        'flex flex-auto items-center justify-center gap-1 border px-3 py-[9px] text-xs whitespace-nowrap transition',
        active
          ? 'border-accent bg-accent-soft font-semibold text-accent'
          : 'border-border bg-surface-2 text-ink-muted hover:border-ink-faint hover:text-ink',
      )}
    >
      {Icon && <Icon size={11} />}
      <span className="truncate">{label}</span>
    </button>
  );
}
