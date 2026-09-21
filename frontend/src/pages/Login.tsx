/**
 * 登录页。支持 ?redirect= 回跳，登录后按 mustChangePassword 引导改密。
 */
import { useState } from "react";
import { Alert, Button, Card, Form, Input, Typography } from "antd";
import { LockOutlined, UserOutlined } from "@ant-design/icons";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getErrorMessage } from "../api/client";
import { useAuth } from "../features/auth/AuthContext";

interface FormValues {
  username: string;
  password: string;
}

/** 只允许站内相对路径回跳，避免开放重定向。 */
function safeRedirect(raw: string | null): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//")) return "/apps";
  return raw;
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
    <div
      style={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        minHeight: "100vh",
        background: "#f0f2f5",
        padding: 16,
      }}
    >
      <Card style={{ width: "100%", maxWidth: 400 }}>
        <Typography.Title level={3} style={{ textAlign: "center", marginTop: 0 }}>
          软件版本管理系统
        </Typography.Title>
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        <Form form={form} layout="vertical" onFinish={handleFinish} requiredMark={false}>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" size="large" autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="密码"
              size="large"
              autoComplete="current-password"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" size="large" block loading={submitting}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
