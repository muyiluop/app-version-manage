/**
 * 用户创建 / 编辑弹窗（仅 admin 可进入）。
 */
import { useEffect } from "react";
import { Form, Input, Modal, Select, Switch } from "antd";
import * as usersApi from "../../api/users";
import { useSubmit } from "../../hooks/useSubmit";
import type { CreateUserInput, UpdateUserInput, UserProfile, UserRole } from "../../types/api";
import { roleLabel } from "../../utils/auth";
import { showMessage } from "../../utils/message";

interface Props {
  open: boolean;
  /** 传入表示编辑，null 表示新建。 */
  user: UserProfile | null;
  onCancel: () => void;
  onSaved: () => void;
}

interface FormValues {
  username: string;
  displayName?: string;
  role: UserRole;
  password?: string;
  isActive?: boolean;
}

const ROLE_OPTIONS: Array<{ label: string; value: UserRole }> = (["admin", "releaser", "viewer"] as UserRole[]).map(
  (role) => ({ label: roleLabel(role), value: role })
);

export default function UserFormModal({ open, user, onCancel, onSaved }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [submitting, submit] = useSubmit();
  const isEdit = user !== null;

  useEffect(() => {
    if (!open) return;
    form.resetFields();
    if (user) {
      form.setFieldsValue({
        username: user.username,
        displayName: user.displayName,
        role: user.role,
        isActive: true,
      });
    } else {
      form.setFieldsValue({ role: "viewer", isActive: true });
    }
  }, [open, user, form]);

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      if (user) {
        const payload: UpdateUserInput = {
          displayName: values.displayName ?? "",
          role: values.role,
          isActive: values.isActive ?? true,
        };
        // 留空表示不重置密码
        if (values.password) payload.password = values.password;
        await usersApi.updateUser(user.id, payload);
        showMessage.success("用户更新成功");
      } else {
        const payload: CreateUserInput = {
          username: values.username.trim(),
          password: values.password ?? "",
          displayName: values.displayName ?? "",
          role: values.role,
        };
        await usersApi.createUser(payload);
        showMessage.success("用户创建成功");
      }
      onSaved();
    });

  return (
    <Modal
      title={isEdit ? `编辑用户 ${user?.username ?? ""}` : "创建用户"}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" onFinish={handleFinish}>
        <Form.Item name="username" label="用户名" rules={[{ required: true, message: "请输入用户名" }]}>
          <Input placeholder="请输入用户名" disabled={isEdit} maxLength={64} />
        </Form.Item>
        <Form.Item name="displayName" label="显示名称">
          <Input placeholder="请输入显示名称" maxLength={64} />
        </Form.Item>
        <Form.Item name="role" label="角色" rules={[{ required: true, message: "请选择角色" }]}>
          <Select options={ROLE_OPTIONS} />
        </Form.Item>
        {isEdit && (
          <Form.Item name="isActive" label="账号状态" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        )}
        <Form.Item
          name="password"
          label={isEdit ? "重置密码" : "初始密码"}
          extra={isEdit ? "留空表示不修改密码" : "长度不少于 6 位"}
          rules={[
            { required: !isEdit, message: "请输入密码" },
            { min: 6, message: "密码长度不能少于 6 位" },
          ]}
        >
          <Input.Password placeholder={isEdit ? "留空表示不修改" : "请输入初始密码"} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
