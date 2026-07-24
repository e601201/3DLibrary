// バックエンド API クライアント。規約は docs/api-conventions.md を参照。

export type Theme = 'dark' | 'light' | 'system';
export type ThumbnailSize = 256 | 512 | 1024;

// 許容値の一覧(バックエンド internal/config の validate と対応)
export const THUMBNAIL_SIZES: readonly ThumbnailSize[] = [256, 512, 1024];
export const THEME_OPTIONS: readonly { value: Theme; label: string }[] = [
  { value: 'dark', label: 'ダーク' },
  { value: 'light', label: 'ライト' },
  { value: 'system', label: 'システム' },
];

export interface Config {
  blenderPath: string;
  libraryDir: string;
  thumbnailSize: ThumbnailSize;
  theme: Theme;
}

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let code = 'unknown';
    let message = `HTTP ${res.status}`;
    try {
      const body: { error: { code: string; message: string } } = await res.json();
      code = body.error.code;
      message = body.error.message;
    } catch {
      // エラー形式でないボディはそのまま HTTP ステータスで報告する
    }
    throw new ApiError(code, message);
  }
  return res.json() as Promise<T>;
}

export interface Asset {
  id: number;
  title: string;
  category: string;
  path: string;
  thumbnailPath: string | null;
  glbPath: string | null;
  polygonCount: number | null;
  size: number;
  isIncomplete: boolean;
  isStale: boolean;
  updatedAt: string;
  createdAt: string;
}

export type AssetSort = 'title' | 'updated_desc' | 'updated_asc';

export interface AssetFilter {
  q?: string;
  category?: string;
  sort?: AssetSort;
}

export function getAssets(filter: AssetFilter = {}): Promise<Asset[]> {
  const params = new URLSearchParams();
  if (filter.q) params.set('q', filter.q);
  if (filter.category) params.set('category', filter.category);
  if (filter.sort) params.set('sort', filter.sort);
  const qs = params.toString();
  return request(qs ? `/api/assets?${qs}` : '/api/assets');
}

export interface JobRef {
  category: string;
  title: string;
}

export interface JobStatus {
  running: JobRef | null;
  pendingCount: number;
  batchDone: number;
  batchTotal: number;
  lastError: (JobRef & { message: string }) | null;
}

export function getJobs(): Promise<JobStatus> {
  return request('/api/jobs');
}

export function postJob(ref: JobRef): Promise<JobStatus> {
  return request('/api/jobs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(ref),
  });
}

// サムネイルの配信 URL。?v= は再スキャンごとに変わる id で、
// 再生成後にブラウザキャッシュへ残った古い画像を避けるため
export function thumbnailUrl(asset: Asset): string | null {
  if (!asset.thumbnailPath) return null;
  return `/api/thumbnails/${encodeURIComponent(asset.category)}/${encodeURIComponent(asset.title)}.png?v=${asset.id}`;
}

export interface CategoryCount {
  name: string;
  count: number;
}

export function getCategories(): Promise<CategoryCount[]> {
  return request('/api/categories');
}

export interface CreateAssetInput {
  title: string;
  category: string;
  template: string;
}

export function createAsset(input: CreateAssetInput): Promise<CreateAssetInput> {
  return request('/api/assets', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
}

export function getTemplates(): Promise<string[]> {
  return request('/api/templates');
}

export function postScan(): Promise<{ assetCount: number }> {
  return request('/api/scan', { method: 'POST' });
}

export function getConfig(): Promise<Config> {
  return request('/api/config');
}

export function putConfig(config: Config): Promise<Config> {
  return request('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
}
