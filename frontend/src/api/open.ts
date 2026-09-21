/**
 * 公开 / v1 兼容接口。
 *
 * 这些接口不使用 v2 信封，且多数无需登录，因此要显式关闭全局 401 跳转与
 * 自动 toast，把错误交给调用方（分享门户）自行呈现。
 */
import { client } from "./client";
import { API_BASE_URL } from "../utils/format";
import type {
  CheckResult,
  OpenChangelogItem,
  OpenLatest,
  Paginated,
  Platform,
  ShareAccessResult,
  SharePasswordRequired,
  LegacyVersion,
} from "../types/api";

const PUBLIC_CONFIG = { skipAuthRedirect: true, silent: true } as const;

export interface ShareAccessQuery {
  password?: string;
  channel?: string;
}

/** POST /share/:token，需要密码时返回 HTTP 209 + requirePassword。 */
export function accessShare(
  token: string,
  query: ShareAccessQuery
): Promise<ShareAccessResult | SharePasswordRequired> {
  return client.post<ShareAccessResult | SharePasswordRequired>(
    `/share/${token}`,
    { password: query.password ?? "" },
    { ...PUBLIC_CONFIG, params: query.channel ? { channel: query.channel } : undefined }
  );
}

/** 类型守卫：区分"需要密码"与正常的分享数据。 */
export function isPasswordRequired(
  result: ShareAccessResult | SharePasswordRequired
): result is SharePasswordRequired {
  return (result as SharePasswordRequired).requirePassword === true;
}

/** POST /share/:token/versions，需要密码的分享同样要带 password。 */
export function listShareVersions(
  token: string,
  params: { password?: string; platform?: Platform; page?: number; pageSize?: number }
): Promise<Paginated<LegacyVersion>> {
  const { password, ...rest } = params;
  return client.post<Paginated<LegacyVersion>>(`/share/${token}/versions`, { password: password ?? "" }, {
    ...PUBLIC_CONFIG,
    params: rest,
  });
}

/** GET /open/latest，v1 扁平结构。 */
export function openLatest(params: {
  identifier: string;
  platform?: Platform;
  channel?: string;
  format?: string;
}): Promise<OpenLatest> {
  return client.get<OpenLatest>("/open/latest", { ...PUBLIC_CONFIG, params });
}

/** GET /open/changelog。 */
export function openChangelog(params: {
  identifier: string;
  platform?: Platform;
  channel?: string;
}): Promise<OpenChangelogItem[]> {
  return client.get<OpenChangelogItem[]>("/open/changelog", { ...PUBLIC_CONFIG, params });
}

/** GET /v2/check，新客户端推荐使用的检测更新接口。 */
export function checkUpdate(params: {
  identifier: string;
  platform?: Platform;
  channel?: string;
  currentVersion?: string;
}): Promise<CheckResult> {
  return client.get<CheckResult>("/v2/check", { ...PUBLIC_CONFIG, params });
}

/** 拼接签名令牌下载地址（无需鉴权，可直接用于 a 标签或 blob 请求）。 */
export function openDownloadUrl(token: string): string {
  return `${API_BASE_URL}/open/download/${token}`;
}

/** 通过签名令牌下载，返回 Blob 以保留原始文件名。 */
export function downloadByToken(token: string): Promise<Blob> {
  return client.get<Blob>(`/open/download/${token}`, {
    ...PUBLIC_CONFIG,
    responseType: "blob",
    timeout: 0,
  });
}
