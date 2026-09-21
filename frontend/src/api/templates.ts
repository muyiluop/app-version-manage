import { client } from "./client";
import type { MessageResult, Platform, Template, TemplateInput, TemplatePreview } from "../types/api";

export function listTemplates(appId: number): Promise<Template[]> {
  return client.get<Template[]>(`/v2/apps/${appId}/templates`);
}

export function createTemplate(appId: number, input: TemplateInput): Promise<Template> {
  return client.post<Template>(`/v2/apps/${appId}/templates`, input);
}

export function updateTemplate(appId: number, templateId: number, input: TemplateInput): Promise<Template> {
  return client.put<Template>(`/v2/apps/${appId}/templates/${templateId}`, input);
}

export function deleteTemplate(appId: number, templateId: number): Promise<MessageResult> {
  return client.delete<MessageResult>(`/v2/apps/${appId}/templates/${templateId}`);
}

/** 预览模板渲染结果，platform 缺省时服务端取应用第一个平台。 */
export function previewTemplate(
  appId: number,
  templateId: number,
  input: { platform?: Platform; channel?: string }
): Promise<TemplatePreview> {
  return client.post<TemplatePreview>(`/v2/apps/${appId}/templates/${templateId}/preview`, input);
}
