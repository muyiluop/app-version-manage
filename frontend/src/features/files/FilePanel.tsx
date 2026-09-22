/**
 * 文件库管理：服务端分页 + 搜索 + 上传/下载/删除 + 清理未使用。
 */
import { useState } from "react";
import { Button, Input, Popconfirm, Progress, Space, Table, Tag, Tooltip, Upload, type UploadProps } from "antd";
import { ClearOutlined, DeleteOutlined, DownloadOutlined, UploadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { TablePaginationConfig } from "antd";
import * as filesApi from "../../api/files";
import { useRequest } from "../../hooks/useRequest";
import { useSubmit } from "../../hooks/useSubmit";
import PageContainer from "../../components/PageContainer";
import TableToolbar from "../../components/TableToolbar";
import { useAuth } from "../auth/AuthContext";
import type { StoredFile } from "../../types/api";
import { canWrite } from "../../utils/auth";
import { saveBlob } from "../../utils/download";
import { formatBytes, formatDateTime } from "../../utils/format";
import { horizontalScroll } from "../../utils/table";
import { showMessage } from "../../utils/message";

export default function FilePanel() {
  const { user } = useAuth();
  const writable = canWrite(user?.role);

  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [uploadPercent, setUploadPercent] = useState<number | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const [, submitTask] = useSubmit();

  const { data, loading, error, refresh } = useRequest(
    ["files", keyword, page, pageSize],
    () => filesApi.listFiles({ keyword: keyword || undefined, page, pageSize })
  );

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  const handleUpload: UploadProps["customRequest"] = async (options) => {
    const file = options.file as unknown as File;
    setUploadPercent(0);
    try {
      const result = await filesApi.uploadFile(file, setUploadPercent);
      options.onSuccess?.(result);
      showMessage.success(result.deduplicated ? `${file.name} 已存在，秒传成功` : `${file.name} 上传成功`);
      refresh();
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error("上传失败"));
    } finally {
      setUploadPercent(null);
    }
  };

  const handleDownload = async (record: StoredFile) => {
    setBusyId(record.id);
    try {
      const blob = await filesApi.downloadFile(record.id);
      saveBlob(blob, record.name);
    } catch {
      // 拦截器已提示
    } finally {
      setBusyId(null);
    }
  };

  const handleDelete = (record: StoredFile) =>
    submitTask(async () => {
      await filesApi.deleteFile(record.id);
      showMessage.success("文件已删除");
      refresh();
    });

  const handleClean = () =>
    submitTask(async () => {
      const result = await filesApi.cleanFiles();
      showMessage.success(`清理完成，共移除 ${result.removed} 个未使用文件`);
      refresh();
    });

  const columns: ColumnsType<StoredFile> = [
    {
      title: "文件名",
      dataIndex: "name",
      key: "name",
      render: (value: string, record) => (
        <Tooltip title={record.key}>
          <span>{value}</span>
        </Tooltip>
      ),
    },
    { title: "大小", dataIndex: "size", key: "size", width: 110, render: (value: number) => formatBytes(value) },
    { title: "类型", dataIndex: "contentType", key: "contentType", render: (value: string) => value || "-" },
    {
      title: "引用数",
      dataIndex: "refCount",
      key: "refCount",
      width: 100,
      render: (value: number) => (value > 0 ? <Tag color="blue">{value}</Tag> : <Tag>0</Tag>),
    },
    {
      title: "存储",
      dataIndex: "storage",
      key: "storage",
      width: 100,
      render: (value: string) => value || "-",
    },
    {
      title: "SHA256",
      dataIndex: "sha256",
      key: "sha256",
      width: 160,
      render: (value: string) =>
        value ? (
          <Tooltip title={value}>
            <span style={{ fontFamily: "monospace", fontSize: 12 }}>{value.slice(0, 12)}...</span>
          </Tooltip>
        ) : (
          "-"
        ),
    },
    {
      title: "上传时间",
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
          <Button
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            loading={busyId === record.id}
            onClick={() => handleDownload(record)}
          >
            下载
          </Button>
          <Popconfirm
            title="删除文件"
            description="仍被版本引用的文件无法删除，是否继续？"
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
    <PageContainer title="文件管理" description="版本产物与图标等对象文件，支持秒传去重与孤儿清理">
      <TableToolbar
        left={
          <Input.Search
            allowClear
            style={{ width: 260 }}
            placeholder="搜索文件名或对象键"
            onSearch={(value) => {
              setKeyword(value.trim());
              setPage(1);
            }}
          />
        }
        right={
          <>
            {writable && (
              <Upload multiple={false} showUploadList={false} customRequest={handleUpload}>
                <Button type="primary" icon={<UploadOutlined />} loading={uploadPercent !== null}>
                  上传文件
                </Button>
              </Upload>
            )}
            {writable && (
              <Popconfirm
                title="清理未使用文件"
                description="将删除所有未被版本或应用引用的文件，是否继续？"
                okText="清理"
                cancelText="取消"
                onConfirm={handleClean}
              >
                <Button icon={<ClearOutlined />}>清理未使用文件</Button>
              </Popconfirm>
            )}
          </>
        }
        error={error}
      />

      {uploadPercent !== null && (
        <Progress percent={uploadPercent} style={{ marginBottom: 16 }} status="active" />
      )}

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.list ?? []}
        loading={loading}
        scroll={horizontalScroll(1100, data?.list)}
        locale={{ emptyText: "暂无文件" }}
        pagination={{
          current: data?.page ?? page,
          pageSize: data?.pageSize ?? pageSize,
          total: data?.total ?? 0,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 个文件`,
        }}
        onChange={handleTableChange}
      />
    </PageContainer>
  );
}
