import React, { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import {
  Card,
  Typography,
  Button,
  List,
  Space,
  Tag,
  Empty,
  Skeleton,
  Drawer,
  Result,
  Descriptions,
  Modal,
  Input,
  Form,
  message,
} from "antd";
import {
  DownloadOutlined,
  HistoryOutlined,
  DesktopOutlined,
  AppleOutlined,
  WindowsOutlined,
  AndroidOutlined,
  LockOutlined,
} from "@ant-design/icons";
import axios from "axios";
import { downloadFile, getFileUrl } from "../utils/file";
import type { Application, Version, Platform } from "../types";

const { Title, Paragraph } = Typography;

// 获取屏幕类型
const getDeviceType = () => {
  const ua = navigator.userAgent;
  if (/mobile|android|iphone|ipad|phone/i.test(ua.toLowerCase())) {
    return "mobile";
  }
  return "desktop";
};

// 平台图标映射
const PlatformIcon = ({ platform }: { platform: Platform }) => {
  switch (platform) {
    case "windows":
      return <WindowsOutlined />;
    case "macos":
      return <AppleOutlined />;
    case "android":
      return <AndroidOutlined />;
    case "ios":
      return <AppleOutlined />;
    default:
      return <DesktopOutlined />;
  }
};

const AppShare: React.FC = () => {
  const { token } = useParams<{ token: string }>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>("");
  const [app, setApp] = useState<Application | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [currentPlatform, setCurrentPlatform] = useState<string>("");
  const [isHistoryVisible, setIsHistoryVisible] = useState(false);
  const [historyVersions, setHistoryVersions] = useState<Version[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [passwordInput, setPasswordInput] = useState("");
  const [isPasswordModalVisible, setIsPasswordModalVisible] = useState(false);
  const [accessGranted, setAccessGranted] = useState(false);
  const deviceType = getDeviceType();

  // 会话内记忆密码：页面加载时尝试读取并自动授权
  useEffect(() => {
    const key = `share_password_${token}`;
    const saved = sessionStorage.getItem(key);
    if (saved) {
      setPasswordInput(saved);
      setAccessGranted(true);
    }
  }, [token]);

  // 加载应用信息
  useEffect(() => {
    const loadAppData = async () => {
      try {
        setLoading(true);
        const response = await axios.post(`/share/${token}`, {
          password: accessGranted ? passwordInput : "",
        });
        if (response.data.requirePassword) {
          // 需要密码验证
          setIsPasswordModalVisible(true);
          return;
        }
        setApp(response.data.app);
        setVersions(response.data.versions);
        setCurrentPlatform(response.data.currentPlatform);
        // 更新页面标题为应用名称
        document.title = response.data.app.name;
      } catch (error: any) {
        if (error.response?.status === 209) {
          // 需要密码验证
          setIsPasswordModalVisible(true);
        } else {
          const errorMsg = error.response?.data?.error || "加载失败";
          setError(errorMsg);
          // 链接无效或过期等错误，清除已记录的密码
          const key = `share_password_${token}`;
          sessionStorage.removeItem(key);
          // 错误时恢复默认标题
          document.title = "应用分享";
        }
      } finally {
        setLoading(false);
      }
    };

    loadAppData();
  }, [token, accessGranted]);

  // 加载版本历史
  const loadHistoryVersions = async (platform?: string) => {
    try {
      setHistoryLoading(true);
      const response = await axios.post(
        `/share/${token}/versions`,
        { password: passwordInput },
        { params: { platform } }
      );
      setHistoryVersions(response.data.list);
    } catch (error: any) {
      message.error(error.response?.data?.error || "加载版本历史失败");
    } finally {
      setHistoryLoading(false);
    }
  };

  // 处理密码提交
  const handlePasswordSubmit = () => {
    if (!passwordInput) {
      message.error("请输入密码");
      return;
    }
    // 记住密码到会话存储，避免本次会话内重复输入
    const key = `share_password_${token}`;
    sessionStorage.setItem(key, passwordInput);
    setAccessGranted(true);
    setIsPasswordModalVisible(false);
  };
  const handleDownloadVersion = (version: Version) => {
    downloadFile(version.filePath, version.fileName, true);
  };

  if (loading) {
    return (
      <Card>
        <Skeleton active avatar paragraph={{ rows: 4 }} />
      </Card>
    );
  }

  // 需要密码验证时，只显示密码框
  if (isPasswordModalVisible && !app) {
    return (
      <Modal
        title={
          <Space>
            <LockOutlined />
            <span>输入访问密码</span>
          </Space>
        }
        open={isPasswordModalVisible}
        onCancel={() => setIsPasswordModalVisible(false)}
        footer={[
          <Button key="cancel" onClick={() => setIsPasswordModalVisible(false)}>
            取消
          </Button>,
          <Button key="submit" type="primary" onClick={handlePasswordSubmit}>
            提交
          </Button>,
        ]}
      >
        <Form layout="vertical">
          <Form.Item label="密码" required>
            <Input.Password
              placeholder="请输入分享密码"
              value={passwordInput}
              onChange={(e) => setPasswordInput(e.target.value)}
              onPressEnter={handlePasswordSubmit}
            />
          </Form.Item>
        </Form>
      </Modal>
    );
  }

  if (error || !app) {
    return <Result status="404" title="访问失败" subTitle={error || "分享链接无效或已过期"} />;
  }

  // 获取当前平台的版本
  const currentVersion = versions.find((v) => v.platform === currentPlatform);

  // 过滤其他平台的版本
  const otherPlatforms = versions.filter((v) => v.platform !== currentPlatform);

  return (
    <div style={{ maxWidth: 1200, margin: "0 auto", padding: deviceType === "mobile" ? "16px" : "24px" }}>
      {/* 应用信息卡片 */}
      <Card>
        <div style={{ display: "flex", alignItems: "flex-start", gap: "24px" }}>
          {app.logo && (
            <div>
              {" "}
              <img
                src={getFileUrl(app.logo)}
                alt={`${app.name} logo`}
                style={{
                  maxHeight: "120px",
                  maxWidth: "200px",
                  objectFit: "contain",
                  borderRadius: "4px",
                }}
              />
            </div>
          )}
          <div style={{ flex: 1 }}>
            <Title level={2} style={{ marginTop: 0 }}>
              {app.name}
            </Title>
            <Paragraph>{app.description}</Paragraph>
          </div>
        </div>
      </Card>

      {/* 当前平台版本信息 */}
      <Card style={{ marginTop: "24px" }}>
        {currentVersion ? (
          <>
            <Space style={{ marginBottom: 16 }}>
              <PlatformIcon platform={currentVersion.platform as Platform} />
              <span>当前平台: {currentVersion.platform}</span>
            </Space>
            <Descriptions column={deviceType === "mobile" ? 1 : 2} bordered>
              <Descriptions.Item label="最新版本">{currentVersion.version}</Descriptions.Item>
              <Descriptions.Item label="发布时间">
                {new Date(currentVersion.createdAt).toLocaleString()}
              </Descriptions.Item>
              <Descriptions.Item label="文件大小">
                {(currentVersion.fileSize / 1024 / 1024).toFixed(2)} MB
              </Descriptions.Item>
              <Descriptions.Item label="强制更新">
                {currentVersion.forceUpdate ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>}
              </Descriptions.Item>
              {currentVersion.changelog && (
                <Descriptions.Item label="更新内容" span={2}>
                  <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{currentVersion.changelog}</pre>
                </Descriptions.Item>
              )}
            </Descriptions>
            <div style={{ marginTop: 16, textAlign: "center" }}>
              <Space>
                <Button
                  type="primary"
                  icon={<DownloadOutlined />}
                  size="large"
                  onClick={() => handleDownloadVersion(currentVersion)}
                >
                  下载最新版本
                </Button>
                <Button
                  icon={<HistoryOutlined />}
                  onClick={() => {
                    setIsHistoryVisible(true);
                    loadHistoryVersions(currentVersion.platform);
                  }}
                >
                  查看历史版本
                </Button>
              </Space>
            </div>
          </>
        ) : (
          <Empty description="未找到当前平台的版本" />
        )}
      </Card>

      {/* 其他平台版本 */}
      {otherPlatforms.length > 0 && (
        <Card title="其他平台" style={{ marginTop: "24px" }}>
          <List
            dataSource={otherPlatforms}
            renderItem={(version) => (
              <List.Item
                actions={[
                  <Button key="download" type="link" onClick={() => handleDownloadVersion(version)}>
                    下载
                  </Button>,
                  <Button
                    key="history"
                    type="link"
                    onClick={() => {
                      setIsHistoryVisible(true);
                      loadHistoryVersions(version.platform);
                    }}
                  >
                    历史版本
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  avatar={<PlatformIcon platform={version.platform as Platform} />}
                  title={`${version.platform} - v${version.version}`}
                  description={`更新于 ${new Date(version.createdAt).toLocaleString()}`}
                />
              </List.Item>
            )}
          />
        </Card>
      )}

      {/* 历史版本抽屉 */}
      <Drawer
        title="历史版本"
        placement={deviceType === "mobile" ? "bottom" : "right"}
        open={isHistoryVisible}
        onClose={() => setIsHistoryVisible(false)}
        width={deviceType === "mobile" ? "100%" : 600}
        height={deviceType === "mobile" ? "80%" : undefined}
      >
        <List
          loading={historyLoading}
          dataSource={historyVersions}
          renderItem={(version) => (
            <List.Item
              actions={[
                <Button key="download" type="link" onClick={() => handleDownloadVersion(version)}>
                  下载
                </Button>,
              ]}
            >
              <List.Item.Meta
                title={
                  <Space>
                    <span>v{version.version}</span>
                    {version.forceUpdate && <Tag color="red">强制更新</Tag>}
                  </Space>
                }
                description={
                  <>
                    <div>发布于: {new Date(version.createdAt).toLocaleString()}</div>
                    {version.changelog && (
                      <div style={{ marginTop: 8 }}>
                        <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{version.changelog}</pre>
                      </div>
                    )}
                  </>
                }
              />
            </List.Item>
          )}
        />
      </Drawer>
    </div>
  );
};

export default AppShare;
