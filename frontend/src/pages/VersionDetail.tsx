import React, { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { Card, Descriptions, Tag, Button, message } from "antd";
import type { Version } from "../types";
import axios from "axios";
import { downloadFile } from "../utils/file";

const VersionDetail: React.FC = () => {
  const { versionId } = useParams<{ versionId: string }>();
  const [version, setVersion] = useState<Version | null>(null);
  const [loading, setLoading] = useState(false);

  const loadVersionData = async () => {
    try {
      setLoading(true);
      const response = await axios.get(`/versions/${versionId}`);
      setVersion(response.data);
    } catch (error) {
      console.error("Failed to load version:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (versionId) {
      loadVersionData();
    }
  }, [versionId]);

  if (!version) {
    return null;
  }

  return (
    <Card loading={loading}>
      <Descriptions title="版本信息" bordered>
        <Descriptions.Item label="版本号">{version.version}</Descriptions.Item>
        <Descriptions.Item label="平台">{version.platform}</Descriptions.Item>
        <Descriptions.Item label="强制更新">
          {version.forceUpdate ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="状态">
          {version.isActive ? <Tag color="green">已发布</Tag> : <Tag color="red">已下架</Tag>}
        </Descriptions.Item>
        <Descriptions.Item label="文件信息" span={3}>
          {version.fileName} ({(version.fileSize / 1024 / 1024).toFixed(2)} MB)
          <Button
            type="link"
            onClick={() => {
              downloadFile(version.filePath, version.fileName).catch(() => {
                message.error("下载文件失败");
              });
            }}
          >
            下载
          </Button>
        </Descriptions.Item>
        {version.changelog && (
          <Descriptions.Item label="更新日志" span={3}>
            <pre style={{ whiteSpace: "pre-wrap" }}>{version.changelog}</pre>
          </Descriptions.Item>
        )}
        <Descriptions.Item label="发布时间">{new Date(version.createdAt).toLocaleString()}</Descriptions.Item>
        <Descriptions.Item label="最后更新">{new Date(version.updatedAt).toLocaleString()}</Descriptions.Item>
      </Descriptions>
    </Card>
  );
};

export default VersionDetail;
