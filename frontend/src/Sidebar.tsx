// design/Design.pen 画面01・04 の左サイドバー(幅 256 / padding 20-16 / gap 20)。
// ロゴ・全件ナビ・カテゴリ・タグ・キャッシュ情報・設定の順に積む。

import { useState } from 'react';
import {
  Armchair,
  Box,
  Car,
  Folder,
  LayoutGrid,
  Palette,
  RefreshCw,
  Search,
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
  remoteViewing: boolean; // 読み取り専用の経路。操作を持つ区画を出さない
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
  remoteViewing,
  onCategory,
  onTag,
  onRescan,
  onBulkGenerate,
  onOpenLibrary,
  onOpenSettings,
}: Props) {
  // タグが増えるとチップ雲から目で探せなくなるので、打って絞る欄を持つ。
  // 絞るのはこのチップ一覧だけで、アセットは絞らない(アセットを探すのは上部の検索欄)。
  const [tagQuery, setTagQuery] = useState('');
  // 絞り込みだけ大文字小文字を無視する(タグの同一性は区別したまま。CONTEXT.md「タグ」)
  const query = tagQuery.trim().toLowerCase();
  const visibleTags = tags.filter((t) => t.name.toLowerCase().includes(query));

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
            <div className="flex items-center gap-1.5 border border-border bg-surface-2 px-2.5 py-[7px] focus-within:border-accent">
              <Search size={12} className="shrink-0 text-ink-faint" />
              <input
                type="search"
                className="w-full min-w-0 bg-transparent text-[12px] text-ink placeholder:text-ink-faint focus:outline-none"
                placeholder="タグを探す..."
                aria-label="タグを探す"
                value={tagQuery}
                onChange={(e) => setTagQuery(e.target.value)}
                onKeyDown={(e) => {
                  // Enter は先頭のタグを選ぶ一方向の操作。同じキーで解除もすると
                  // 連打で状態が振れるため、選択中のタグに当たったときは何もしない
                  // (解除はチップか見出しの × で)。IME の変換確定 Enter は拾わない
                  if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
                    // 「先頭」は GET /api/tags の名前順そのもの(並べ替えない)
                    const first = visibleTags[0];
                    if (!first || first.name === tag) return;
                    onTag(first.name);
                    onOpenLibrary();
                  }
                  // 消すものがあった Escape は外へ伝えない(TagSuggestInput と同じ扱い)
                  if (e.key === 'Escape' && tagQuery !== '') {
                    e.stopPropagation();
                    setTagQuery('');
                  }
                }}
              />
            </div>
            {visibleTags.length === 0 ? (
              // チップ雲が丸ごと消えたとき、絞り込みの結果なのかタグが無いのかを区別する
              <p className="text-[11px] text-ink-faint">該当するタグがありません</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {visibleTags.map((t) => (
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
            )}
          </div>
        )}
      </div>

      {/* キャッシュ区画は容量表示ごと畳む。残る容量表示は削除・再生成の
          判断材料でしかなく、その操作がどちらも無い経路では読む意味がない */}
      {!remoteViewing && (
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
      )}

      {!remoteViewing && (
        <NavItem
          icon={SettingsIcon}
          label="設定"
          active={page === 'settings'}
          onClick={onOpenSettings}
        />
      )}
    </aside>
  );
}
