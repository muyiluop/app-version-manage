import React from "react";
import { Layout as AntLayout, Menu } from "antd";
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { AppstoreOutlined, FolderOutlined } from "@ant-design/icons";

const { Header, Content, Sider } = AntLayout;

const Layout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const menuItems = [
    { key: "/apps", icon: <AppstoreOutlined />, label: "应用管理" },
    { key: "/files", icon: <FolderOutlined />, label: "文件管理" },
  ];

  return (
    <AntLayout style={{ minHeight: "100vh" }}>
      <Header style={{ padding: 0, background: "#fff" }}>
        <div style={{ margin: "0 16px", lineHeight: "64px", fontSize: "18px", fontWeight: "bold" }}>
          软件版本管理系统
        </div>
      </Header>{" "}
      <AntLayout hasSider>
        <Sider
          width={200}
          style={{
            background: "#fff",
            overflow: "auto",
            height: "calc(100vh - 64px)",
            position: "fixed",
            left: 0,
          }}
        >
          <Menu
            mode="inline"
            selectedKeys={[location.pathname]}
            style={{ height: "100%", borderRight: 0 }}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
          />
        </Sider>
        <AntLayout style={{ marginLeft: 200 }}>
          <Content style={{ margin: "24px", background: "#fff", padding: 24, minHeight: 280 }}>
            <Outlet />
          </Content>
        </AntLayout>
      </AntLayout>
    </AntLayout>
  );
};

export default Layout;
