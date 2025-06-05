import React from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ConfigProvider, theme, App as AntApp } from "antd";
import "./App.css";
import Login from "./pages/Login";
import Layout from "./components/Layout";
import AppList from "./pages/AppList";
import AppDetail from "./pages/AppDetail";
import FileManager from "./pages/FileManager";
import VersionDetail from "./pages/VersionDetail";
import AppShare from "./pages/AppShare";

const App: React.FC = () => {
  return (
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: "#1677ff",
        },
      }}
    >
      <AntApp>
        <BrowserRouter>
          <Routes>
            <Route path="/share/:token" element={<AppShare />} />
            <Route path="/login" element={<Login />} />
            <Route path="/" element={<Layout />}>
              <Route index element={<Navigate to="/apps" replace />} />
              <Route path="apps" element={<AppList />} />
              <Route path="apps/:id" element={<AppDetail />} />
              <Route path="versions/:versionId" element={<VersionDetail />} />
              <Route path="files" element={<FileManager />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  );
};

export default App;
