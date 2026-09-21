import { client } from "./client";
import type { AuditLog, PageQuery, Paginated } from "../types/api";

export interface AuditListQuery extends PageQuery {
  action?: string;
}

export function listAuditLogs(params: AuditListQuery): Promise<Paginated<AuditLog>> {
  return client.get<Paginated<AuditLog>>("/v2/audit-logs", { params });
}

/** 审计日志 action 常见取值，用于筛选下拉。 */
export const AUDIT_ACTIONS: Array<{ label: string; value: string }> = [
  { label: "登录", value: "auth.login" },
  { label: "修改密码", value: "auth.change_password" },
  { label: "创建用户", value: "user.create" },
  { label: "更新用户", value: "user.update" },
  { label: "删除用户", value: "user.delete" },
  { label: "创建应用", value: "app.create" },
  { label: "更新应用", value: "app.update" },
  { label: "删除应用", value: "app.delete" },
  { label: "发布版本", value: "version.publish" },
  { label: "更新版本", value: "version.update" },
  { label: "删除版本", value: "version.delete" },
  { label: "上传文件", value: "file.upload" },
  { label: "删除文件", value: "file.delete" },
  { label: "创建分享", value: "share.create" },
];
