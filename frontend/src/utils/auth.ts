/**
 * 会话存储与权限判断。
 *
 * 为什么集中在这里：token 的读写点很多（拦截器、登录页、路由守卫），
 * 集中一处可以避免 key 拼写不一致导致的"登录了却取不到 token"。
 */
import type { LoginResult, UserProfile, UserRole } from "../types/api";

const ACCESS_TOKEN_KEY = "avm_access_token";
const REFRESH_TOKEN_KEY = "avm_refresh_token";
const USER_KEY = "avm_user";

/** 类型守卫：localStorage 里的内容不可信，需要收窄后再用。 */
function isUserProfile(value: unknown): value is UserProfile {
  if (typeof value !== "object" || value === null) return false;
  const record = value as Record<string, unknown>;
  return typeof record.id === "number" && typeof record.username === "string" && typeof record.role === "string";
}

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function getStoredUser(): UserProfile | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return isUserProfile(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function saveUser(user: UserProfile): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function saveSession(result: LoginResult): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, result.accessToken);
  localStorage.setItem(REFRESH_TOKEN_KEY, result.refreshToken);
  saveUser(result.user);
}

export function clearSession(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

/** 是否具备写权限（admin / releaser）。 */
export function canWrite(role?: UserRole | null): boolean {
  return role === "admin" || role === "releaser";
}

/** 是否为管理员。 */
export function isAdmin(role?: UserRole | null): boolean {
  return role === "admin";
}

/** 角色中文名。 */
export function roleLabel(role: UserRole): string {
  switch (role) {
    case "admin":
      return "管理员";
    case "releaser":
      return "发布员";
    case "viewer":
      return "只读用户";
    default:
      return role;
  }
}
