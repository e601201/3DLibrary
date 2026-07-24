import { useEffect, useState } from 'react';
import { createAsset, getTemplates } from './api';

// ドット始まりの名前はカテゴリとして拒否・スキャン対象外なので、
// 実在のカテゴリ名と衝突しない番兵値になる
const NEW_CATEGORY = '.new-category';

const inputClass =
  'w-full rounded border border-neutral-300 bg-white px-3 py-2 text-sm ' +
  'dark:border-neutral-600 dark:bg-neutral-800';

type Props = {
  categories: string[]; // 既存カテゴリ(現在の一覧から)
  onClose: () => void;
  onCreated: () => void;
};

export default function NewAssetModal({ categories, onClose, onCreated }: Props) {
  const [title, setTitle] = useState('');
  const [categoryChoice, setCategoryChoice] = useState(categories[0] ?? NEW_CATEGORY);
  const [newCategory, setNewCategory] = useState('');
  const [templates, setTemplates] = useState<string[] | null>(null);
  const [template, setTemplate] = useState('');
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

  const category = categoryChoice === NEW_CATEGORY ? newCategory.trim() : categoryChoice;
  const canSubmit = title.trim() !== '' && category !== '' && template !== '' && !submitting;

  const handleCreate = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await createAsset({ title: title.trim(), category, template });
      onCreated();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setSubmitting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-10 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <section
        className="w-full max-w-md rounded-lg border border-neutral-300 bg-white p-6 dark:border-neutral-700 dark:bg-neutral-800"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold">新規アセット作成</h2>
        <div className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm">
            タイトル
            <input
              type="text"
              className={inputClass}
              value={title}
              placeholder="Wooden Chair"
              onChange={(e) => setTitle(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            カテゴリ
            <select
              className={inputClass}
              value={categoryChoice}
              onChange={(e) => setCategoryChoice(e.target.value)}
            >
              {categories.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
              <option value={NEW_CATEGORY}>新規カテゴリ…</option>
            </select>
          </label>
          {categoryChoice === NEW_CATEGORY && (
            <label className="flex flex-col gap-1 text-sm">
              新規カテゴリ名
              <input
                type="text"
                className={inputClass}
                value={newCategory}
                placeholder="Props"
                onChange={(e) => setNewCategory(e.target.value)}
              />
            </label>
          )}
          <label className="flex flex-col gap-1 text-sm">
            テンプレート
            {templates === null ? (
              <span className="text-xs text-neutral-500">読み込み中…</span>
            ) : templates.length === 0 ? (
              <span className="text-xs text-neutral-500">
                templates/ に .blend がありません。ファイルを置くとここに現れます
              </span>
            ) : (
              <select
                className={inputClass}
                value={template}
                onChange={(e) => setTemplate(e.target.value)}
              >
                {templates.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
            )}
          </label>
          {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="rounded border border-neutral-300 px-4 py-2 text-sm hover:bg-neutral-100 dark:border-neutral-600 dark:hover:bg-neutral-700"
              onClick={onClose}
            >
              キャンセル
            </button>
            <button
              type="button"
              className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50"
              disabled={!canSubmit}
              onClick={() => void handleCreate()}
            >
              {submitting ? '作成中…' : '作成'}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
