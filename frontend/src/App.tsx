import { Navigate, Route, Routes, BrowserRouter } from "react-router-dom";
import { App as AntApp, ConfigProvider } from "antd";
import zhCN from "antd/locale/zh_CN";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Layout from "./components/Layout";
import { AuthProvider } from "./features/auth/AuthContext";
import { RequireAdmin, RequireAuth } from "./features/auth/RequireAuth";
import Login from "./pages/Login";
import ChangePassword from "./pages/ChangePassword";
import Apps from "./pages/Apps";
import AppDetail from "./pages/AppDetail";
import Files from "./pages/Files";
import Users from "./pages/Users";
import Audit from "./pages/Audit";
import NotFound from "./pages/NotFound";
import SharePortal from "./features/share-portal/SharePortal";
import { appTheme } from "./styles/theme";

// 默认不重试、不自动聚焦刷新：列表类页面更适合显式 refresh
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 0, refetchOnWindowFocus: false },
  },
});

const App = () => {
  return (
    <ConfigProvider locale={zhCN} theme={appTheme}>
      <AntApp>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <AuthProvider>
              <Routes>
                {/* 分享前台与登录页不套后台框架 */}
                <Route path="/share/:token" element={<SharePortal />} />
                <Route path="/login" element={<Login />} />
                <Route
                  path="/change-password"
                  element={
                    <RequireAuth>
                      <ChangePassword />
                    </RequireAuth>
                  }
                />
                <Route
                  path="/"
                  element={
                    <RequireAuth>
                      <Layout />
                    </RequireAuth>
                  }
                >
                  <Route index element={<Navigate to="/apps" replace />} />
                  <Route path="apps" element={<Apps />} />
                  <Route path="apps/:id" element={<AppDetail />} />
                  <Route path="files" element={<Files />} />
                  <Route
                    path="users"
                    element={
                      <RequireAdmin>
                        <Users />
                      </RequireAdmin>
                    }
                  />
                  <Route
                    path="audit"
                    element={
                      <RequireAdmin>
                        <Audit />
                      </RequireAdmin>
                    }
                  />
                </Route>
                <Route path="*" element={<NotFound />} />
              </Routes>
            </AuthProvider>
          </BrowserRouter>
        </QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  );
};

export default App;
