/**
 * 模板管理：列表、创建、编辑、预览、删除。
 */
import { useState } from "react";
import { Button, Popconfirm, Space, Table, Tooltip, Typography } from "antd";
import { DeleteOutlined, EditOutlined, EyeOutlined, PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import TemplateFormModal from "./TemplateFormModal";
import TemplatePreviewModal from "./TemplatePreviewModal";
import * as templatesApi from "../../api/templates";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import { useAuth } from "../auth/AuthContext";
import type { Application, Channel, Template } from "../../types/api";
import { canWrite } from "../../utils/auth";
import { formatDateTime } from "../../utils/format";
import { showMessage } from "../../utils/message";

interface Props {
  app: Application;
  channels: Channel[];
}

export default function TemplatePanel({ app, channels }: Props) {
  const { user } = useAuth();
  const writable = canWrite(user?.role);
  const { data, loading, error, refresh } = useRequest(["templates", app.id], () => templatesApi.listTemplates(app.id));

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Template | null>(null);
  const [previewing, setPreviewing] = useState<Template | null>(null);
  const [, submitTask] = useSubmit();

  const handleDelete = (record: Template) =>
    submitTask(async () => {
      await templatesApi.deleteTemplate(app.id, record.id);
      showMessage.success("模板已删除");
      refresh();
    });

  const columns: ColumnsType<Template> = [
    { title: "模板名称", dataIndex: "name", key: "name" },
    {
      title: "说明",
      dataIndex: "description",
      key: "description",
      render: (value: string) => value || "-",
    },
    {
      title: "内容",
      dataIndex: "content",
      key: "content",
      render: (value: string) => (
        <Tooltip title={<pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>{value}</pre>}>
          <Typography.Text style={{ maxWidth: 320 }} ellipsis>
            {value.replace(/\s+/g, " ")}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: "更新时间",
      dataIndex: "updatedAt",
      key: "updatedAt",
      width: 180,
      render: (value: string) => formatDateTime(value),
    },
    {
      title: "操作",
      key: "action",
      width: 220,
      render: (_, record) => (
        <Space size={0}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setPreviewing(record)}>
            预览
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            disabled={!writable}
            onClick={() => {
              setEditing(record);
              setFormOpen(true);
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="删除模板"
            description="删除后无法恢复，是否继续？"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={() => handleDelete(record)}
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />} disabled={!writable}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: "flex", justifyContent: "space-between" }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          disabled={!writable}
          onClick={() => {
            setEditing(null);
            setFormOpen(true);
          }}
        >
          创建模板
        </Button>
        {error && <span style={{ color: "#ff4d4f" }}>{error}</span>}
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data ?? []}
        loading={loading}
        pagination={false}
        locale={{ emptyText: "暂无模板" }}
      />

      <TemplateFormModal
        open={formOpen}
        appId={app.id}
        template={editing}
        onCancel={() => {
          setFormOpen(false);
          setEditing(null);
        }}
        onSaved={() => {
          setFormOpen(false);
          setEditing(null);
          refresh();
        }}
      />

      <TemplatePreviewModal
        open={previewing !== null}
        app={app}
        channels={channels}
        template={previewing}
        onClose={() => setPreviewing(null)}
      />
    </div>
  );
}
