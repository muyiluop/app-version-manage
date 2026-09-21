/**
 * 登录态上下文。
 *
 * 为什么需要它：Layout 菜单、路由守卫、按钮权限都依赖当前用户与角色，
 * 集中一份状态避免每个页面各自请求 profile。
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import * as authApi from "../../api/auth";
import type { LoginResult, UserProfile } from "../../types/api";
import { clearSession, getAccessToken, getStoredUser, saveSession, saveUser } from "../../utils/auth";

interface AuthContextValue {
  user: UserProfile | null;
  /** 启动时正在用本地 token 恢复会话。 */
  initializing: boolean;
  login: (username: string, password: string) => Promise<LoginResult>;
  logout: () => void;
  /** 重新拉取当前用户信息。 */
  reload: () => Promise<void>;
  /** 本地更新用户信息（改密成功后同步 mustChangePassword）。 */
  updateUser: (user: UserProfile) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  // 先用本地缓存渲染，避免刷新页面时菜单闪烁
  const [user, setUser] = useState<UserProfile | null>(() => getStoredUser());
  const [initializing, setInitializing] = useState<boolean>(() => getAccessToken() !== null);

  const updateUser = useCallback((next: UserProfile) => {
    saveUser(next);
    setUser(next);
  }, []);

  const logout = useCallback(() => {
    clearSession();
    setUser(null);
  }, []);

  const reload = useCallback(async () => {
    const current = await authApi.profile();
    updateUser(current);
  }, [updateUser]);

  // 启动时校验本地 token 是否仍然有效
  useEffect(() => {
    if (!getAccessToken()) {
      setInitializing(false);
      return;
    }
    let cancelled = false;
    authApi
      .profile()
      .then((current) => {
        if (!cancelled) updateUser(current);
      })
      .catch(() => {
        if (!cancelled) {
          clearSession();
          setUser(null);
        }
      })
      .finally(() => {
        if (!cancelled) setInitializing(false);
      });
    return () => {
      cancelled = true;
    };
  }, [updateUser]);

  const login = useCallback(async (username: string, password: string) => {
    const result = await authApi.login(username, password);
    saveSession(result);
    setUser(result.user);
    return result;
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ user, initializing, login, logout, reload, updateUser }),
    [user, initializing, login, logout, reload, updateUser]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth 必须在 AuthProvider 内部使用");
  }
  return context;
}
