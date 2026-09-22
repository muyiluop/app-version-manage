/**
 * 登录页。支持 ?redirect= 回跳，登录后按 mustChangePassword 引导改密。
 */
import { useState } from "react";
import { Alert, Button, Form, Input } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getErrorMessage } from "../api/client";
import { useAuth } from "../features/auth/AuthContext";
import { safeRedirect } from "../utils/redirect";

interface FormValues {
  username: string;
  password: string;
}

export default function Login() {
  const [form] = Form.useForm<FormValues>();
  const { login } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const handleFinish = async (values: FormValues) => {
    setSubmitting(true);
    setError("");
    try {
      const result = await login(values.username.trim(), values.password);
      if (result.user.mustChangePassword) {
        navigate("/change-password", { replace: true });
        return;
      }
      navigate(safeRedirect(searchParams.get("redirect")), { replace: true });
    } catch (err) {
      setError(getErrorMessage(err) || "登录失败，请检查用户名和密码");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-brand">
          <span className="auth-brand__mark">V</span>
          <h1 className="auth-brand__title">版本发布系统</h1>
          <p className="auth-brand__sub">应用版本管理与分发平台</p>
        </div>

        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

        <Form form={form} layout="vertical" onFinish={handleFinish} requiredMark={false} size="large">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input prefix={<UserOutlined className="text-muted" />} placeholder="用户名" autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password
              prefix={<LockOutlined className="text-muted" />}
              placeholder="密码"
              autoComplete="current-password"
              onPressEnter={() => form.submit()}
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Button type="primary" htmlType="submit" block loading={submitting}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
