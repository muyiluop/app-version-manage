/**
 * 修改密码页。
 *
 * 后端在改密成功后会吊销该用户的全部令牌，因此这里必须清理本地会话并回到登录页；
 * 但要注意先同步 mustChangePassword，否则路由守卫会把 /change-password 写进
 * 登录页的 ?redirect=，导致登录后又被送回本页。
 */
import { useState } from "react";
import { Alert, App as AntApp, Button, Form, Input } from "antd";
import { LockOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { changePassword } from "../api/auth";
import { getErrorMessage } from "../api/client";
import { useAuth } from "../features/auth/AuthContext";

interface FormValues {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export default function ChangePassword() {
  const [form] = Form.useForm<FormValues>();
  const { user, logout, updateUser } = useAuth();
  const navigate = useNavigate();
  const { message } = AntApp.useApp();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const handleFinish = async (values: FormValues) => {
    setSubmitting(true);
    setError("");
    try {
      const result = await changePassword(values.oldPassword, values.newPassword);
      // 先用后端返回的权威状态同步本地用户，守卫就不会再判定需要改密
      updateUser(result.user);
      message.success(result.message || "密码已修改，请重新登录");
      logout();
      navigate("/login", { replace: true });
    } catch (err) {
      setError(getErrorMessage(err) || "修改密码失败");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-brand">
          <span className="auth-brand__mark">V</span>
          <h1 className="auth-brand__title">修改密码</h1>
          <p className="auth-brand__sub">修改后需要使用新密码重新登录</p>
        </div>

        {user?.mustChangePassword && (
          <Alert
            type="warning"
            showIcon
            message="首次登录或密码已被重置，请先修改密码后再继续使用"
            style={{ marginBottom: 16 }}
          />
        )}
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

        <Form form={form} layout="vertical" onFinish={handleFinish} requiredMark={false} size="large">
          <Form.Item name="oldPassword" label="原密码" rules={[{ required: true, message: "请输入原密码" }]}>
            <Input.Password prefix={<LockOutlined className="text-muted" />} placeholder="请输入原密码" />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: "请输入新密码" },
              { min: 6, message: "密码长度不能少于 6 位" },
            ]}
          >
            <Input.Password prefix={<LockOutlined className="text-muted" />} placeholder="至少 6 位" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={["newPassword"]}
            rules={[
              { required: true, message: "请再次输入新密码" },
              ({ getFieldValue }) => ({
                validator: (_, value: string) => {
                  if (!value || getFieldValue("newPassword") === value) return Promise.resolve();
                  return Promise.reject(new Error("两次输入的密码不一致"));
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined className="text-muted" />} placeholder="请再次输入新密码" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Button type="primary" htmlType="submit" block loading={submitting}>
              确认修改
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
