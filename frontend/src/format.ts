// 表示用フォーマッタ(一覧・詳細で共用)

// dashForZero: 一覧のリスト表示では 0(不完全アセット)を「—」にする
export function formatSize(bytes: number, dashForZero = false): string {
  if (bytes <= 0) return dashForZero ? '—' : '0 B';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatDate(iso: string): string {
  // 不完全アセットの updatedAt は Go のゼロ値(0001-01-01…)で来る
  if (iso.startsWith('0001-')) return '—';
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
