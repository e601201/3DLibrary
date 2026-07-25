// design/Design.pen 画面04 の設定ページ(本文幅 680)。
// 一般 / キャッシュ・生成 / 外観 の 3 セクションと保存フッター。

import { useEffect, useRef, useState } from 'react';
import { Check, Monitor, Moon, Sun, Trash2 } from 'lucide-react';
import {
  deleteCache,
  putConfig,
  THUMBNAIL_SIZES,
  type CacheInfo,
  type Config,
  type Theme,
  type ThumbnailSize,
} from './api';
import { formatSize } from './format';
import { applyTheme } from './theme';
import { Banner, Button, SectionLabel, Segmented, TextInput, type LucideIcon } from './ui';

type SaveState =
  | { kind: 'idle' }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'error'; message: string };

type ClearState =
  | { kind: 'idle' }
  | { kind: 'clearing' }
  | { kind: 'done' }
  | { kind: 'error'; message: string };

type Props = {
  initial: Config;
  cache: CacheInfo | null;
  onSaved: (config: Config) => void;
  onCacheCleared: () => void;
};

const THEME_SEGMENTS: { value: Theme; label: string; icon: LucideIcon }[] = [
  { value: 'dark', label: 'ダーク', icon: Moon },
  { value: 'light', label: 'ライト', icon: Sun },
  { value: 'system', label: 'システム', icon: Monitor },
];

export default function Settings({ initial, cache, onSaved, onCacheCleared }: Props) {
  const [draft, setDraft] = useState(initial);
  const [save, setSave] = useState<SaveState>({ kind: 'idle' });
  const [clear, setClear] = useState<ClearState>({ kind: 'idle' });

  // テーマは選択と同時にプレビュー適用する。保存せずにページを離れたら
  // 最後に保存されたテーマへ戻す。
  const savedTheme = useRef(initial.theme);
  useEffect(() => {
    return () => applyTheme(savedTheme.current);
  }, []);

  const update = (patch: Partial<Config>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setSave({ kind: 'idle' });
  };

  const handleReset = () => {
    setDraft(initial);
    applyTheme(savedTheme.current);
    setSave({ kind: 'idle' });
  };

  const handleClearCache = async () => {
    // 削除は不可逆なので必ず確認する(キャッシュは再生成可能)
    if (
      !window.confirm(
        'キャッシュ(GLB・サムネイル・抽出メタデータ)を全て削除しますか?\nsource とタグは残ります。再生成は「不足分を一括生成」で行えます。',
      )
    ) {
      return;
    }
    setClear({ kind: 'clearing' });
    try {
      await deleteCache();
      setClear({ kind: 'done' });
      onCacheCleared();
    } catch (err) {
      setClear({ kind: 'error', message: err instanceof Error ? err.message : String(err) });
    }
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
    <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-12 py-8">
      <div className="flex flex-col gap-1">
        <h1 className="font-heading text-xl leading-none font-bold text-ink">設定</h1>
        <p className="text-xs text-ink-faint">アプリケーションの動作と外観を設定します</p>
      </div>

      <div className="flex w-full max-w-[680px] flex-col gap-6">
        <Section label="一般">
          <div className="flex flex-col gap-2">
            <p className="text-[13px] font-semibold text-ink">Blender実行ファイル</p>
            <TextInput
              mono
              value={draft.blenderPath}
              ariaLabel="Blender実行ファイル"
              placeholder="C:\Program Files\Blender Foundation\Blender 5.2\blender.exe"
              onChange={(v) => update({ blenderPath: v })}
            />
          </div>
          <div className="flex flex-col gap-2">
            <p className="text-[13px] font-semibold text-ink">ライブラリディレクトリ</p>
            <TextInput
              mono
              value={draft.libraryDir}
              ariaLabel="ライブラリディレクトリ"
              placeholder="C:\Users\you\3DLibrary"
              onChange={(v) => update({ libraryDir: v })}
            />
            <p className="text-[11px] text-ink-faint">
              作成済みの空ディレクトリを指定すると source / cache / templates とテンプレートが自動作成されます
            </p>
          </div>
        </Section>

        <Section label="キャッシュ・生成">
          <SettingRow
            label="サムネイルサイズ"
            description="Blender CLIで生成する画像の解像度"
          >
            <Segmented
              mono
              label="サムネイルサイズ"
              value={draft.thumbnailSize}
              options={THUMBNAIL_SIZES.map((size) => ({ value: size, label: `${size}px` }))}
              onChange={(size) => update({ thumbnailSize: size as ThumbnailSize })}
            />
          </SettingRow>
          <SettingRow
            label="キャッシュの削除"
            description={`削除したキャッシュは「不足分を一括生成」で再生成できます${
              cache ? ` (${formatSize(cache.sizeBytes)})` : ''
            }`}
          >
            <Button
              variant="danger"
              icon={Trash2}
              disabled={clear.kind === 'clearing'}
              onClick={() => void handleClearCache()}
            >
              {clear.kind === 'clearing' ? '削除中…' : 'キャッシュを削除'}
            </Button>
          </SettingRow>
          {clear.kind === 'done' && <Banner tone="info">キャッシュを削除しました</Banner>}
          {clear.kind === 'error' && (
            <Banner tone="error">削除に失敗しました: {clear.message}</Banner>
          )}
        </Section>

        <Section label="外観">
          <SettingRow label="テーマ" description="インターフェースの配色">
            <Segmented
              label="テーマ"
              value={draft.theme}
              options={THEME_SEGMENTS}
              onChange={(theme) => {
                update({ theme });
                applyTheme(theme); // 保存を待たず即時反映
              }}
            />
          </SettingRow>
        </Section>

        <div className="flex items-center justify-end gap-2">
          {save.kind === 'saved' && (
            <span className="font-mono text-[11px] text-accent">保存しました</span>
          )}
          {save.kind === 'error' && (
            <span className="text-[11px] text-danger">{save.message}</span>
          )}
          <Button onClick={handleReset}>元に戻す</Button>
          <Button
            variant="primary"
            icon={Check}
            disabled={save.kind === 'saving'}
            onClick={() => void handleSave()}
          >
            {save.kind === 'saving' ? '保存中…' : '変更を保存'}
          </Button>
        </div>
      </div>
    </div>
  );
}

// セクション見出し(アクセント色の mono ラベル)と枠付きの本体
function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3.5">
      <SectionLabel tone="accent">{label}</SectionLabel>
      <div className="flex flex-col gap-[18px] border border-border bg-surface p-5">{children}</div>
    </section>
  );
}

// 左に説明・右に操作を置く行
function SettingRow({
  label,
  description,
  children,
}: {
  label: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-4">
      <div className="flex min-w-0 flex-col gap-[3px]">
        <p className="text-[13px] font-semibold text-ink">{label}</p>
        <p className="text-[11px] text-ink-faint">{description}</p>
      </div>
      {children}
    </div>
  );
}
