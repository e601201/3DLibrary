// スプライト(全周レンダリングを 1 枚に敷き詰めたシート)の読み解き方。
// シート形状は生成側(internal/generate/generate.py)と同じ定数で、
// 一覧カードのホバースクラブが使う(PRD hover-scrub-preview)。

export const SPRITE_FRAMES = 48;
export const SPRITE_COLS = 8;
export const SPRITE_ROWS = 6;

// background-size: シート 1 枚を「1 フレーム分の枠」に対して何倍に
// 引き伸ばすか。列数・行数がそのまま倍率になる。
export const SPRITE_BACKGROUND_SIZE = `${SPRITE_COLS * 100}% ${SPRITE_ROWS * 100}%`;

// カード画像領域内のカーソル X(左端からの px)をフレーム番号へ写像する。
// 左端 = フレーム 0(サムネイルと同じ角度)、右端 = 最終フレーム。
// 領域外へ出た座標も端のフレームに丸める(ホバー終了直前の値で飛ばない)。
export function frameAtX(x: number, width: number): number {
  if (!(width > 0)) return 0;
  const frame = Math.floor((x / width) * SPRITE_FRAMES);
  return Math.min(Math.max(frame, 0), SPRITE_FRAMES - 1);
}

// フレーム番号を background-position の % へ写像する。
// n 分割のシートでは、位置 % は列・行を (n - 1) で割った割合になる。
export function framePosition(frame: number): string {
  const col = frame % SPRITE_COLS;
  const row = Math.floor(frame / SPRITE_COLS);
  const x = (col / (SPRITE_COLS - 1)) * 100;
  const y = (row / (SPRITE_ROWS - 1)) * 100;
  return `${x}% ${y}%`;
}

// ポジションバーの位置(0〜1)。フレームではなくカーソルそのものを追う。
export function scrubRatio(x: number, width: number): number {
  if (!(width > 0)) return 0;
  return Math.min(Math.max(x / width, 0), 1);
}

// ホバーを持たない環境(タッチ)ではスクラブしない(PRD 対象外 v1)。
export function canHoverScrub(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(hover: hover)').matches;
}
