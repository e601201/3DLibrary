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
  if (res.status === 204) return undefined as T;
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
  tags: string[];
}

export type AssetSort = 'title' | 'updated_desc' | 'updated_asc';

export interface AssetFilter {
  q?: string;
  category?: string;
  tag?: string;
  sort?: AssetSort;
}

export function getAssets(filter: AssetFilter = {}): Promise<Asset[]> {
  const params = new URLSearchParams();
  if (filter.q) params.set('q', filter.q);
  if (filter.category) params.set('category', filter.category);
  if (filter.tag) params.set('tag', filter.tag);
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

// 不足分(未生成+陳腐化)を一括でキューに積む
export function postBulkJobs(): Promise<{ enqueued: number; status: JobStatus }> {
  return request('/api/jobs/bulk', { method: 'POST' });
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

// GLB キャッシュの配信 URL(未生成なら null)
export function glbUrl(asset: Asset): string | null {
  if (!asset.glbPath) return null;
  return `/api/glb/${encodeURIComponent(asset.category)}/${encodeURIComponent(asset.title)}.glb?v=${asset.id}`;
}

// 抽出メタデータ(生成時に Blender が .blend から読み取る統計情報)
export interface ExtractedMetadata {
  objectCount: number;
  collectionCount: number;
  materialCount: number;
  polygonCount: number;
  textureCount: number;
  hasAnimation: boolean;
}

// 未生成(404)は null を返す
export async function getExtractedMetadata(
  category: string,
  title: string,
): Promise<ExtractedMetadata | null> {
  try {
    return await request(
      `/api/extracted-metadata/${encodeURIComponent(category)}/${encodeURIComponent(title)}.json`,
    );
  } catch (err) {
    if (err instanceof ApiError && err.code === 'not_found') return null;
    throw err;
  }
}

export interface FileEntry {
  name: string;
  size: number;
  isDir: boolean;
  fileCount: number;
}

export interface AssetFiles {
  entries: FileEntry[];
  glbSize: number | null;
}

export function getAssetFiles(category: string, title: string): Promise<AssetFiles> {
  return request(
    `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/files`,
  );
}

export interface BrowseEntry {
  name: string;
  size: number;
}

// アセット直下のサブディレクトリのファイル一覧(無ければ空)
export function getAssetDir(
  category: string,
  title: string,
  subdir: string,
): Promise<BrowseEntry[]> {
  return request(
    `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/dir/${encodeURIComponent(subdir)}`,
  );
}

// アセット内ファイルの配信 URL(読み取り専用)
export function assetRawUrl(category: string, title: string, rel: string): string {
  const encoded = rel.split('/').map(encodeURIComponent).join('/');
  return `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/raw/${encoded}`;
}

export function postReveal(category: string, title: string): Promise<void> {
  return request(
    `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/reveal`,
    { method: 'POST' },
  );
}

export function postOpenBlender(category: string, title: string): Promise<void> {
  return request(
    `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/open`,
    { method: 'POST' },
  );
}

export interface CategoryCount {
  name: string;
  count: number;
}

export function getCategories(): Promise<CategoryCount[]> {
  return request('/api/categories');
}

// タグ一覧(件数付き)
export type TagCount = CategoryCount;

export function getTags(): Promise<TagCount[]> {
  return request('/api/tags');
}

export function putTags(
  category: string,
  title: string,
  tags: string[],
): Promise<{ tags: string[] }> {
  return request(
    `/api/assets/${encodeURIComponent(category)}/${encodeURIComponent(title)}/tags`,
    {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tags }),
    },
  );
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

export interface CacheInfo {
  sizeBytes: number;
  fileCount: number;
}

export function getCacheInfo(): Promise<CacheInfo> {
  return request('/api/cache');
}

// キャッシュを丸ごと削除する(再生成は「不足分を一括生成」で)
export function deleteCache(): Promise<void> {
  return request('/api/cache', { method: 'DELETE' });
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
