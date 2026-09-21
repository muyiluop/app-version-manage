/**
 * 展示层格式化工具：字节、时间、平台名称与图标。
 */
import { createElement, type ReactNode } from "react";
import {
  AndroidOutlined,
  AppleOutlined,
  DesktopOutlined,
  HarmonyOSOutlined,
  LinuxOutlined,
  WindowsOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import type { Platform, VersionStatus } from "../types/api";

/** v2 接口前缀，默认 /api。 */
export const API_BASE_URL: string = import.meta.env.VITE_API_BASE_URL || "/api";

/** 字节格式化，保留两位小数。 */
export function formatBytes(size: number | null | undefined): string {
  if (size === null || size === undefined || Number.isNaN(size)) return "-";
  if (size <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = size;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

/** 统一时间格式 YYYY-MM-DD HH:mm:ss。 */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "-";
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format("YYYY-MM-DD HH:mm:ss") : "-";
}

/** 只保留日期。 */
export function formatDate(value: string | null | undefined): string {
  if (!value) return "-";
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format("YYYY-MM-DD") : "-";
}

/** 平台中文名。 */
export function platformLabel(platform: Platform | string | null | undefined): string {
  switch (platform) {
    case "android":
      return "Android";
    case "ios":
      return "iOS";
    case "windows":
      return "Windows";
    case "macos":
      return "macOS";
    case "linux":
      return "Linux";
    case "harmony":
      return "HarmonyOS";
    default:
      return platform ? String(platform) : "-";
  }
}

/** 平台图标（用 createElement 保持本文件为 .ts）。 */
export function platformIcon(platform: Platform | string | null | undefined): ReactNode {
  switch (platform) {
    case "windows":
      return createElement(WindowsOutlined);
    case "android":
      return createElement(AndroidOutlined);
    case "ios":
      return createElement(AppleOutlined);
    case "macos":
      return createElement(AppleOutlined);
    case "linux":
      return createElement(LinuxOutlined);
    case "harmony":
      return createElement(HarmonyOSOutlined);
    default:
      return createElement(DesktopOutlined);
  }
}

/** 应用图标地址：契约规定为 /api/static/logos/<key>。 */
export function logoUrl(key: string | null | undefined): string {
  if (!key) return "";
  return `${API_BASE_URL}/static/logos/${key}`;
}

/** 版本状态中文名与颜色。 */
export function versionStatusMeta(status: VersionStatus): { label: string; color: string } {
  switch (status) {
    case "published":
      return { label: "已发布", color: "green" };
    case "archived":
      return { label: "已下架", color: "orange" };
    case "draft":
    default:
      return { label: "草稿", color: "default" };
  }
}

/** 版本状态下拉选项。 */
export const VERSION_STATUS_OPTIONS: Array<{ label: string; value: VersionStatus }> = [
  { label: "草稿", value: "draft" },
  { label: "已发布", value: "published" },
  { label: "已下架", value: "archived" },
];

/** 复制文本到剪贴板，返回是否成功（非安全上下文下 clipboard 可能不可用）。 */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(textarea);
    return ok;
  } catch {
    return false;
  }
}
