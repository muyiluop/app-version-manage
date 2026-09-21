import { Navigate, useLocation } from "react-router-dom";
import { Result, Spin } from "antd";
import type { ReactNode } from "react";
import { useAuth } from "./AuthContext";
import { isAdmin } from "../../utils/auth";

/** 未登录跳转登录页并带上回跳地址；强制改密用户先引导到改密页。 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, initializing } = useAuth();
  const location = useLocation();

  if (initializing) {
    return (
      <div style={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh" }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!user) {
    const redirect = location.pathname + location.search;
    return <Navigate to={`/login?redirect=${encodeURIComponent(redirect)}`} replace />;
  }

  if (user.mustChangePassword && location.pathname !== "/change-password") {
    return <Navigate to="/change-password" replace />;
  }

  return <>{children}</>;
}

/** 仅管理员可见的页面（用户管理、审计日志）。 */
export function RequireAdmin({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  if (!isAdmin(user?.role)) {
    return <Result status="403" title="403" subTitle="仅管理员可以访问该页面" />;
  }
  return <>{children}</>;
}
