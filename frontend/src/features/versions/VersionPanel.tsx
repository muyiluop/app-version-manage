/**
 * 版本管理面板：按平台/通道/状态/关键字筛选 + 服务端分页 + 发布/编辑/上架下架/删除。
 */
import { useState } from "react";
import { Button, Input, Popconfirm, Select, Space, Table, Tag } from "antd";
import { DeleteOutlined, EditOutlined, EyeOutlined, PlusOutlined, StopOutlined, UploadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { TablePaginationConfig } from "antd";
import PublishVersionModal from "./PublishVersionModal";
import VersionDetailDrawer from "./VersionDetailDrawer";
import * as versionsApi from "../../api/versions";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import { useAuth } from "../auth/AuthContext";
import type { Application, Channel, Platform, Version, VersionStatus } from "../../types/api";
import { VERSION_STATUS_OPTIONS, formatBytes, formatDateTime, platformLabel, versionStatusMeta } from "../../utils/format";
import { horizontalScroll } from "../../utils/table";
import { canWrite } from "../../utils/auth";
import { showMessage } from "../../utils/message";

interface Props {
  app: Application;
  channels: Channel[];
}

export default function VersionPanel({ app, channels }: Props) {
  const { user } = useAuth();
  const writable = canWrite(user?.role);

  const [platform, setPlatform] = useState<Platform | undefined>();
  const [channel, setChannel] = useState<string | undefined>();
  const [status, setStatus] = useState<VersionStatus | undefined>();
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [publishOpen, setPublishOpen] = useState(false);
  const [editing, setEditing] = useState<Version | null>(null);
  const [detail, setDetail] = useState<Version | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [, submitTask] = useSubmit();

  const { data, loading, error, refresh } = useRequest(
    ["versions", app.id, platform ?? "", channel ?? "", status ?? "", keyword, page, pageSize],
    () =>
      versionsApi.listVersions({
        appId: app.id,
        platform,
        channel,
        status,
        keyword: keyword || undefined,
        page,
        pageSize,
      })
  );

  const rows = data?.list ?? [];

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  const handleStatusChange = async (record: Version, next: VersionStatus) => {
    setBusyId(record.id);
    try {
      await versionsApi.setVersionStatus(record.id, next);
      showMessage.success(next === "published" ? "版本已上架" : "版本已下架");
      refresh();
    } catch {
      // 拦截器已提示
    } finally {
      setBusyId(null);
    }
  };

  const handleDelete = (record: Version) =>
    submitTask(async () => {
      await versionsApi.deleteVersion(record.id);
      showMessage.success("版本已删除");
      refresh();
    });

  const columns: ColumnsType<Version> = [
    {
      title: "版本号",
      dataIndex: "version",
      key: "version",
      // 版本号是识别主键，固定不换行，避免列被挤压时折成两行
      render: (value: string) => <span className="mono" style={{ fontWeight: 500, whiteSpace: "nowrap" }}>{value}</span>,
    },
    {
      title: "平台",
      dataIndex: "platform",
      key: "platform",
      render: (value: Platform) => platformLabel(value),
    },
    { title: "通道", dataIndex: "channel", key: "channel", render: (value: string) => <Tag color="blue">{value}</Tag> },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      render: (value: VersionStatus) => {
        const meta = versionStatusMeta(value);
        return <Tag color={meta.color}>{meta.label}</Tag>;
      },
    },
    {
      title: "强制更新",
      dataIndex: "forceUpdate",
      key: "forceUpdate",
      render: (value: boolean) => (value ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>),
    },
    {
      title: "文件大小",
      dataIndex: "fileSize",
      key: "fileSize",
      render: (value: number) => formatBytes(value),
    },
    { title: "下载次数", dataIndex: "downloadCount", key: "downloadCount", width: 100 },
    {
      title: "发布时间",
      dataIndex: "publishedAt",
      key: "publishedAt",
      width: 180,
      render: (value: string | null, record) => (
        <span style={{ whiteSpace: "nowrap" }}>{formatDateTime(value ?? record.createdAt)}</span>
      ),
    },
    {
      title: "操作",
      key: "action",
      // 四个带图标的操作按钮需要约 280px，窄了会被裁掉
      width: 300,
      fixed: "right",
      render: (_, record) => (
        <Space size={0}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetail(record)}>
            详情
          </Button>
          {writable && (
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setEditing(record);
                setPublishOpen(true);
              }}
            >
              编辑
            </Button>
          )}
          {writable &&
            (record.status === "published" ? (
              <Button
                type="link"
                size="small"
                icon={<StopOutlined />}
                loading={busyId === record.id}
                onClick={() => handleStatusChange(record, "archived")}
              >
                下架
              </Button>
            ) : (
              <Button
                type="link"
                size="small"
                icon={<UploadOutlined />}
                loading={busyId === record.id}
                onClick={() => handleStatusChange(record, "published")}
              >
                上架
              </Button>
            ))}
          {writable && (
            <Popconfirm
              title="删除版本"
              description="删除后无法恢复，是否继续？"
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
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        {writable && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null);
              setPublishOpen(true);
            }}
          >
            发布版本
          </Button>
        )}
        <Select
          allowClear
          style={{ width: 150 }}
          placeholder="平台筛选"
          value={platform}
          onChange={(value: Platform | undefined) => {
            setPlatform(value);
            setPage(1);
          }}
          options={app.platforms.map((item) => ({ label: platformLabel(item), value: item }))}
        />
        <Select
          allowClear
          style={{ width: 150 }}
          placeholder="通道筛选"
          value={channel}
          onChange={(value: string | undefined) => {
            setChannel(value);
            setPage(1);
          }}
          options={channels.map((item) => ({ label: item.name || item.key, value: item.key }))}
        />
        <Select
          allowClear
          style={{ width: 150 }}
          placeholder="状态筛选"
          value={status}
          onChange={(value: VersionStatus | undefined) => {
            setStatus(value);
            setPage(1);
          }}
          options={VERSION_STATUS_OPTIONS}
        />
        <Input.Search
          allowClear
          style={{ width: 220 }}
          placeholder="搜索版本号或文件名"
          onSearch={(value) => {
            setKeyword(value.trim());
            setPage(1);
          }}
        />
        {error && <span style={{ color: "#ff4d4f" }}>{error}</span>}
      </Space>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={rows}
        loading={loading}
        scroll={horizontalScroll(1200, rows)}
        locale={{ emptyText: "暂无版本数据" }}
        pagination={{
          current: data?.page ?? page,
          pageSize: data?.pageSize ?? pageSize,
          total: data?.total ?? 0,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
        }}
        onChange={handleTableChange}
      />

      <PublishVersionModal
        open={publishOpen}
        app={app}
        channels={channels}
        version={editing}
        onCancel={() => {
          setPublishOpen(false);
          setEditing(null);
        }}
        onSaved={() => {
          setPublishOpen(false);
          setEditing(null);
          refresh();
        }}
      />
      <VersionDetailDrawer open={detail !== null} version={detail} onClose={() => setDetail(null)} />
    </div>
  );
}
