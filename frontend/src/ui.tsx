// design/Design.pen の Components フレーム(Button Primary / Button Secondary /
// Nav Item / Tag Chip)と、各画面で繰り返し現れる素片をまとめたプリミティブ。
// 寸法・配色はすべてデザインの実測値をそのまま使っている。

import type { ComponentType, ReactNode } from 'react';

export function cx(...parts: (string | false | null | undefined)[]): string {
  return parts.filter(Boolean).join(' ');
}

export type LucideIcon = ComponentType<{ size?: number | string; className?: string; strokeWidth?: number }>;

type ButtonVariant = 'primary' | 'secondary' | 'danger';

type ButtonProps = {
  children: ReactNode;
  icon?: LucideIcon;
  variant?: ButtonVariant;
  disabled?: boolean;
  title?: string;
  full?: boolean; // サイドバーのように幅いっぱいに伸ばす(中身は左揃えのまま)
  onClick?: () => void;
};

// Button Primary: fill accent / padding [10,16] / gap 8 / label 13-600
// Button Secondary: fill surface-2 + border / 同寸法 / label 13-500
export function Button({
  children,
  icon: Icon,
  variant = 'secondary',
  disabled = false,
  title,
  full = false,
  onClick,
}: ButtonProps) {
  // 寸法はバリアントごとに持つ。base 側に置くと Tailwind の
  // 出力順しだいでバリアントの上書きが効かなくなる。
  const base =
    'inline-flex shrink-0 items-center gap-2 leading-none transition ' +
    'disabled:cursor-not-allowed disabled:opacity-50';
  const styles: Record<ButtonVariant, string> = {
    primary: 'bg-accent px-4 py-2.5 text-[13px] font-semibold text-white hover:brightness-110',
    secondary:
      'border border-border bg-surface-2 px-4 py-2.5 text-[13px] font-medium text-ink hover:border-ink-faint',
    danger:
      'border border-danger px-3.5 py-[9px] text-xs font-medium text-danger hover:bg-danger/10',
  };
  const iconColor: Record<ButtonVariant, string> = {
    primary: 'text-white',
    secondary: 'text-ink-muted',
    danger: 'text-danger',
  };
  return (
    <button
      type="button"
      title={title}
      disabled={disabled}
      onClick={onClick}
      className={cx(base, styles[variant], full && 'w-full')}
    >
      {Icon && <Icon size={variant === 'danger' ? 14 : 15} className={iconColor[variant]} />}
      <span className="truncate">{children}</span>
    </button>
  );
}

// アイコンのみの枠付きボタン(詳細ヘッダーの「戻る」など。padding 8)
export function IconButton({
  icon: Icon,
  label,
  disabled = false,
  active = false,
  onClick,
}: {
  icon: LucideIcon;
  label: string; // aria-label 兼 tooltip
  disabled?: boolean;
  active?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
      className={cx(
        'inline-flex shrink-0 items-center justify-center border p-2 transition',
        'disabled:cursor-not-allowed disabled:opacity-40',
        active
          ? 'border-accent bg-accent-soft text-accent'
          : 'border-border text-ink-muted hover:border-ink-faint hover:text-ink',
      )}
    >
      <Icon size={15} />
    </button>
  );
}

// Tag Chip: padding [4,10] / border / mono 11
export function Chip({
  children,
  active = false,
  onClick,
  title,
}: {
  children: ReactNode;
  active?: boolean;
  onClick?: () => void;
  title?: string;
}) {
  const className = cx(
    'inline-flex items-center gap-1.5 border px-2.5 py-1 font-mono text-[11px] leading-none transition',
    active
      ? 'border-accent bg-accent-soft text-accent'
      : 'border-border text-ink-muted hover:border-ink-faint hover:text-ink',
  );
  if (!onClick) {
    return (
      <span className={className} title={title}>
        {children}
      </span>
    );
  }
  return (
    <button type="button" className={className} title={title} onClick={onClick}>
      {children}
    </button>
  );
}

// セクション見出し: mono 10 / letter-spacing 1
export function SectionLabel({
  children,
  tone = 'faint',
}: {
  children: ReactNode;
  tone?: 'faint' | 'accent';
}) {
  return (
    <p
      className={cx(
        'font-mono text-[10px] leading-none tracking-[1px]',
        tone === 'accent' ? 'text-accent' : 'text-ink-faint',
      )}
    >
      {children}
    </p>
  );
}

// セグメントコントロール(サムネイルサイズ・テーマ・表示切替)
export function Segmented<T extends string | number>({
  value,
  options,
  onChange,
  label,
  mono = false, // サムネイルサイズはデザイン上 mono 11px
}: {
  value: T;
  options: { value: T; label: string; icon?: LucideIcon }[];
  onChange: (value: T) => void;
  label: string;
  mono?: boolean;
}) {
  return (
    <div className="flex shrink-0 border border-border" role="group" aria-label={label}>
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={String(o.value)}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(o.value)}
            className={cx(
              'inline-flex items-center gap-1.5 px-3.5 py-[7px] leading-none transition',
              mono ? 'font-mono text-[11px]' : 'text-xs',
              active
                ? 'bg-accent-soft font-semibold text-accent'
                : 'bg-surface-2 text-ink-muted hover:text-ink',
            )}
          >
            {o.icon && <o.icon size={13} className={active ? 'text-accent' : 'text-ink-faint'} />}
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

// 入力欄: fill surface-2 / border / padding [9,12]
export function TextInput({
  value,
  onChange,
  placeholder,
  mono = false,
  disabled = false,
  autoFocus = false,
  ariaLabel,
  onKeyDown,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  mono?: boolean;
  disabled?: boolean;
  autoFocus?: boolean;
  ariaLabel?: string;
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}) {
  return (
    <input
      type="text"
      value={value}
      disabled={disabled}
      autoFocus={autoFocus}
      aria-label={ariaLabel}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={onKeyDown}
      className={cx(
        'w-full min-w-0 border border-border bg-surface-2 px-3 py-[9px] text-ink',
        'placeholder:text-ink-faint focus:border-accent focus:outline-none disabled:opacity-50',
        mono ? 'font-mono text-[11px]' : 'text-[13px]',
      )}
    />
  );
}

// ビューポート上に重ねるチップ(バッジ・操作ヒント)。
// 下地が常に暗色なので配色は stage-* で固定する
export function OverlayChip({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-1.5 border border-stage-border bg-stage/80 px-2.5 py-[5px] font-mono text-[10px] leading-none text-stage-ink-muted backdrop-blur-sm">
      {children}
    </div>
  );
}

// 中央寄せの空状態・読み込み・エラー表示
export function Centered({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-2.5 px-6 py-16 text-center">
      {children}
    </div>
  );
}

export function Banner({
  tone,
  children,
}: {
  tone: 'error' | 'info' | 'accent';
  children: ReactNode;
}) {
  const styles = {
    error: 'border-danger/60 text-danger',
    info: 'border-border text-ink-muted',
    accent: 'border-accent/60 text-accent',
  } as const;
  return (
    <p className={cx('border bg-surface px-4 py-2.5 text-[13px]', styles[tone])}>{children}</p>
  );
}
