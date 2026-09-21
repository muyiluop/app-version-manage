import { client } from "./client";
import type { Application, AppInput, MessageResult, PageQuery, Paginated } from "../types/api";

export interface AppListQuery extends PageQuery {
  keyword?: string;
}

/** 应用分页列表，keyword 匹配名称/标识。 */
export function listApps(params: AppListQuery): Promise<Paginated<Application>> {
  return client.get<Paginated<Application>>("/v2/apps", { params });
}

export function getApp(id: number): Promise<Application> {
  return client.get<Application>(`/v2/apps/${id}`);
}

export function createApp(input: AppInput): Promise<Application> {
  return client.post<Application>("/v2/apps", input);
}

export function updateApp(id: number, input: AppInput): Promise<Application> {
  return client.put<Application>(`/v2/apps/${id}`, input);
}

/** 删除应用（仅 admin），级联软删版本并清理通道/分享/模板。 */
export function deleteApp(id: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/apps/${id}`);
}
