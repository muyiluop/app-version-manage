/**
 * 发布 / 编辑版本弹窗。
 *
 * 关键点：产物先调 /v2/files/upload 拿到 fileKey，再随版本一起提交；
 * 服务端能由 fileKey 补全 fileName/fileSize/fileSha256/contentType，
 * 这里保留上传返回值是为了让发布者在提交前就能核对产物信息。
 */
import { useEffect, useState } from "react";
import { Form, Input, Modal, Select, Space, Switch, Tag, Typography, Upload, type UploadProps } from "antd";
import { InboxOutlined } from "@ant-design/icons";
import * as versionsApi from "../../api/versions";
import { uploadFile } from "../../api/files";
import { useSubmit } from "../../hooks/useSubmit";
import type { Application, Channel, Platform, StoredFile, Version, VersionStatus } from "../../types/api";
import { VERSION_STATUS_OPTIONS, formatBytes, platformLabel } from "../../utils/format";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  app: Application;
  channels: Channel[];
  /** 传入表示编辑已有版本，null 表示新发布。 */
  version: Version | null;
  onCancel: () => void;
  onSaved: () => void;
}

interface FormValues {
  version: string;
  platform: Platform;
  channel: string;
  fileKey?: string;
  changelog?: string;
  ext?: string;
  forceUpdate?: boolean;
  minSupportedVersion?: string;
  status: VersionStatus;
}

/** 宽松的语义化版本校验，最终以服务端解析为准。 */
const SEMVER_PATTERN = /^\d+(\.\d+){0,3}([-+][0-9A-Za-z.-]+)?$/;

/** ext 必须是非数组的 JSON 对象。 */
function validateExt(value: string | undefined): string | null {
  if (!value || !value.trim()) return null;
  try {
    const parsed: unknown = JSON.parse(value);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return "ext 必须是 JSON 对象，例如 {\"key\": \"value\"}";
    }
    return null;
  } catch {
    return "ext 不是合法的 JSON";
  }
}

