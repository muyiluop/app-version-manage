/**
 * 创建 / 编辑应用弹窗。
 *
 * 契约要求：identifier 创建后不可改（后端更新接口虽然接收，但一旦改动会破坏
 * 客户端更新检查），因此编辑态一律禁用该输入框。
 */
import { useEffect, useState } from "react";
import { Form, Input, Modal, Select, Upload, type UploadProps } from "antd";
import { LoadingOutlined, PlusOutlined } from "@ant-design/icons";
import * as appsApi from "../../api/apps";
import { uploadFile } from "../../api/files";
import { useSubmit } from "../../hooks/useSubmit";
import type { AppInput, Application, Platform } from "../../types/api";
import { PLATFORMS } from "../../types/api";
import { logoUrl, platformLabel } from "../../utils/format";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  /** 传入表示编辑，null 表示创建。 */
  app: Application | null;
  onCancel: () => void;
  onSaved: (app: Application) => void;
}

interface FormValues {
  name: string;
  identifier: string;
  logo?: string;
  description?: string;
  platforms: Platform[];
}

const PLATFORM_OPTIONS = PLATFORMS.map((platform) => ({ label: platformLabel(platform), value: platform }));

export default function AppFormModal({ open, app, onCancel, onSaved }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [submitting, submit] = useSubmit();
  const [uploading, setUploading] = useState(false);
  const logo = Form.useWatch("logo", form);
  const isEdit = app !== null;

  // 每次打开时按当前应用回填，避免残留上一次的编辑内容
  useEffect(() => {
    if (!open) return;
    if (app) {
      form.setFieldsValue({
        name: app.name,
        identifier: app.identifier,
        logo: app.logo,
        description: app.description,
        platforms: app.platforms,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({ platforms: [] });
    }
  }, [open, app, form]);

  const handleUpload: UploadProps["customRequest"] = async (options) => {
    const file = options.file as unknown as File;
    setUploading(true);
    try {
      const result = await uploadFile(file);
      form.setFieldValue("logo", result.file.key);
      options.onSuccess?.(result);
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error("图标上传失败"));
    } finally {
      setUploading(false);
    }
  };

  const beforeUpload = (file: File) => {
    if (!file.type.startsWith("image/")) {
      showMessage.error("只能上传图片文件");
      return Upload.LIST_IGNORE;
    }
    return true;
  };

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      const payload: AppInput = {
        name: values.name.trim(),
        identifier: values.identifier.trim(),
        logo: values.logo ?? "",
        description: values.description ?? "",
        platforms: values.platforms,
      };
      const saved = isEdit ? await appsApi.updateApp(app.id, payload) : await appsApi.createApp(payload);
      showMessage.success(isEdit ? "应用更新成功" : "应用创建成功");
      onSaved(saved);
    });

  return (
    <Modal
      title={isEdit ? "编辑应用" : "创建应用"}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      destroyOnHidden
      width={560}
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item name="name" label="应用名称" rules={[{ required: true, message: "请输入应用名称" }]}>
          <Input placeholder="请输入应用名称" maxLength={100} />
        </Form.Item>

        <Form.Item
          name="identifier"
          label="应用标识"
          tooltip={isEdit ? "应用标识创建后不可修改" : "用于客户端检查更新，建议使用反向域名"}
          rules={[
            { required: true, message: "请输入应用标识" },
            { pattern: /^[A-Za-z0-9._-]+$/, message: "仅支持字母、数字、点、下划线与中划线" },
          ]}
        >
          <Input placeholder="例如 com.example.app" disabled={isEdit} maxLength={100} />
        </Form.Item>

        <Form.Item name="platforms" label="支持平台" rules={[{ required: true, message: "请选择支持平台" }]}
          tooltip={isEdit ? "已有版本的平台无法移除" : undefined}>
          <Select mode="multiple" placeholder="请选择支持平台" options={PLATFORM_OPTIONS} />
        </Form.Item>

        <Form.Item label="应用图标（Logo）">
          <Form.Item name="logo" hidden>
            <Input />
          </Form.Item>
          <Upload
            accept="image/*"
            maxCount={1}
            showUploadList={false}
            beforeUpload={beforeUpload}
            customRequest={handleUpload}
          >
            <div style={{ cursor: "pointer" }}>
              {logo ? (
                <img src={logoUrl(logo)} alt="应用图标" style={{ maxHeight: 80, maxWidth: 160, objectFit: "contain" }} />
              ) : (
                <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
                  {uploading ? <LoadingOutlined /> : <PlusOutlined />}
                  <span style={{ marginTop: 8, fontSize: 12 }}>上传图标</span>
                </div>
              )}
            </div>
          </Upload>
        </Form.Item>

        <Form.Item name="description" label="应用描述">
          <Input.TextArea rows={3} placeholder="请输入应用描述" maxLength={500} showCount />
        </Form.Item>
      </Form>
    </Modal>
  );
}
