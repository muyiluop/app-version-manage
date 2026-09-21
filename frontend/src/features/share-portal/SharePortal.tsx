/**
 * 分享前台门户 /share/:token。
 *
 * 说明：v1 兼容的分享接口不返回 channel 与 sha256，因此这里额外调用公开的
 * /v2/check 做一次只读补全；补全失败不影响主流程展示。
 * 保留移动端适配：deviceType 控制间距与历史版本抽屉方向（移动端从底部弹出）。
 */
import { useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  List,
  Pagination,
  Result,
  Skeleton,
  Space,
  Tag,
  Typography,
} from "antd";
import { CopyOutlined, DownloadOutlined, HistoryOutlined } from "@ant-design/icons";
import PasswordGate from "./PasswordGate";
import { accessShare, checkUpdate, isPasswordRequired, listShareVersions, openDownloadUrl } from "../../api/open";
import { getErrorMessage } from "../../api/client";
import { downloadByUrl } from "../../utils/download";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import type { CheckResult, LegacyVersion, Platform, ShareAccessResult } from "../../types/api";
import { PLATFORMS } from "../../types/api";
import { copyText, formatBytes, formatDateTime, logoUrl, platformIcon, platformLabel } from "../../utils/format";
import { showMessage } from "../../utils/message";

const { Title, Paragraph } = Typography;

type Stage = "loading" | "password" | "ready" | "error";

const HISTORY_PAGE_SIZE = 10;

/** 移动端判定，用于调整间距与抽屉方向。 */
function isMobile(): boolean {
  return /mobile|android|iphone|ipad|phone/i.test(navigator.userAgent.toLowerCase());
}

/** 本地兜底平台识别，后端 X-Platform/UA 未给出结果时使用。 */
function detectPlatform(): Platform {
  const ua = navigator.userAgent.toLowerCase();
  if (ua.includes("android")) return "android";
  if (ua.includes("iphone") || ua.includes("ipad")) return "ios";
  if (ua.includes("harmony")) return "harmony";
  if (ua.includes("macintosh") || ua.includes("mac os")) return "macos";
  if (ua.includes("linux")) return "linux";
  return "windows";
}

function normalizePlatform(raw: string | undefined): Platform | undefined {
  const value = (raw ?? "").trim().toLowerCase();
  return (PLATFORMS as readonly string[]).includes(value) ? (value as Platform) : undefined;
}

