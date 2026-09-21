/**
 * 分享访问密码输入页。
 *
 * 密码错误的提示由父组件传入，这里只负责输入与提交，保持展示与逻辑分离。
 */
import { useState } from "react";
import { Alert, Button, Card, Form, Input, Typography } from "antd";
import { LockOutlined } from "@ant-design/icons";

interface Props {
  appName?: string;
  error: string;
  loading: boolean;
  onSubmit: (password: string) => void;
}

export default function PasswordGate({ appName, error, loading, onSubmit }: Props) {
  const [password, setPassword] = useState("");

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
        <Typography.Title level={4} style={{ textAlign: "center", marginTop: 0 }}>
          <LockOutlined /> 请输入访问密码
        </Typography.Title>
        {appName && (
          <Typography.Paragraph type="secondary" style={{ textAlign: "center" }}>
            {appName}
          </Typography.Paragraph>
        )}
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        <Form
          layout="vertical"
          onFinish={() => {
            if (password) onSubmit(password);
          }}
        >
          <Form.Item label="访问密码" required>
            <Input.Password
              size="large"
              autoFocus
              value={password}
              placeholder="请输入分享密码"
              onChange={(event) => setPassword(event.target.value)}
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" size="large" block htmlType="submit" loading={loading} disabled={!password}>
              确定
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
