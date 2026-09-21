/**
 * axios 实例：注入鉴权头、解包 v2 信封、统一错误提示。
 *
 * 为什么只用一个实例：v1 兼容接口与 v2 接口同源（都在 /api 下），
 * 用一个实例可以让鉴权头与错误处理保持一致；区别仅在于成功响应是否带信封，
 * 因此解包逻辑按"是否含 code=OK"判断，而不是按路径判断。
 */
import axios, { AxiosError, type AxiosRequestConfig } from "axios";
import type { ApiEnvelope } from "../types/api";
import { clearSession, getAccessToken } from "../utils/auth";
import { showMessage } from "../utils/message";

declare module "axios" {
  interface AxiosRequestConfig {
    /** 401 时只提示、不跳转登录页（登录接口与分享前台使用）。 */
    skipAuthRedirect?: boolean;
    /** 由调用方自行处理错误提示，拦截器不弹 toast。 */
    silent?: boolean;
  }
}

const baseURL: string = import.meta.env.VITE_API_BASE_URL || "/api";

const http = axios.create({ baseURL, timeout: 120_000 });

http.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.set("Authorization", `Bearer ${token}`);
  }
  return config;
});

/** 收窄为 v2 信封。 */
function isEnvelope(body: unknown): body is ApiEnvelope<unknown> {
  return typeof body === "object" && body !== null && typeof (body as { code?: unknown }).code === "string";
}

http.interceptors.response.use(
  (response) => {
    const body: unknown = response.data;
    // 只有 v2 成功信封才取 data；v1 兼容接口与二进制响应原样返回。
    if (isEnvelope(body) && body.code === "OK") {
      return body.data;
    }
    return body;
  },
  (error: AxiosError<unknown>) => {
    handleError(error);
    return Promise.reject(error);
  }
);

/** 从错误响应体里提取后端 message（兼容 v1 的 { error } 结构）。 */
function extractMessage(body: unknown): string {
  if (typeof body !== "object" || body === null) return "";
  const record = body as Record<string, unknown>;
  if (typeof record.message === "string") return record.message;
  if (typeof record.error === "string") return record.error;
  return "";
}

function handleError(error: AxiosError<unknown>): void {
  const status = error.response?.status;
  const bodyMessage = extractMessage(error.response?.data);
  const silent = error.config?.silent === true;
  const skipRedirect = error.config?.skipAuthRedirect === true;

  if (status === 401) {
    if (!skipRedirect) {
      clearSession();
      // 已经在登录页时不再跳转，避免循环。
      if (!window.location.pathname.startsWith("/login")) {
        const redirect = window.location.pathname + window.location.search;
        window.location.href = `/login?redirect=${encodeURIComponent(redirect)}`;
      }
    }
    if (!silent) {
      showMessage.error(bodyMessage || "登录状态已失效，请重新登录");
    }
    return;
  }

  if (silent) return;

  if (status === 403) {
    showMessage.error(bodyMessage || "没有权限执行此操作");
    return;
  }
  if (status === 404) {
    showMessage.error(bodyMessage || "请求的资源不存在");
    return;
  }
  if (status === 409) {
    showMessage.error(bodyMessage || "数据冲突，请刷新后重试");
    return;
  }
  if (status === 413) {
    showMessage.error(bodyMessage || "上传内容超过大小限制");
    return;
  }
  if (status === 429) {
    showMessage.error(bodyMessage || "操作过于频繁，请稍后再试");
    return;
  }
  if (status !== undefined && status >= 500) {
    showMessage.error(bodyMessage || "服务器内部错误，请稍后重试");
    return;
  }
  if (status !== undefined) {
    showMessage.error(bodyMessage || "操作失败");
    return;
  }
  if (error.code === "ECONNABORTED") {
    showMessage.error("请求超时，请稍后重试");
    return;
  }
  showMessage.error("网络异常，请检查网络连接");
}

/**
 * 拦截器已经把响应解包为 data，因此对外暴露的方法直接返回业务数据。
 */
export interface HttpClient {
  get<T>(url: string, config?: AxiosRequestConfig): Promise<T>;
  delete<T>(url: string, config?: AxiosRequestConfig): Promise<T>;
  post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
  put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
  patch<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
}

export const client = http as unknown as HttpClient;

/** 供 hook 把异常转换为可展示文案。 */
export function getErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const message = extractMessage(error.response?.data);
    if (message) return message;
    if (error.response?.status !== undefined) return `请求失败（HTTP ${error.response.status}）`;
    if (error.code === "ECONNABORTED") return "请求超时，请稍后重试";
    return "网络异常，请检查网络连接";
  }
  if (error instanceof Error) return error.message;
  return "未知错误";
}
