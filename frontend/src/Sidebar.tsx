// design/Design.pen 画面01・04 の左サイドバー(幅 256 / padding 20-16 / gap 20)。
// ロゴ・全件ナビ・カテゴリ・タグ・キャッシュ情報・設定の順に積む。

import {
  Armchair,
  Box,
  Car,
  Folder,
  LayoutGrid,
  Palette,
  RefreshCw,
  Settings as SettingsIcon,
  Sparkles,
  Sword,
  Trees,
  User,
  Zap,
} from 'lucide-react';
import type { CacheInfo, CategoryCount, TagCount } from './api';
import { formatSize } from './format';
import { Button, Chip, SectionLabel, cx, type LucideIcon } from './ui';

// デザインはカテゴリごとに固有アイコンを持つ。カテゴリはユーザーが自由に
// 増やせる(CONTEXT.md)ので、既知の名前だけ当てて残りはフォルダにする。
const CATEGORY_ICONS: Record<string, LucideIcon> = {
  character: User,
  characters: User,
  prop: Armchair,
  props: Armchair,
  environment: Trees,
  environments: Trees,
  vehicle: Car,
  vehicles: Car,
  weapon: Sword,
  weapons: Sword,
  effect: Sparkles,
  effects: Sparkles,
  material: Palette,
  materials: Palette,
};

function categoryIcon(name: string): LucideIcon {
  return CATEGORY_ICONS[name.toLowerCase()] ?? Folder;
}

// Nav Item コンポーネント(padding 8-10 / gap 10 / 右端にカウント)
function NavItem({
  icon: Icon,
  label,
  count,
  active,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  count?: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      className={cx(
        'flex w-full items-center justify-between gap-2.5 px-2.5 py-2 text-left transition',
        active ? 'bg-accent-soft' : 'hover:bg-surface-2',
      )}
    >
      <span className="flex min-w-0 items-center gap-2.5">
        <Icon size={15} className={cx('shrink-0', active ? 'text-accent' : 'text-ink-muted')} />
        <span
          className={cx(
            'truncate text-[13px]',
            active ? 'font-semibold text-ink' : 'text-ink-muted',
          )}
        >
          {label}
        </span>
      </span>
      {count !== undefined && (
        <span className={cx('font-mono text-[11px]', active ? 'text-accent' : 'text-ink-faint')}>
          {count}
        </span>
      )}
    </button>
  );
}

type Props = {
  page: 'library' | 'settings';
  categories: CategoryCount[];
  tags: TagCount[];
  category: string; // '' = すべて
  tag: string; // '' = 絞り込みなし
  totalCount: number;
  cache: CacheInfo | null;
  missingCount: number | null; // 生成が必要なアセット数(不明なら null)
  scanning: boolean;
  jobsActive: boolean;
  disabled: boolean; // ライブラリ未設定
  onCategory: (category: string) => void;
  onTag: (tag: string) => void;
  onRescan: () => void;
  onBulkGenerate: () => void;
  onOpenLibrary: () => void;
  onOpenSettings: () => void;
};

export default function Sidebar({
  page,
  categories,
  tags,
  category,
  tag,
  totalCount,
  cache,
  missingCount,
  scanning,
  jobsActive,
  disabled,
  onCategory,
  onTag,
  onRescan,
  onBulkGenerate,
  onOpenLibrary,
  onOpenSettings,
}: Props) {
  return (
    <aside className="flex w-64 shrink-0 flex-col gap-5 border-r border-border bg-surface px-4 py-5">
      <div className="flex items-center gap-2.5 px-1">
        <div className="flex size-7 shrink-0 items-center justify-center bg-accent">
          <Box size={16} className="text-white" />
        </div>
        <div className="flex min-w-0 flex-col gap-0.5">
          <p className="font-heading text-[15px] leading-none font-bold tracking-[0.5px] text-ink">
            3D LIBRARY
          </p>
          <p className="font-mono text-[8px] leading-none tracking-[0.8px] text-ink-faint">
            LOCAL ASSET MANAGER
          </p>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto">
        <nav className="flex flex-col gap-0.5">
          <NavItem
            icon={LayoutGrid}
            label="すべてのアセット"
            count={totalCount}
            active={page === 'library' && category === ''}
            onClick={() => {
              onCategory('');
              onOpenLibrary();
            }}
          />
        </nav>

        {categories.length > 0 && (
          <div className="flex flex-col gap-1.5">
            <SectionLabel>カテゴリ</SectionLabel>
            <nav className="flex flex-col gap-0.5">
              {categories.map((c) => (
                <NavItem
                  key={c.name}
                  icon={categoryIcon(c.name)}
                  label={c.name}
                  count={c.count}
                  active={page === 'library' && category === c.name}
                  onClick={() => {
                    onCategory(c.name);
                    onOpenLibrary();
                  }}
                />
              ))}
            </nav>
          </div>
        )}

        {tags.length > 0 && (
          <div className="flex flex-col gap-2">
            <SectionLabel>タグ</SectionLabel>
            <div className="flex flex-wrap gap-1.5">
              {tags.map((t) => (
                <Chip
                  key={t.name}
                  active={tag === t.name}
                  title={`${t.name}(${t.count} 件)`}
                  onClick={() => {
                    onTag(tag === t.name ? '' : t.name);
                    onOpenLibrary();
                  }}
                >
                  {t.name}
                </Chip>
              ))}
            </div>
          </div>
        )}
      </div>

      <div className="flex flex-col gap-2 border border-border bg-surface-2 p-3">
        <div className="flex items-center justify-between">
          <SectionLabel>キャッシュ</SectionLabel>
          <span className="font-mono text-[11px] text-ink-muted">
            {cache ? formatSize(cache.sizeBytes) : '—'}
          </span>
        </div>
        <Button icon={RefreshCw} full disabled={scanning || disabled} onClick={onRescan}>
          {scanning ? 'スキャン中…' : '再スキャン'}
        </Button>
        <Button
          icon={Zap}
          full
          disabled={disabled || jobsActive || missingCount === 0}
          title="キャッシュ未生成または要更新のアセットだけを順番に生成します"
          onClick={onBulkGenerate}
        >
          {missingCount === null ? '不足分を一括生成' : `不足分を一括生成 (${missingCount})`}
        </Button>
      </div>

      <NavItem
        icon={SettingsIcon}
        label="設定"
        active={page === 'settings'}
        onClick={onOpenSettings}
      />
    </aside>
  );
}
