import { client } from "./client";
import type { ChangePasswordResult, LoginResult, UserProfile } from "../types/api";

/** 登录：401 属预期分支，由页面提示，不触发全局跳转。 */
export function login(username: string, password: string): Promise<LoginResult> {
  return client.post<LoginResult>("/v2/auth/login", { username, password }, { skipAuthRedirect: true, silent: true });
}

/** 用 refreshToken 换取新的令牌对。 */
export function refresh(refreshToken: string): Promise<LoginResult> {
  return client.post<LoginResult>("/v2/auth/refresh", { refreshToken }, { skipAuthRedirect: true, silent: true });
}

/** 当前用户信息；启动时用来恢复会话，失败由 AuthProvider 清理本地状态。 */
export function profile(): Promise<UserProfile> {
  return client.get<UserProfile>("/v2/auth/profile", { skipAuthRedirect: true });
}

/** 修改密码，成功后旧令牌全部失效；返回更新后的用户信息。 */
export function changePassword(oldPassword: string, newPassword: string): Promise<ChangePasswordResult> {
  return client.post<ChangePasswordResult>("/v2/auth/change-password", { oldPassword, newPassword });
}
