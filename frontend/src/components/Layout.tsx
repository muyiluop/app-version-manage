/**
 * 后台主框架：侧边菜单 + 面包屑 + 内容区。
 *
 * 菜单选中项由 pathname 派生（不再直接用 pathname 作为 key），
 * 否则 /apps/1 这类子路由不会高亮任何一项。
 */
import { useMemo } from "react";
import { Avatar, Breadcrumb, Dropdown, Layout as AntLayout, Menu, Space, Tag, type MenuProps } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  AppstoreOutlined,
  AuditOutlined,
  FolderOutlined,
  KeyOutlined,
  LogoutOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import ErrorBoundary from "./ErrorBoundary";
import { useAuth } from "../features/auth/AuthContext";
import { isAdmin, roleLabel } from "../utils/auth";

const { Header, Content, Sider } = AntLayout;

interface MenuEntry {
  key: string;
  label: string;
  icon: React.ReactNode;
  /** 仅管理员可见。 */
  adminOnly?: boolean;
}

const MENU_ENTRIES: MenuEntry[] = [
  { key: "/apps", label: "应用管理", icon: <AppstoreOutlined /> },
  { key: "/files", label: "文件管理", icon: <FolderOutlined /> },
  { key: "/users", label: "用户管理", icon: <TeamOutlined />, adminOnly: true },
  { key: "/audit", label: "审计日志", icon: <AuditOutlined />, adminOnly: true },
];

/** 由 pathname 推导出一级菜单 key。 */
function resolveMenuKey(pathname: string): string {
  const matched = MENU_ENTRIES.map((entry) => entry.key).find(
    (key) => pathname === key || pathname.startsWith(`${key}/`)
  );
  return matched ?? "/apps";
}

/** 面包屑：固定两项足够覆盖当前路由层级。 */
function resolveBreadcrumb(pathname: string): string[] {
  const segments = pathname.split("/").filter(Boolean);
  if (segments.length === 0) return [];
  const root = resolveMenuKey(pathname);
  const rootLabel = MENU_ENTRIES.find((entry) => entry.key === root)?.label ?? "";
  if (segments.length === 1) return [rootLabel];
  if (root === "/apps") return [rootLabel, "应用详情"];
  return [rootLabel];
}

const Layout = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();

  const menuItems = useMemo<MenuProps["items"]>(
    () =>
      MENU_ENTRIES.filter((entry) => !entry.adminOnly || isAdmin(user?.role)).map((entry) => ({
        key: entry.key,
        icon: entry.icon,
        label: entry.label,
      })),
    [user?.role]
  );

  const selectedKey = resolveMenuKey(location.pathname);
  const breadcrumbItems = useMemo(
    () => resolveBreadcrumb(location.pathname).map((title) => ({ title })),
    [location.pathname]
  );

  const userMenu: MenuProps = {
    items: [
      { key: "change-password", icon: <KeyOutlined />, label: "修改密码" },
      { type: "divider" },
      { key: "logout", icon: <LogoutOutlined />, label: "退出登录", danger: true },
    ],
    onClick: ({ key }) => {
      if (key === "logout") {
        logout();
        navigate("/login", { replace: true });
        return;
      }
      navigate("/change-password");
    },
  };

  return (
    <AntLayout style={{ minHeight: "100vh" }}>
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 24px",
          background: "#fff",
          borderBottom: "1px solid #f0f0f0",
        }}
      >
        <div style={{ fontSize: 18, fontWeight: 600 }}>软件版本管理系统</div>
        <Dropdown menu={userMenu} placement="bottomRight">
          <Space style={{ cursor: "pointer" }}>
            <Avatar size="small" icon={<UserOutlined />} />
            <span>{user?.displayName || user?.username || "未登录"}</span>
            {user && <Tag color="blue">{roleLabel(user.role)}</Tag>}
          </Space>
        </Dropdown>
      </Header>
      <AntLayout hasSider>
        <Sider
          width={200}
          style={{ background: "#fff", overflow: "auto", height: "calc(100vh - 64px)", position: "fixed", left: 0 }}
        >
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            style={{ height: "100%", borderRight: 0 }}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
          />
        </Sider>
        <AntLayout style={{ marginLeft: 200 }}>
          <Content style={{ margin: 24 }}>
            <Breadcrumb items={breadcrumbItems} style={{ marginBottom: 16 }} />
            <div style={{ background: "#fff", padding: 24, minHeight: 280, borderRadius: 8 }}>
              <ErrorBoundary>
                <Outlet />
              </ErrorBoundary>
            </div>
          </Content>
        </AntLayout>
      </AntLayout>
    </AntLayout>
  );
};

export default Layout;
