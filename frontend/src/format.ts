// 表示用フォーマッタ(一覧・詳細で共用)

// dashForZero: 一覧のリスト表示では 0(不完全アセット)を「—」にする
export function formatSize(bytes: number, dashForZero = false): string {
  if (bytes <= 0) return dashForZero ? '—' : '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

// 不完全アセットの updatedAt は Go のゼロ値(0001-01-01…)で来る
function isZeroTime(iso: string): boolean {
  return iso === '' || iso.startsWith('0001-');
}

export function formatDate(iso: string): string {
  if (isZeroTime(iso)) return '—';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// カードのフッターは日付だけ(デザイン 01: "2026-07-21")
export function formatDay(iso: string): string {
  if (isZeroTime(iso)) return '—';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// カードのポリゴン数は概数表示(デザイン 01: "12.4K POLYS")
export function formatPolygons(count: number | null): string {
  if (count === null) return '—';
  if (count < 1000) return `${count} POLYS`;
  if (count < 1_000_000) return `${(count / 1000).toFixed(1)}K POLYS`;
  return `${(count / 1_000_000).toFixed(1)}M POLYS`;
}

// 詳細のメタデータ表は実数をカンマ区切りで出す(デザイン 02: "12,438")
export function formatNumber(value: number): string {
  return value.toLocaleString('en-US');
}
