import { client } from "./client";
import type { CreateShareInput, MessageResult, Share, UpdateShareInput } from "../types/api";

export function listShares(appId: number): Promise<Share[]> {
  return client.get<Share[]>(`/v2/apps/${appId}/shares`);
}

export function createShare(appId: number, input: CreateShareInput): Promise<Share> {
  return client.post<Share>(`/v2/apps/${appId}/shares`, input);
}

/** password 传空字符串清除密码，expiresInDays 传 0 表示永久。 */
export function updateShare(appId: number, shareId: number, input: UpdateShareInput): Promise<Share> {
  return client.put<Share>(`/v2/apps/${appId}/shares/${shareId}`, input);
}

export function deactivateShare(appId: number, shareId: number): Promise<MessageResult> {
  return client.put<MessageResult>(`/v2/apps/${appId}/shares/${shareId}/deactivate`);
}

export function deleteShare(appId: number, shareId: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/apps/${appId}/shares/${shareId}`);
}
