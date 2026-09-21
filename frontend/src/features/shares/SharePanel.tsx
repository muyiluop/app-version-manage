/**
 * 分享管理：列表、创建（密码/有效期）、编辑、禁用、删除、复制链接。
 */
import { useState } from "react";
import { Button, Input, Modal, Popconfirm, Space, Table, Tag, Tooltip, Typography } from "antd";
import { CopyOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, StopOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import ShareFormModal from "./ShareFormModal";
import * as sharesApi from "../../api/shares";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import { useAuth } from "../auth/AuthContext";
import type { Share } from "../../types/api";
import { canWrite } from "../../utils/auth";
import { copyText, formatDateTime } from "../../utils/format";
import { showMessage } from "../../utils/message";

interface Props {
  appId: number;
}

/** 生成分享前台地址。 */
function shareLink(token: string): string {
  return `${window.location.origin}/share/${token}`;
}

export default function SharePanel({ appId }: Props) {
  const { user } = useAuth();
  const writable = canWrite(user?.role);
  const { data, loading, error, refresh } = useRequest(["shares", appId], () => sharesApi.listShares(appId));

  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Share | null>(null);
  const [created, setCreated] = useState<{ token: string; hasPassword: boolean } | null>(null);
  const [, submitTask] = useSubmit();

  const handleCopy = async (token: string) => {
    const ok = await copyText(shareLink(token));
    if (ok) showMessage.success("分享链接已复制到剪贴板");
  };

  const handleDeactivate = (record: Share) =>
    submitTask(async () => {
      await sharesApi.deactivateShare(appId, record.id);
      showMessage.success("分享已禁用");
      refresh();
    });

  const handleDelete = (record: Share) =>
    submitTask(async () => {
      await sharesApi.deleteShare(appId, record.id);
      showMessage.success("分享已删除");
      refresh();
    });

  const columns: ColumnsType<Share> = [
    {
      title: "分享令牌",
      key: "token",
      render: (_, record) => (
        <Space size={4}>
          <Typography.Text style={{ fontFamily: "monospace", fontSize: 12 }} ellipsis={{ tooltip: record.token }}>
            {record.token.length > 18 ? `${record.token.slice(0, 18)}...` : record.token}
          </Typography.Text>
          <Tooltip title="复制分享链接">
            <Button type="text" size="small" icon={<LinkOutlined />} onClick={() => handleCopy(record.token)} />
          </Tooltip>
        </Space>
      ),
    },
    {
      title: "密码保护",
      key: "hasPassword",
      width: 110,
      render: (_, record) => (record.hasPassword ? <Tag color="red">已设置</Tag> : <Tag>无</Tag>),
    },
    {
      title: "有效期",
      key: "expiresAt",
      width: 160,
      render: (_, record) =>
        record.expiresAt ? formatDateTime(record.expiresAt) : <Tag color="green">永久有效</Tag>,
    },
    {
      title: "状态",
      key: "isActive",
      width: 100,
      render: (_, record) => (record.isActive ? <Tag color="green">已启用</Tag> : <Tag color="red">已禁用</Tag>),
    },
    { title: "访问次数", dataIndex: "accessCount", key: "accessCount", width: 100 },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      key: "createdAt",
      width: 180,
      render: (value: string) => formatDateTime(value),
    },
    {
      title: "操作",
      key: "action",
      width: 220,
      render: (_, record) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            disabled={!writable}
            onClick={() => {
              setEditing(record);
              setModalOpen(true);
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="禁用分享"
            description="禁用后该链接将无法访问，是否继续？"
            okText="确认"
            cancelText="取消"
            onConfirm={() => handleDeactivate(record)}
          >
            <Button type="link" size="small" danger icon={<StopOutlined />} disabled={!writable || !record.isActive}>
              禁用
            </Button>
          </Popconfirm>
          <Popconfirm
            title="删除分享"
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
            setModalOpen(true);
          }}
        >
          创建分享链接
        </Button>
        {error && <span style={{ color: "#ff4d4f" }}>{error}</span>}
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data ?? []}
        loading={loading}
        pagination={false}
        locale={{ emptyText: "暂无分享链接" }}
      />

      <ShareFormModal
        open={modalOpen}
        appId={appId}
        share={editing}
        onCancel={() => {
          setModalOpen(false);
          setEditing(null);
        }}
        onSaved={(saved, isCreated) => {
          setModalOpen(false);
          setEditing(null);
          if (isCreated) setCreated({ token: saved.token, hasPassword: saved.hasPassword });
          refresh();
        }}
      />

      <Modal
        title="分享链接已生成"
        open={created !== null}
        onCancel={() => setCreated(null)}
        footer={[
          <Button key="close" onClick={() => setCreated(null)}>
            关闭
          </Button>,
          <Button
            key="copy"
            type="primary"
            icon={<CopyOutlined />}
            onClick={() => created && handleCopy(created.token)}
          >
            复制链接
          </Button>,
        ]}
      >
        <p>请复制下方链接分享给需要的人：</p>
        <Input.TextArea readOnly autoSize value={created ? shareLink(created.token) : ""} />
        {created?.hasPassword && <p style={{ marginTop: 8, color: "#8c8c8c" }}>该分享已设置访问密码，请一并告知。</p>}
      </Modal>
    </div>
  );
}
