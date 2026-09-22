/**
 * 应用列表：关键字搜索 + 服务端分页 + 创建/删除。
 */
import { useState } from "react";
import { Button, Input, Popconfirm, Space, Table, Tag, Typography } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { TablePaginationConfig } from "antd";
import { useNavigate } from "react-router-dom";
import AppFormModal from "../features/apps/AppFormModal";
import PageContainer from "../components/PageContainer";
import TableToolbar from "../components/TableToolbar";
import * as appsApi from "../api/apps";
import { useRequest } from "../hooks/useRequest";
import { useSubmit } from "../hooks/useSubmit";
import { useAuth } from "../features/auth/AuthContext";
import type { Application, Platform } from "../types/api";
import { canWrite, isAdmin } from "../utils/auth";
import { formatDateTime, logoUrl, platformLabel } from "../utils/format";
import { horizontalScroll } from "../utils/table";
import { showMessage } from "../utils/message";

export default function Apps() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [createOpen, setCreateOpen] = useState(false);
  const [, submitTask] = useSubmit();

  const { data, loading, error, refresh } = useRequest(["apps", keyword, page, pageSize], () =>
    appsApi.listApps({ keyword: keyword || undefined, page, pageSize })
  );

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  const handleDelete = (record: Application) =>
    submitTask(async () => {
      await appsApi.deleteApp(record.id);
      showMessage.success("应用已删除");
      refresh();
    });

  const columns: ColumnsType<Application> = [
    {
      title: "图标",
      dataIndex: "logo",
      key: "logo",
      width: 80,
      render: (value: string) => (value ? <img className="app-logo" src={logoUrl(value)} alt="应用图标" /> : "-"),
    },
    {
      title: "应用名称",
      dataIndex: "name",
      key: "name",
      render: (value: string, record) => (
        <Typography.Link onClick={() => navigate(`/apps/${record.id}`)}>{value}</Typography.Link>
      ),
    },
    {
      title: "应用标识",
      dataIndex: "identifier",
      key: "identifier",
      render: (value: string) => <span className="mono">{value}</span>,
    },
    {
      title: "支持平台",
      dataIndex: "platforms",
      key: "platforms",
      render: (value: Platform[]) => (
        <span className="tag-list">
          {value.map((platform) => (
            <Tag key={platform}>{platformLabel(platform)}</Tag>
          ))}
        </span>
      ),
    },
    {
      title: "默认通道",
      dataIndex: "defaultChannel",
      key: "defaultChannel",
      width: 120,
      render: (value: string) => <Tag color="blue">{value}</Tag>,
    },
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
      width: 180,
      render: (_, record) => (
        <Space size={0}>
          <Button type="link" size="small" onClick={() => navigate(`/apps/${record.id}`)}>
            查看详情
          </Button>
          {isAdmin(user?.role) && (
            <Popconfirm
              title="删除应用"
              description="将级联删除该应用的版本、通道、分享与模板，是否继续？"
              okText="删除"
              cancelText="取消"
              okButtonProps={{ danger: true }}
              onConfirm={() => handleDelete(record)}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title="应用管理"
      description="维护应用、支持平台与默认发布通道"
      extra={
        canWrite(user?.role) && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            创建应用
          </Button>
        )
      }
    >
      <TableToolbar
        left={
          <Input.Search
            allowClear
            style={{ width: 260 }}
            placeholder="搜索应用名称或标识"
            onSearch={(value) => {
              setKeyword(value.trim());
              setPage(1);
            }}
          />
        }
        error={error}
      />

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.list ?? []}
        loading={loading}
        scroll={horizontalScroll(1000, data?.list)}
        locale={{ emptyText: "暂无应用" }}
        pagination={{
          current: data?.page ?? page,
          pageSize: data?.pageSize ?? pageSize,
          total: data?.total ?? 0,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 个应用`,
        }}
        onChange={handleTableChange}
      />

      <AppFormModal
        open={createOpen}
        app={null}
        onCancel={() => setCreateOpen(false)}
        onSaved={() => {
          setCreateOpen(false);
          refresh();
        }}
      />
    </PageContainer>
  );
}
