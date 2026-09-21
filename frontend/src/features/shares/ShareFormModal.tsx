/**
 * 分享链接创建 / 编辑弹窗。
 *
 * 编辑时的语义来自契约：password 传空字符串表示"清除密码"，字段缺省表示"保持不变"；
 * 因此用显式的"操作类型"下拉来表达意图，避免把"留空"误判成清除。
 */
import { useEffect } from "react";
import { Form, Input, InputNumber, Modal, Select } from "antd";
import * as sharesApi from "../../api/shares";
import { useSubmit } from "../../hooks/useSubmit";
import type { Share, UpdateShareInput } from "../../types/api";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  appId: number;
  /** 传入表示编辑，null 表示新建。 */
  share: Share | null;
  onCancel: () => void;
  onSaved: (share: Share, created: boolean) => void;
}

type PasswordAction = "keep" | "clear" | "set";
type ExpiresAction = "keep" | "permanent" | "days";

interface FormValues {
  password?: string;
  expiresInDays?: number;
  passwordAction?: PasswordAction;
  expiresAction?: ExpiresAction;
}

const PASSWORD_ACTIONS: Array<{ label: string; value: PasswordAction }> = [
  { label: "保持不变", value: "keep" },
  { label: "清除密码", value: "clear" },
  { label: "设置新密码", value: "set" },
];

const EXPIRES_ACTIONS: Array<{ label: string; value: ExpiresAction }> = [
  { label: "保持不变", value: "keep" },
  { label: "永久有效", value: "permanent" },
  { label: "指定天数", value: "days" },
];

export default function ShareFormModal({ open, appId, share, onCancel, onSaved }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [submitting, submit] = useSubmit();
  const isEdit = share !== null;
  const passwordAction = Form.useWatch("passwordAction", form);
  const expiresAction = Form.useWatch("expiresAction", form);

  useEffect(() => {
    if (!open) return;
    form.resetFields();
    if (share) {
      form.setFieldsValue({ passwordAction: "keep", expiresAction: "keep" });
    } else {
      form.setFieldsValue({ expiresInDays: 0 });
    }
  }, [open, share, form]);

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      if (share) {
        const payload: UpdateShareInput = {};
        if (values.passwordAction === "clear") payload.password = "";
        if (values.passwordAction === "set") payload.password = values.password ?? "";
        if (values.expiresAction === "permanent") payload.expiresInDays = 0;
        if (values.expiresAction === "days") payload.expiresInDays = values.expiresInDays ?? 0;

        if (Object.keys(payload).length === 0) {
          showMessage.warning("没有需要修改的内容");
          return;
        }
        const saved = await sharesApi.updateShare(appId, share.id, payload);
        showMessage.success("分享更新成功");
        onSaved(saved, false);
        return;
      }

      const saved = await sharesApi.createShare(appId, {
        password: values.password ?? "",
        expiresInDays: values.expiresInDays ?? 0,
      });
      showMessage.success("分享链接已生成");
      onSaved(saved, true);
    });

  return (
    <Modal
      title={isEdit ? "编辑分享" : "创建分享链接"}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        {isEdit ? (
          <>
            <Form.Item name="passwordAction" label="访问密码" rules={[{ required: true, message: "请选择密码操作" }]}>
              <Select options={PASSWORD_ACTIONS} />
            </Form.Item>
            {passwordAction === "set" && (
              <Form.Item
                name="password"
                label="新密码"
                rules={[
                  { required: true, message: "请输入新密码" },
                  { min: 4, message: "密码长度不少于 4 位" },
                ]}
              >
                <Input.Password placeholder="请输入新的访问密码" />
              </Form.Item>
            )}
            <Form.Item name="expiresAction" label="有效期" rules={[{ required: true, message: "请选择有效期操作" }]}>
              <Select options={EXPIRES_ACTIONS} />
            </Form.Item>
            {expiresAction === "days" && (
              <Form.Item
                name="expiresInDays"
                label="有效期（天）"
                rules={[{ required: true, message: "请输入有效期天数" }]}
              >
                <InputNumber min={1} max={3650} style={{ width: "100%" }} placeholder="请输入天数" />
              </Form.Item>
            )}
          </>
        ) : (
          <>
            <Form.Item name="password" label="访问密码" rules={[{ min: 4, message: "密码长度不少于 4 位" }]}>
              <Input.Password placeholder="不填写则不设置密码" />
            </Form.Item>
            <Form.Item
              name="expiresInDays"
              label="有效期（天）"
              extra="0 表示永久有效"
              rules={[{ type: "number", min: 0, message: "有效期不能为负数" }]}
            >
              <InputNumber min={0} max={3650} style={{ width: "100%" }} placeholder="天数，0 表示永久有效" />
            </Form.Item>
          </>
        )}
      </Form>
    </Modal>
  );
}
