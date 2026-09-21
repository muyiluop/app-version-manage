/**
 * 通道管理：列表 + 增删改 + 设为默认。
 *
 * 默认通道与仍被版本引用的通道由服务端拒绝删除（409），前端只做二次确认。
 */
import { useState } from "react";
import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Table, Tag, Tooltip } from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined, StarOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import * as channelsApi from "../../api/channels";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import type { Channel, ChannelInput } from "../../types/api";
import { showMessage } from "../../utils/message";
import { canWrite } from "../../utils/auth";
import { useAuth } from "../auth/AuthContext";

interface Props {
  appId: number;
}

interface FormValues {
  key: string;
  name: string;
  sort: number;
}

export default function ChannelPanel({ appId }: Props) {
  const { user } = useAuth();
  const writable = canWrite(user?.role);
  const { data, loading, error, refresh } = useRequest(["channels", appId], () => channelsApi.listChannels(appId));

  const [form] = Form.useForm<FormValues>();
  const [editing, setEditing] = useState<Channel | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [submitting, submit] = useSubmit();
  const [rowBusy, setRowBusy] = useState<number | null>(null);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ sort: (data?.length ?? 0) + 1 });
    setModalOpen(true);
  };

  const openEdit = (record: Channel) => {
    setEditing(record);
    form.setFieldsValue({ key: record.key, name: record.name, sort: record.sort });
    setModalOpen(true);
  };

  const handleFinish = (values: FormValues) =>
    submit(async () => {
      const payload: ChannelInput = { key: values.key.trim(), name: values.name.trim(), sort: values.sort };
      if (editing) {
        await channelsApi.updateChannel(appId, editing.id, payload);
        showMessage.success("通道更新成功");
      } else {
        await channelsApi.createChannel(appId, payload);
        showMessage.success("通道创建成功");
      }
      setModalOpen(false);
      refresh();
    });

  const handleSetDefault = async (record: Channel) => {
    setRowBusy(record.id);
    try {
      await channelsApi.setDefaultChannel(appId, record.key);
      showMessage.success(`已将 ${record.name || record.key} 设为默认通道`);
      refresh();
    } catch {
      // 拦截器已提示
    } finally {
      setRowBusy(null);
    }
  };

  const handleDelete = async (record: Channel) => {
    setRowBusy(record.id);
    try {
      await channelsApi.deleteChannel(appId, record.id);
      showMessage.success("通道已删除");
      refresh();
    } catch {
      // 拦截器已提示
    } finally {
      setRowBusy(null);
    }
  };

  const columns: ColumnsType<Channel> = [
    {
      title: "通道 Key",
      dataIndex: "key",
      key: "key",
      render: (key: string, record) => (
        <Space>
          <span style={{ fontFamily: "monospace" }}>{key}</span>
          {record.isDefault && <Tag color="gold">默认</Tag>}
        </Space>
      ),
    },
    { title: "通道名称", dataIndex: "name", key: "name", render: (name: string) => name || "-" },
    { title: "排序", dataIndex: "sort", key: "sort", width: 90 },
    {
      title: "操作",
      key: "action",
      width: 240,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} disabled={!writable} onClick={() => openEdit(record)}>
            编辑
          </Button>
          {!record.isDefault && (
            <Tooltip title="设为默认通道">
              <Button
                type="link"
                size="small"
                icon={<StarOutlined />}
                disabled={!writable}
                loading={rowBusy === record.id}
                onClick={() => handleSetDefault(record)}
              >
                设为默认
              </Button>
            </Tooltip>
          )}
          <Popconfirm
            title="删除通道"
            description="默认通道或已有版本的通道无法删除，是否继续？"
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
        <Button type="primary" icon={<PlusOutlined />} disabled={!writable} onClick={openCreate}>
          新增通道
        </Button>
        {error && <span style={{ color: "#ff4d4f" }}>{error}</span>}
      </div>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={data ?? []}
        loading={loading}
        pagination={false}
        locale={{ emptyText: "暂无通道" }}
      />

      <Modal
        title={editing ? "编辑通道" : "新增通道"}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={submitting}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={handleFinish}>
          <Form.Item
            name="key"
            label="通道 Key"
            rules={[
              { required: true, message: "请输入通道 Key" },
              { pattern: /^[a-z0-9_-]+$/, message: "仅支持小写字母、数字、下划线与中划线" },
            ]}
          >
            <Input placeholder="例如 stable" maxLength={32} />
          </Form.Item>
          <Form.Item name="name" label="通道名称" rules={[{ required: true, message: "请输入通道名称" }]}>
            <Input placeholder="例如 稳定版" maxLength={64} />
          </Form.Item>
          <Form.Item name="sort" label="排序" rules={[{ required: true, message: "请输入排序值" }]}>
            <InputNumber min={0} max={9999} style={{ width: "100%" }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
