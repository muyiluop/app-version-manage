/**
 * 后台主框架：深色侧边导航 + 顶部栏 + 内容区。
 *
 * 设计要点：
 * - 侧栏固定定位并支持折叠（状态记在 localStorage，刷新后保持）；
 * - 菜单选中项由 pathname 派生（不能直接用 pathname 作 key，否则 /apps/1 不会高亮）；
 * - 顶部栏 sticky，承载折叠按钮、面包屑与用户菜单；
 * - 颜色与圆角全部来自样式令牌，组件内不再写死颜色。
 */
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Avatar, Breadcrumb, Dropdown, Layout as AntLayout, Menu, Tag, Tooltip, type MenuProps } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import {
  AppstoreOutlined,
  AuditOutlined,
  FolderOutlined,
  KeyOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import ErrorBoundary from "./ErrorBoundary";
import { useAuth } from "../features/auth/AuthContext";
import { isAdmin, roleLabel } from "../utils/auth";

const { Header, Content, Sider } = AntLayout;

const COLLAPSE_KEY = "avm_sider_collapsed";

interface MenuEntry {
  key: string;
  label: string;
  icon: ReactNode;
  /** 仅管理员可见。 */
  adminOnly?: boolean;
}

// 「文件管理」暂时从侧栏隐藏。
//
// 原因：该页展示的其实是上传的副产物——发布产物、应用图标上传时都会写入 files 表，
// 页面本身可做的事有限，而其中真正被依赖的「秒传去重」与「清理未使用文件」
// 都在后端接口上，不依赖这个页面。
//
// 这里只摘掉菜单入口：路由 /files 与全部接口保持可用（直接访问仍可打开），
// 需要恢复时把开关改回 true 即可。
const SHOW_FILE_MANAGER = false;

/** 全部入口：参与菜单选中与面包屑解析，即使某项不展示。 */
const ALL_MENU_ENTRIES: MenuEntry[] = [
  { key: "/apps", label: "应用管理", icon: <AppstoreOutlined /> },
  { key: "/files", label: "文件管理", icon: <FolderOutlined /> },
  { key: "/users", label: "用户管理", icon: <TeamOutlined />, adminOnly: true },
  { key: "/audit", label: "审计日志", icon: <AuditOutlined />, adminOnly: true },
];

/** 实际在侧栏展示的入口。 */
const MENU_ENTRIES: MenuEntry[] = ALL_MENU_ENTRIES.filter(
  (entry) => entry.key !== "/files" || SHOW_FILE_MANAGER
);

/** 由 pathname 推导出一级菜单 key。 */
function resolveMenuKey(pathname: string): string {
  const matched = ALL_MENU_ENTRIES.map((entry) => entry.key).find(
    (key) => pathname === key || pathname.startsWith(`${key}/`)
  );
  return matched ?? "/apps";
}

/** 面包屑：一级菜单名 + 二级语义化名称。 */
function resolveBreadcrumb(pathname: string): string[] {
  const segments = pathname.split("/").filter(Boolean);
  if (segments.length === 0) return [];
  const root = resolveMenuKey(pathname);
  const rootLabel = ALL_MENU_ENTRIES.find((entry) => entry.key === root)?.label ?? "";
  if (segments.length === 1) return [rootLabel];
  if (root === "/apps") {
    return [rootLabel, segments[0] === "apps" ? "应用详情" : segments[1]];
  }
  return [rootLabel];
}

const Layout = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuth();

  const [collapsed, setCollapsed] = useState<boolean>(
    () => localStorage.getItem(COLLAPSE_KEY) === "1"
  );

  useEffect(() => {
    localStorage.setItem(COLLAPSE_KEY, collapsed ? "1" : "0");
  }, [collapsed]);

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

  const siderWidth = collapsed ? 72 : 240;

  return (
    <AntLayout className="app-shell">
      <Sider
        className="app-sider"
        theme="dark"
        width={240}
        collapsedWidth={72}
        collapsed={collapsed}
        breakpoint="lg"
        onBreakpoint={(broken) => setCollapsed(broken)}
        trigger={null}
      >
        <div className="app-brand">
          <span className="app-brand__mark">V</span>
          {!collapsed && (
            <span className="app-brand__text">
              <span className="app-brand__title">版本发布系统</span>
              <span className="app-brand__sub">App Version Manage</span>
            </span>
          )}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>

      <AntLayout className="app-main" style={{ marginLeft: siderWidth, transition: "margin-left 0.2s" }}>
        <Header className="app-header">
          <div className="app-header__left">
            <Tooltip title={collapsed ? "展开菜单" : "收起菜单"}>
              <button
                type="button"
                className="app-header__toggle"
                aria-label={collapsed ? "展开菜单" : "收起菜单"}
                onClick={() => setCollapsed((value) => !value)}
              >
                {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              </button>
            </Tooltip>
            <Breadcrumb className="app-header__breadcrumb" items={breadcrumbItems} />
          </div>

          <div className="app-header__left">
            <Dropdown menu={userMenu} placement="bottomRight" trigger={["click"]}>
              <div className="app-header__user">
                <Avatar size={28} style={{ background: "#1677ff" }} icon={<UserOutlined />} />
                <span className="app-header__username">
                  {user?.displayName || user?.username || "未登录"}
                </span>
                {user && <Tag color="blue" style={{ marginInlineEnd: 0 }}>{roleLabel(user.role)}</Tag>}
              </div>
            </Dropdown>
          </div>
        </Header>

        <Content className="app-content">
          <ErrorBoundary>
            <Outlet />
          </ErrorBoundary>
        </Content>
      </AntLayout>
    </AntLayout>
  );
};

export default Layout;
