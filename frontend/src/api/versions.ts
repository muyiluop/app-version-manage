import { client } from "./client";
import type {
  MessageResult,
  PageQuery,
  Paginated,
  Platform,
  PublishVersionInput,
  UpdateVersionInput,
  Version,
  VersionStatus,
} from "../types/api";

export interface VersionListQuery extends PageQuery {
  appId: number;
  platform?: Platform;
  channel?: string;
  status?: VersionStatus;
  keyword?: string;
}

/** 版本分页列表，appId 必填。 */
export function listVersions(params: VersionListQuery): Promise<Paginated<Version>> {
  return client.get<Paginated<Version>>("/v2/versions", { params });
}

export function getVersion(id: number): Promise<Version> {
  return client.get<Version>(`/v2/versions/${id}`);
}

/** 发布版本，fileKey 必填且必须已存在于文件库。 */
export function publishVersion(input: PublishVersionInput): Promise<Version> {
  return client.post<Version>("/v2/versions", input);
}

/** 更新版本，传任意子集字段。 */
export function updateVersion(id: number, input: UpdateVersionInput): Promise<Version> {
  return client.put<Version>(`/v2/versions/${id}`, input);
}

/** 单独设置状态：上架(published)/下架(archived)/草稿(draft)。 */
export function setVersionStatus(id: number, status: VersionStatus): Promise<Version> {
  return client.put<Version>(`/v2/versions/${id}/status`, { status });
}

export function deleteVersion(id: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/versions/${id}`);
}

/** 需要登录的产物下载，走 blob 以携带 Authorization 头。 */
export function downloadVersion(id: number): Promise<Blob> {
  return client.get<Blob>(`/v2/versions/${id}/download`, { responseType: "blob", timeout: 0 });
}
