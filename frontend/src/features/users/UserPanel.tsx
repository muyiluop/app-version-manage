/**
 * 用户管理：列表、创建、改角色/状态/重置密码、删除（仅 admin 可见）。
 */
import { useState } from "react";
import { Button, Popconfirm, Space, Table, Tag } from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { TablePaginationConfig } from "antd";
import UserFormModal from "./UserFormModal";
import PageContainer from "../../components/PageContainer";
import TableToolbar from "../../components/TableToolbar";
import * as usersApi from "../../api/users";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import { useAuth } from "../auth/AuthContext";
import type { UserProfile, UserRole } from "../../types/api";
import { roleLabel } from "../../utils/auth";
import { formatDateTime } from "../../utils/format";
import { showMessage } from "../../utils/message";

const ROLE_COLOR: Record<UserRole, string> = { admin: "red", releaser: "blue", viewer: "default" };

export default function UserPanel() {
  const { user: currentUser } = useAuth();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<UserProfile | null>(null);
  const [, submitTask] = useSubmit();

  const { data, loading, error, refresh } = useRequest(["users", page, pageSize], () =>
    usersApi.listUsers({ page, pageSize })
  );

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  const handleDelete = (record: UserProfile) =>
    submitTask(async () => {
      await usersApi.deleteUser(record.id);
      showMessage.success("用户已删除");
      refresh();
    });

  const columns: ColumnsType<UserProfile> = [
    { title: "用户名", dataIndex: "username", key: "username" },
    { title: "显示名称", dataIndex: "displayName", key: "displayName", render: (value: string) => value || "-" },
    {
      title: "角色",
      dataIndex: "role",
      key: "role",
      width: 120,
      render: (value: UserRole) => <Tag color={ROLE_COLOR[value]}>{roleLabel(value)}</Tag>,
    },
    {
      title: "需改密",
      dataIndex: "mustChangePassword",
      key: "mustChangePassword",
      width: 100,
      render: (value: boolean) => (value ? <Tag color="orange">是</Tag> : <Tag color="green">否</Tag>),
    },
    {
      title: "最后登录",
      dataIndex: "lastLoginAt",
      key: "lastLoginAt",
      width: 180,
      render: (value: string | null) => formatDateTime(value),
    },
    {
      title: "操作",
      key: "action",
      width: 170,
      render: (_, record) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => {
              setEditing(record);
              setModalOpen(true);
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="删除用户"
            description="不能删除自己与最后一个管理员，是否继续？"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={() => handleDelete(record)}
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              disabled={record.id === currentUser?.id}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="用户管理" description="账号、角色与状态；重置密码后该用户下次登录需改密">
      <TableToolbar
        right={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null);
              setModalOpen(true);
            }}
          >
            创建用户
          </Button>
        }
        error={error}
      />

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.list ?? []}
        loading={loading}
        locale={{ emptyText: "暂无用户" }}
        pagination={{
          current: data?.page ?? page,
          pageSize: data?.pageSize ?? pageSize,
          total: data?.total ?? 0,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 个用户`,
        }}
        onChange={handleTableChange}
      />

      <UserFormModal
        open={modalOpen}
        user={editing}
        onCancel={() => {
          setModalOpen(false);
          setEditing(null);
        }}
        onSaved={() => {
          setModalOpen(false);
          setEditing(null);
          refresh();
        }}
      />
    </PageContainer>
  );
}
