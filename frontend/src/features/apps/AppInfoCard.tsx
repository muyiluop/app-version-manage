/**
 * 应用信息卡片：展示基本信息，并提供编辑与删除入口。
 */
import { useState } from "react";
import { Button, Card, Descriptions, Popconfirm, Space, Tag, Typography } from "antd";
import { DeleteOutlined, EditOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import AppFormModal from "./AppFormModal";
import * as appsApi from "../../api/apps";
import { useSubmit } from "../../hooks/useSubmit";
import { useAuth } from "../auth/AuthContext";
import type { Application } from "../../types/api";
import { canWrite, isAdmin } from "../../utils/auth";
import { formatDateTime, logoUrl, platformLabel } from "../../utils/format";
import { showMessage } from "../../utils/message";

interface Props {
  app: Application;
  onChanged: () => void;
}

export default function AppInfoCard({ app, onChanged }: Props) {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [editOpen, setEditOpen] = useState(false);
  const [deleting, removeApp] = useSubmit();

  const handleDelete = () =>
    removeApp(async () => {
      await appsApi.deleteApp(app.id);
      showMessage.success("应用已删除");
      navigate("/apps", { replace: true });
    });

  return (
    <Card>
      <div style={{ display: "flex", gap: 24, alignItems: "flex-start" }}>
        <div style={{ width: 120, flexShrink: 0 }}>
          {app.logo ? (
            <img
              src={logoUrl(app.logo)}
              alt="应用图标"
              style={{ width: 120, height: 120, objectFit: "contain", borderRadius: 8, background: "#fafafa" }}
            />
          ) : (
            <div
              style={{
                width: 120,
                height: 120,
                borderRadius: 8,
                background: "#fafafa",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: "#bfbfbf",
              }}
            >
              无图标
            </div>
          )}
        </div>
        <div style={{ flex: 1 }}>
          <Space style={{ width: "100%", justifyContent: "space-between", alignItems: "flex-start" }}>
            <Typography.Title level={3} style={{ margin: 0 }}>
              {app.name}
            </Typography.Title>
            <Space>
              {canWrite(user?.role) && (
                <Button type="primary" icon={<EditOutlined />} onClick={() => setEditOpen(true)}>
                  编辑应用
                </Button>
              )}
              {isAdmin(user?.role) && (
                <Popconfirm
                  title="删除应用"
                  description="将级联删除该应用的版本、通道、分享与模板，且无法恢复，是否继续？"
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true }}
                  onConfirm={handleDelete}
                >
                  <Button danger icon={<DeleteOutlined />} loading={deleting}>
                    删除应用
                  </Button>
                </Popconfirm>
              )}
            </Space>
          </Space>
          <Descriptions column={{ xs: 1, sm: 2 }} size="small" style={{ marginTop: 16 }}>
            <Descriptions.Item label="应用标识">{app.identifier}</Descriptions.Item>
            <Descriptions.Item label="默认通道">
              <Tag color="blue">{app.defaultChannel}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="支持平台" span={2}>
              <Space wrap>
                {app.platforms.map((platform) => (
                  <Tag key={platform}>{platformLabel(platform)}</Tag>
                ))}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">{formatDateTime(app.createdAt)}</Descriptions.Item>
            <Descriptions.Item label="更新时间">{formatDateTime(app.updatedAt)}</Descriptions.Item>
            <Descriptions.Item label="应用描述" span={2}>
              {app.description || "-"}
            </Descriptions.Item>
          </Descriptions>
        </div>
      </div>

      <AppFormModal
        open={editOpen}
        app={app}
        onCancel={() => setEditOpen(false)}
        onSaved={() => {
          setEditOpen(false);
          onChanged();
        }}
      />
    </Card>
  );
}
