import { client } from "./client";
import type { Channel, ChannelInput, MessageResult } from "../types/api";

/** 通道列表返回数组而非分页结构。 */
export function listChannels(appId: number): Promise<Channel[]> {
  return client.get<Channel[]>(`/v2/apps/${appId}/channels`);
}

export function createChannel(appId: number, input: ChannelInput): Promise<Channel> {
  return client.post<Channel>(`/v2/apps/${appId}/channels`, input);
}

export function updateChannel(appId: number, channelId: number, input: ChannelInput): Promise<Channel> {
  return client.put<Channel>(`/v2/apps/${appId}/channels/${channelId}`, input);
}

/** 默认通道或存在版本的通道会被服务端拒绝（409）。 */
export function deleteChannel(appId: number, channelId: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/apps/${appId}/channels/${channelId}`);
}

export function setDefaultChannel(appId: number, key: string): Promise<MessageResult> {
  return client.put<MessageResult>(`/v2/apps/${appId}/default-channel`, { key });
}
