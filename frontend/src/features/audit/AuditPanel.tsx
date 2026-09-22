/**
 * 审计日志：按 action 过滤 + 分页。
 */
import { useState } from "react";
import { Select, Table, Tag, Tooltip } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { TablePaginationConfig } from "antd";
import PageContainer from "../../components/PageContainer";
import TableToolbar from "../../components/TableToolbar";
import * as auditApi from "../../api/audit";
import { useRequest } from "../../hooks/useRequest";
import type { AuditLog } from "../../types/api";
import { formatDateTime } from "../../utils/format";
import { horizontalScroll } from "../../utils/table";

export default function AuditPanel() {
  const [action, setAction] = useState<string | undefined>();
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const { data, loading, error } = useRequest(["audit-logs", action ?? "", page, pageSize], () =>
    auditApi.listAuditLogs({ action, page, pageSize })
  );

  const handleTableChange = (pagination: TablePaginationConfig) => {
    setPage(pagination.current ?? 1);
    setPageSize(pagination.pageSize ?? 20);
  };

  const columns: ColumnsType<AuditLog> = [
    {
      title: "时间",
      dataIndex: "createdAt",
      key: "createdAt",
      width: 180,
      render: (value: string) => formatDateTime(value),
    },
    { title: "操作人", dataIndex: "actorName", key: "actorName", width: 140, render: (value: string) => value || "-" },
    {
      title: "动作",
      dataIndex: "action",
      key: "action",
      width: 180,
      render: (value: string) => <Tag>{value}</Tag>,
    },
    {
      title: "对象",
      key: "target",
      width: 160,
      render: (_, record) => (record.targetType ? `${record.targetType}#${record.targetId || "-"}` : "-"),
    },
    {
      title: "摘要",
      dataIndex: "summary",
      key: "summary",
      render: (value: string) => (
        <Tooltip title={value}>
          <span>{value || "-"}</span>
        </Tooltip>
      ),
    },
    { title: "IP", dataIndex: "ip", key: "ip", width: 140, render: (value: string) => value || "-" },
    {
      title: "结果",
      dataIndex: "success",
      key: "success",
      width: 90,
      render: (value: boolean) => (value ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>),
    },
  ];

  return (
    <PageContainer title="审计日志" description="关键操作留痕：谁在什么时候改了什么">
      <TableToolbar
        left={
          <Select
            allowClear
            showSearch
            style={{ width: 240 }}
            placeholder="按动作过滤"
            value={action}
            onChange={(value: string | undefined) => {
              setAction(value);
              setPage(1);
            }}
            options={auditApi.AUDIT_ACTIONS}
          />
        }
        error={error}
      />

      <Table
        rowKey="id"
        columns={columns}
        dataSource={data?.list ?? []}
        loading={loading}
        scroll={horizontalScroll(1100, data?.list)}
        locale={{ emptyText: "暂无审计日志" }}
        pagination={{
          current: data?.page ?? page,
          pageSize: data?.pageSize ?? pageSize,
          total: data?.total ?? 0,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
        }}
        onChange={handleTableChange}
      />
    </PageContainer>
  );
}
