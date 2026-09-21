/**
 * 模板创建 / 编辑弹窗。
 */
import { useEffect } from "react";
import { Form, Input, Modal } from "antd";
import * as templatesApi from "../../api/templates";
import { useSubmit } from "../../hooks/useSubmit";
import type { Template, TemplateInput } from "../../types/api";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  appId: number;
  /** 传入表示编辑，null 表示新建。 */
  template: Template | null;
  onCancel: () => void;
  onSaved: () => void;
}

interface FormValues {
  name: string;
  description?: string;
  content: string;
}

export default function TemplateFormModal({ open, appId, template, onCancel, onSaved }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [submitting, submit] = useSubmit();
  const isEdit = template !== null;

  useEffect(() => {
    if (!open) return;
    form.resetFields();
    if (template) {
      form.setFieldsValue({ name: template.name, description: template.description, content: template.content });
    }
  }, [open, template, form]);

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      const payload: TemplateInput = {
        name: values.name.trim(),
        description: values.description ?? "",
        content: values.content,
      };
      if (template) {
        await templatesApi.updateTemplate(appId, template.id, payload);
        showMessage.success("模板更新成功");
      } else {
        await templatesApi.createTemplate(appId, payload);
        showMessage.success("模板创建成功");
      }
      onSaved();
    });

  return (
    <Modal
      title={isEdit ? `编辑模板 ${template?.name ?? ""}` : "创建模板"}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      destroyOnHidden
      width={720}
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item name="name" label="模板名称" rules={[{ required: true, message: "请输入模板名称" }]}>
          <Input placeholder="例如 latest.json（也作为开放接口 format 参数的取值）" maxLength={100} />
        </Form.Item>
        <Form.Item name="description" label="模板说明">
          <Input placeholder="可选，用于说明模板用途" maxLength={255} />
        </Form.Item>
        <Form.Item
          name="content"
          label="模板内容"
          extra="支持 {{.app.*}}、{{.ver.*}} 与 {{.ext.*}} 变量；创建/更新时服务端会做语法校验"
          rules={[{ required: true, message: "请输入模板内容" }]}
        >
          <Input.TextArea rows={12} placeholder={'示例：{"version": "{{.ver.version}}"}'} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
