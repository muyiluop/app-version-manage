/**
 * 版本详情抽屉：展示契约里的全部字段，并提供产物下载。
 */
import { Button, Descriptions, Drawer, Space, Tag, Typography } from "antd";
import { CopyOutlined, DownloadOutlined } from "@ant-design/icons";
import * as versionsApi from "../../api/versions";
import { useSubmit } from "../../hooks/useSubmit";
import type { Version } from "../../types/api";
import { copyText, formatBytes, formatDateTime, platformLabel, versionStatusMeta } from "../../utils/format";
import { saveBlob } from "../../utils/download";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  version: Version | null;
  onClose: () => void;
}

/** ext 是 JSON 文本，展示时格式化为缩进形式，解析失败则原样显示。 */
function prettyExt(ext: string): string {
  if (!ext) return "-";
  try {
    return JSON.stringify(JSON.parse(ext), null, 2);
  } catch {
    return ext;
  }
}

export default function VersionDetailDrawer({ open, version, onClose }: Props) {
  const [downloading, submitDownload] = useSubmit();

  if (!version) return null;
  const status = versionStatusMeta(version.status);

  const handleDownload = () =>
    submitDownload(async () => {
      const blob = await versionsApi.downloadVersion(version.id);
      saveBlob(blob, version.fileName || `version-${version.version}`);
    });

  return (
    <Drawer
      title={`版本详情 · ${version.version}`}
      open={open}
      onClose={onClose}
      width={640}
      extra={
        <Button type="primary" icon={<DownloadOutlined />} loading={downloading} onClick={handleDownload}>
          下载产物
        </Button>
      }
    >
      <Descriptions column={2} bordered size="small">
        <Descriptions.Item label="版本号">{version.version}</Descriptions.Item>
        <Descriptions.Item label="预发布">
          {version.prerelease ? <Tag>{version.prerelease}</Tag> : "-"}
        </Descriptions.Item>
        <Descriptions.Item label="平台">{platformLabel(version.platform)}</Descriptions.Item>
        <Descriptions.Item label="通道">
          <Tag color="blue">{version.channel}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="状态">
          <Tag color={status.color}>{status.label}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="强制更新">
          {version.forceUpdate ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="最低支持版本">{version.minSupportedVersion || "-"}</Descriptions.Item>
        <Descriptions.Item label="下载次数">{version.downloadCount}</Descriptions.Item>
        <Descriptions.Item label="文件名" span={2}>
          {version.fileName || "-"}
        </Descriptions.Item>
        <Descriptions.Item label="文件大小">{formatBytes(version.fileSize)}</Descriptions.Item>
        <Descriptions.Item label="Content-Type">{version.contentType || "-"}</Descriptions.Item>
        <Descriptions.Item label="fileKey" span={2}>
          <Typography.Text code copyable={{ text: version.fileKey }}>
            {version.fileKey}
          </Typography.Text>
        </Descriptions.Item>
        <Descriptions.Item label="SHA256" span={2}>
          {version.fileSha256 ? (
            <Space>
              <Typography.Text code style={{ wordBreak: "break-all" }}>
                {version.fileSha256}
              </Typography.Text>
              <Button
                type="text"
                size="small"
                icon={<CopyOutlined />}
                onClick={async () => {
                  const ok = await copyText(version.fileSha256);
                  if (ok) showMessage.success("已复制 SHA256");
                }}
              />
            </Space>
          ) : (
            "-"
          )}
        </Descriptions.Item>
        <Descriptions.Item label="发布时间">{formatDateTime(version.publishedAt)}</Descriptions.Item>
        <Descriptions.Item label="创建时间">{formatDateTime(version.createdAt)}</Descriptions.Item>
        <Descriptions.Item label="更新时间" span={2}>
          {formatDateTime(version.updatedAt)}
        </Descriptions.Item>
        <Descriptions.Item label="更新日志" span={2}>
          <pre style={{ whiteSpace: "pre-wrap", margin: 0, maxHeight: 240, overflow: "auto" }}>
            {version.changelog || "-"}
          </pre>
        </Descriptions.Item>
        <Descriptions.Item label="扩展信息" span={2}>
          <pre style={{ whiteSpace: "pre-wrap", margin: 0, maxHeight: 240, overflow: "auto" }}>
            {prettyExt(version.ext)}
          </pre>
        </Descriptions.Item>
      </Descriptions>
    </Drawer>
  );
}
