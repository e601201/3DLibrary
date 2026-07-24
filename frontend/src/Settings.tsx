import { useEffect, useRef, useState } from 'react';
import {
  putConfig,
  THEME_OPTIONS,
  THUMBNAIL_SIZES,
  type Config,
  type Theme,
  type ThumbnailSize,
} from './api';
import { applyTheme } from './theme';

type SaveState =
  | { kind: 'idle' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'error'; message: string };

type Props = {
  initial: Config;
  onSaved: (config: Config) => void;
};

const inputClass =
  'w-full rounded border border-neutral-300 bg-white px-3 py-2 text-sm ' +
  'dark:border-neutral-600 dark:bg-neutral-800';

export default function Settings({ initial, onSaved }: Props) {
  const [draft, setDraft] = useState(initial);
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });

  // テーマは選択と同時にプレビュー適用する。保存せずにパネルを閉じたら
  // 最後に保存されたテーマへ戻す。
  const savedTheme = useRef(initial.theme);
  useEffect(() => {
    return () => applyTheme(savedTheme.current);
  }, []);

  const update = (patch: Partial<Config>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setSave({ kind: 'idle' });
  };

  const handleSave = async () => {
    setSave({ kind: 'saving' });
    try {
      const saved = await putConfig(draft);
      savedTheme.current = saved.theme;
      onSaved(saved);
      setSave({ kind: 'saved' });
    } catch (err) {
      setSave({ kind: 'error', message: err instanceof Error ? err.message : String(err) });
    }
  };

  return (
    <section className="w-full max-w-lg rounded-lg border border-neutral-300 bg-white p-6 dark:border-neutral-700 dark:bg-neutral-800">
      <h2 className="mb-4 text-lg font-semibold">設定</h2>
      <div className="flex flex-col gap-4">
        <label className="flex flex-col gap-1 text-sm">
          Blender実行ファイル
          <input
            type="text"
            className={inputClass}
            value={draft.blenderPath}
            placeholder="/Applications/Blender.app/Contents/MacOS/Blender"
            onChange={(e) => update({ blenderPath: e.target.value })}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          ライブラリディレクトリ
          <input
            type="text"
            className={inputClass}
            value={draft.libraryDir}
            placeholder="/Users/you/3DLibrary"
            onChange={(e) => update({ libraryDir: e.target.value })}
          />
          <span className="text-xs text-neutral-500 dark:text-neutral-400">
            作成済みの空ディレクトリを指定すると source / cache / templates とテンプレートが自動作成されます
          </span>
        </label>
        <label className="flex flex-col gap-1 text-sm">
          サムネイルサイズ
          <select
            className={inputClass}
            value={draft.thumbnailSize}
            onChange={(e) => update({ thumbnailSize: Number(e.target.value) as ThumbnailSize })}
          >
            {THUMBNAIL_SIZES.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm">
          テーマ
          <select
            className={inputClass}
            value={draft.theme}
            onChange={(e) => {
              const theme = e.target.value as Theme;
              update({ theme });
              applyTheme(theme); // 保存を待たず即時反映
            }}
          >
            {THEME_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <div className="flex items-center gap-3">
          <button
            type="button"
            className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-50"
            disabled={save.kind === 'saving'}
            onClick={handleSave}
          >
            保存
          </button>
          {save.kind === 'saving' && <span className="text-sm text-neutral-500">保存中…</span>}
          {save.kind === 'saved' && <span className="text-sm text-green-600 dark:text-green-400">保存しました</span>}
          {save.kind === 'error' && (
            <span className="text-sm text-red-600 dark:text-red-400">{save.message}</span>
          )}
        </div>
      </div>
    </section>
  );
}