export default function PublishVersionModal({ open, app, channels, version, onCancel, onSaved }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [submitting, submit] = useSubmit();
  const [uploading, setUploading] = useState(false);
  /** 本次会话新上传的产物，优先于编辑目标版本自带的元数据。 */
  const [uploaded, setUploaded] = useState<StoredFile | null>(null);
  const fileKey = Form.useWatch("fileKey", form);
  const isEdit = version !== null;

  useEffect(() => {
    if (!open) return;
    setUploaded(null);
    if (version) {
      form.setFieldsValue({
        version: version.version,
        platform: version.platform,
        channel: version.channel,
        fileKey: version.fileKey,
        changelog: version.changelog,
        ext: version.ext,
        forceUpdate: version.forceUpdate,
        minSupportedVersion: version.minSupportedVersion,
        status: version.status,
      });
      return;
    }
    form.resetFields();
    form.setFieldsValue({
      platform: app.platforms[0],
      channel: app.defaultChannel || channels[0]?.key || "stable",
      forceUpdate: false,
      status: "published",
    });
  }, [open, version, app, channels, form]);

  const handleUpload: UploadProps["customRequest"] = async (options) => {
    const file = options.file as unknown as File;
    setUploading(true);
    try {
      const result = await uploadFile(file);
      setUploaded(result.file);
      form.setFieldValue("fileKey", result.file.key);
      options.onSuccess?.(result);
      showMessage.success(result.deduplicated ? "文件已存在，秒传成功" : "产物上传成功");
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error("产物上传失败"));
    } finally {
      setUploading(false);
    }
  };

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      const payload = {
        platform: values.platform,
        channel: values.channel,
        version: values.version.trim(),
        fileKey: values.fileKey ?? "",
        changelog: values.changelog ?? "",
        ext: values.ext ?? "",
        forceUpdate: values.forceUpdate ?? false,
        minSupportedVersion: values.minSupportedVersion?.trim() ?? "",
        status: values.status,
      };
      if (version) {
        await versionsApi.updateVersion(version.id, payload);
        showMessage.success("版本更新成功");
      } else {
        await versionsApi.publishVersion({ appId: app.id, ...payload });
        showMessage.success("版本发布成功");
      }
      onSaved();
    });

  const fileName = uploaded?.name ?? version?.fileName ?? "";
  const fileSize = uploaded?.size ?? version?.fileSize ?? 0;
  const fileSha256 = uploaded?.sha256 ?? version?.fileSha256 ?? "";

  return (
    <Modal
      title={isEdit ? `编辑版本 ${version?.version ?? ""}` : "发布版本"}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      destroyOnHidden
      width={640}
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Space size={16} style={{ display: "flex" }} align="start">
          <Form.Item
            name="version"
            label="版本号"
            style={{ flex: 1 }}
            rules={[
              { required: true, message: "请输入版本号" },
              { pattern: SEMVER_PATTERN, message: "版本号格式不合法，例如 1.2.3" },
            ]}
          >
            <Input placeholder="例如 1.2.3" maxLength={64} />
          </Form.Item>
          <Form.Item
            name="platform"
            label="发布平台"
            style={{ flex: 1 }}
            rules={[{ required: true, message: "请选择发布平台" }]}
          >
            <Select
              placeholder="请选择平台"
              options={app.platforms.map((platform) => ({ label: platformLabel(platform), value: platform }))}
            />
          </Form.Item>
          <Form.Item
            name="channel"
            label="发布通道"
            style={{ flex: 1 }}
            rules={[{ required: true, message: "请选择发布通道" }]}
          >
            <Select
              placeholder="请选择通道"
              options={channels.map((channel) => ({
                label: `${channel.name || channel.key}${channel.isDefault ? "（默认）" : ""}`,
                value: channel.key,
              }))}
            />
          </Form.Item>
        </Space>

        <Form.Item
          label="版本产物"
          required={!isEdit}
          extra={isEdit ? "不重新上传则沿用原产物" : "相同 SHA256 的文件会自动秒传"}
        >
          <Form.Item name="fileKey" hidden>
            <Input />
          </Form.Item>
          <Upload.Dragger multiple={false} showUploadList={false} customRequest={handleUpload} disabled={uploading}>
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">{uploading ? "上传中..." : "点击或拖拽文件到此处上传"}</p>
            <p className="ant-upload-hint">上传完成后自动带出 fileKey、文件大小与 SHA256</p>
          </Upload.Dragger>
          {fileKey && (
            <div style={{ marginTop: 12, padding: 12, background: "#fafafa", borderRadius: 6 }}>
              <Space direction="vertical" size={4} style={{ width: "100%" }}>
                <span>文件名：{fileName || "-"}</span>
                <span>文件大小：{formatBytes(fileSize)}</span>
                <span style={{ wordBreak: "break-all" }}>
                  fileKey：<Typography.Text code>{fileKey}</Typography.Text>
                </span>
                <span style={{ wordBreak: "break-all" }}>SHA256：{fileSha256 || "-"}</span>
                {isEdit && !uploaded && <Tag color="blue">沿用该版本已有产物</Tag>}
              </Space>
            </div>
          )}
        </Form.Item>

        <Form.Item
          name="ext"
          label="扩展信息（ext）"
          extra="必须为 JSON 对象，会在客户端获取版本时原样返回"
          rules={[
            {
              validator: (_, value: string | undefined) => {
                const error = validateExt(value);
                return error ? Promise.reject(new Error(error)) : Promise.resolve();
              },
            },
          ]}
        >
          <Input.TextArea rows={3} placeholder='{"key": "value"}' />
        </Form.Item>

        <Form.Item
          name="minSupportedVersion"
          label="最低支持版本"
          tooltip="低于该版本的客户端将被判定为强制更新"
          rules={[{ pattern: SEMVER_PATTERN, message: "版本号格式不合法" }]}
        >
          <Input placeholder="例如 1.0.0，留空表示不限制" maxLength={64} />
        </Form.Item>

        <Form.Item name="changelog" label="更新日志">
          <Input.TextArea rows={4} placeholder="请输入更新日志" />
        </Form.Item>

        <Space size={48}>
          <Form.Item name="forceUpdate" label="强制更新" valuePropName="checked">
            <Switch checkedChildren="是" unCheckedChildren="否" />
          </Form.Item>
          <Form.Item name="status" label="版本状态" rules={[{ required: true, message: "请选择版本状态" }]}>
            <Select style={{ width: 160 }} options={VERSION_STATUS_OPTIONS} />
          </Form.Item>
        </Space>
      </Form>
    </Modal>
  );
}
