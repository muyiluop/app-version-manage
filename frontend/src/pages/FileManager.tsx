import React, { useState, useEffect } from "react";
import { Table, Button, Space, Upload } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import type { UploadProps } from "antd";
import type { File } from "../types";
import axios from "axios";
import { downloadFile } from "../utils/file";
import { showMessage } from "../utils/message";

const FileManager: React.FC = () => {
  const [files, setFiles] = useState<File[]>([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });

  const loadFiles = async (page: number = 1, pageSize: number = 10) => {
    try {
      setLoading(true);
      const response = await axios.get("/files", {
        params: { page, pageSize },
      });
      setFiles(response.data.list);
      setPagination({
        current: response.data.page,
        pageSize: response.data.pageSize,
        total: response.data.total,
      });
    } catch (error) {
      showMessage.error("获取文件列表失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFiles();
  }, []);

  const handleTableChange = (newPagination: any) => {
    loadFiles(newPagination.current, newPagination.pageSize);
  };

  const handleCleanUnusedFiles = async () => {
    try {
      await axios.post("/files/clean");
      showMessage.success("清理未使用文件成功");
      loadFiles(pagination.current, pagination.pageSize);
    } catch (error) {
      showMessage.error("清理未使用文件失败");
    }
  };
  const uploadProps: UploadProps = {
    name: "file",
    action: `${import.meta.env.VITE_API_BASE_URL}/files/upload`,
    headers: {
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    onChange(info) {
      if (info.file.status === "done") {
        showMessage.success(`${info.file.name} 上传成功`);
        loadFiles();
      } else if (info.file.status === "error") {
        showMessage.error(`${info.file.name} 上传失败`);
      }
    },
  };

  const columns = [
    {
      title: "文件名",
      dataIndex: "name",
      key: "name",
    },
    {
      title: "文件大小",
      dataIndex: "size",
      key: "size",
      render: (size: number) => {
        const units = ["B", "KB", "MB", "GB"];
        let result = size;
        let unitIndex = 0;
        while (result >= 1024 && unitIndex < units.length - 1) {
          result /= 1024;
          unitIndex++;
        }
        return `${result.toFixed(2)} ${units[unitIndex]}`;
      },
    },
    {
      title: "文件类型",
      dataIndex: "type",
      key: "type",
    },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      key: "createdAt",
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: "文件哈希",
      dataIndex: "hash",
      key: "hash",
      ellipsis: true,
    },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: File) => (
        <Space>
          <Button
            type="link"
            onClick={() => {
              downloadFile(record.path, record.name).catch(() => {
                showMessage.error("下载文件失败");
              });
            }}
          >
            下载
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: "flex", justifyContent: "space-between" }}>
        <Space>
          <Upload {...uploadProps}>
            <Button type="primary" icon={<UploadOutlined />}>
              上传文件
            </Button>
          </Upload>
        </Space>
        <Button onClick={handleCleanUnusedFiles}>清理未使用文件</Button>
      </div>
      <Table
        columns={columns}
        dataSource={files}
        rowKey="id"
        loading={loading}
        pagination={pagination}
        onChange={handleTableChange}
      />
    </div>
  );
};

export default FileManager;