export default function SharePortal() {
  const { token = "" } = useParams<{ token: string }>();
  const storageKey = `share_password_${token}`;
  const passwordRef = useRef<string>(sessionStorage.getItem(storageKey) ?? "");
  const deviceType = useMemo(() => (isMobile() ? "mobile" : "desktop"), []);
  const fallbackPlatform = useMemo(() => detectPlatform(), []);

  const [attempt, setAttempt] = useState(0);
  const [stage, setStage] = useState<Stage>("loading");
  const [passwordError, setPasswordError] = useState("");
  const [errorText, setErrorText] = useState("");
  const [payload, setPayload] = useState<ShareAccessResult | null>(null);

  const [historyOpen, setHistoryOpen] = useState(false);
  const [historyPage, setHistoryPage] = useState(1);

  const currentPlatform = useMemo<Platform | undefined>(
    () => normalizePlatform(payload?.currentPlatform) ?? fallbackPlatform,
    [payload?.currentPlatform, fallbackPlatform]
  );

  // 访问分享：attempt 变化触发重试，避免用同一个密码重复请求时被依赖去重挡住
  useEffect(() => {
    if (!token) {
      setStage("error");
      setErrorText("分享链接无效");
      return;
    }
    let cancelled = false;
    setStage("loading");
    accessShare(token, { password: passwordRef.current })
      .then((result) => {
        if (cancelled) return;
        if (isPasswordRequired(result)) {
          setStage("password");
          setPasswordError(passwordRef.current ? "密码错误，请重新输入" : "");
          return;
        }
        setPayload(result);
        setPasswordError("");
        setStage("ready");
        document.title = result.app.name || "应用分享";
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        if (passwordRef.current) {
          // 已带密码仍失败：判定为密码错误并清除本地记忆，避免反复自动失败
          sessionStorage.removeItem(storageKey);
          passwordRef.current = "";
          setStage("password");
          setPasswordError(getErrorMessage(error) || "密码错误，请重新输入");
          return;
        }
        setStage("error");
        setErrorText(getErrorMessage(error) || "分享链接无效或已失效");
        document.title = "应用分享";
      });
    return () => {
      cancelled = true;
    };
  }, [token, attempt, storageKey]);

  // 渠道与 sha256 只能从 v2 check 补全（v1 分享接口没有这两个字段）
  const { data: checkResult } = useRequest(
    ["share-check", payload?.app.identifier ?? "", currentPlatform ?? ""],
    () => checkUpdate({ identifier: payload?.app.identifier ?? "", platform: currentPlatform }),
    { enabled: stage === "ready" && Boolean(payload?.app.identifier) }
  );

  const { data: historyData, loading: historyLoading } = useRequest(
    ["share-history", token, currentPlatform ?? "", historyPage],
    () =>
      listShareVersions(token, {
        password: passwordRef.current,
        platform: currentPlatform,
        page: historyPage,
        pageSize: HISTORY_PAGE_SIZE,
      }),
    { enabled: historyOpen }
  );

  const [downloading, submitDownload] = useSubmit();

  if (stage === "loading") {
    return (
      <div style={{ maxWidth: 900, margin: "0 auto", padding: deviceType === "mobile" ? 16 : 24 }}>
        <Card>
          <Skeleton active avatar paragraph={{ rows: 4 }} />
        </Card>
      </div>
    );
  }

  if (stage === "password") {
    return (
      <PasswordGate
        error={passwordError}
        loading={false}
        onSubmit={(value) => {
          passwordRef.current = value;
          sessionStorage.setItem(storageKey, value);
          setPasswordError("");
          setAttempt((current) => current + 1);
        }}
      />
    );
  }

  if (stage === "error" || !payload) {
    return <Result status="404" title="访问失败" subTitle={errorText || "分享链接无效或已过期"} />;
  }

  const versions = payload.versions;
  const currentVersion = versions.find((item) => item.platform === currentPlatform) ?? versions[0];
  const otherPlatforms = versions.filter((item) => item !== currentVersion);
  const check: CheckResult | undefined = checkResult;
  const channel = check?.channel;
  const sha256 = check?.latest.sha256;

  const handleDownload = (version: LegacyVersion) =>
    submitDownload(async () => {
      downloadByUrl(openDownloadUrl(version.filePath), version.fileName);
      showMessage.success("已开始下载");
    });

  const handleCopySha = async (value: string) => {
    const ok = await copyText(value);
    if (ok) showMessage.success("已复制 SHA256");
  };

  const renderVersionMeta = (version: LegacyVersion) => (
    <Space direction="vertical" size={2} style={{ width: "100%" }}>
      <span>发布于：{formatDateTime(version.createdAt)}</span>
      {version.changelog && <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{version.changelog}</pre>}
    </Space>
  );

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: deviceType === "mobile" ? 16 : 24 }}>
      <Card>
        <div style={{ display: "flex", gap: 24, alignItems: "flex-start" }}>
          {payload.app.logo && (
            <img
              src={logoUrl(payload.app.logo)}
              alt={`${payload.app.name} 图标`}
              style={{ width: 96, height: 96, objectFit: "contain", borderRadius: 8, background: "#fafafa" }}
            />
          )}
          <div style={{ flex: 1 }}>
            <Title level={2} style={{ marginTop: 0, marginBottom: 8 }}>
              {payload.app.name}
            </Title>
            <Paragraph type="secondary" style={{ marginBottom: 8 }}>
              {payload.app.identifier}
            </Paragraph>
            {payload.app.description && <Paragraph style={{ marginBottom: 8 }}>{payload.app.description}</Paragraph>}
            <Space wrap>
              {channel && <Tag color="blue">渠道：{channel}</Tag>}
              {payload.app.platforms.map((platform) => (
                <Tag key={platform}>{platformLabel(platform)}</Tag>
              ))}
            </Space>
          </div>
        </div>
      </Card>

      <Card style={{ marginTop: 24 }}>
        {currentVersion ? (
          <>
            <Space style={{ marginBottom: 16 }} wrap>
              {platformIcon(currentVersion.platform)}
              <span>当前平台：{platformLabel(currentVersion.platform)}</span>
              {check?.hasUpdate === false && <Tag color="green">已是最新</Tag>}
              {check?.forceUpdate && <Tag color="red">强制更新</Tag>}
            </Space>
            <Descriptions column={deviceType === "mobile" ? 1 : 2} bordered size="small">
              <Descriptions.Item label="最新版本">{currentVersion.version}</Descriptions.Item>
              <Descriptions.Item label="发布渠道">{channel ?? "-"}</Descriptions.Item>
              <Descriptions.Item label="发布时间">{formatDateTime(currentVersion.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="文件大小">{formatBytes(currentVersion.fileSize)}</Descriptions.Item>
              <Descriptions.Item label="强制更新">
                {currentVersion.forceUpdate ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="最低支持版本">{check?.minSupportedVersion || "-"}</Descriptions.Item>
              <Descriptions.Item label="SHA256" span={deviceType === "mobile" ? 1 : 2}>
                {sha256 ? (
                  <Space>
                    <Typography.Text style={{ fontFamily: "monospace", fontSize: 12, wordBreak: "break-all" }}>
                      {sha256}
                    </Typography.Text>
                    <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => handleCopySha(sha256)} />
                  </Space>
                ) : (
                  "-"
                )}
              </Descriptions.Item>
              {currentVersion.changelog && (
                <Descriptions.Item label="更新内容" span={deviceType === "mobile" ? 1 : 2}>
                  <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{currentVersion.changelog}</pre>
                </Descriptions.Item>
              )}
            </Descriptions>
            {check?.belowMinimumVersion && (
              <Alert
                type="warning"
                showIcon
                style={{ marginTop: 16 }}
                message="当前版本已低于最低支持版本，请升级到最新版本"
              />
            )}
            <div style={{ marginTop: 16, textAlign: "center" }}>
              <Space wrap>
                <Button
                  type="primary"
                  icon={<DownloadOutlined />}
                  size="large"
                  loading={downloading}
                  onClick={() => handleDownload(currentVersion)}
                >
                  下载最新版本
                </Button>
                <Button
                  icon={<HistoryOutlined />}
                  size="large"
                  onClick={() => {
                    setHistoryPage(1);
                    setHistoryOpen(true);
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

      {otherPlatforms.length > 0 && (
        <Card title="其他平台" style={{ marginTop: 24 }}>
          <List
            dataSource={otherPlatforms}
            renderItem={(version) => (
              <List.Item
                actions={[
                  <Button key="download" type="link" onClick={() => handleDownload(version)}>
                    下载
                  </Button>,
                  <Button
                    key="history"
                    type="link"
                    onClick={() => {
                      setHistoryPage(1);
                      setHistoryOpen(true);
                    }}
                  >
                    历史版本
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  avatar={platformIcon(version.platform)}
                  title={`${platformLabel(version.platform)} · v${version.version}`}
                  description={renderVersionMeta(version)}
                />
              </List.Item>
            )}
          />
        </Card>
      )}

      <Drawer
        title="历史版本"
        placement={deviceType === "mobile" ? "bottom" : "right"}
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        width={deviceType === "mobile" ? "100%" : 560}
        height={deviceType === "mobile" ? "80%" : undefined}
      >
        <List
          loading={historyLoading}
          dataSource={historyData?.list ?? []}
          locale={{ emptyText: "暂无历史版本" }}
          renderItem={(version) => (
            <List.Item
              actions={[
                <Button key="download" type="link" onClick={() => handleDownload(version)}>
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
                description={renderVersionMeta(version)}
              />
            </List.Item>
          )}
        />
        {(historyData?.total ?? 0) > HISTORY_PAGE_SIZE && (
          <Pagination
            style={{ marginTop: 16, textAlign: "center" }}
            current={historyPage}
            pageSize={HISTORY_PAGE_SIZE}
            total={historyData?.total ?? 0}
            onChange={(nextPage) => setHistoryPage(nextPage)}
            showSizeChanger={false}
          />
        )}
      </Drawer>
    </div>
  );
}
