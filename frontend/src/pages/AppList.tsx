import React, { useState, useEffect } from "react";
import { Table, Button, Modal, Form, Input, Select, Upload, Space } from "antd";
import { PlusOutlined, UploadOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import type { UploadProps } from "antd";
import axios from "axios";
import type { ColumnsType } from "antd/es/table";
import { getFileUrl } from "../utils/file";
import { showMessage } from "../utils/message";

interface Application {
  id: number;
  name: string;
  identifier: string;
  logo: string;
  description: string;
  platforms: string[];
  createdAt: string;
}

const platformOptions = [
  { label: "Android", value: "android" },
  { label: "iOS", value: "ios" },
  { label: "Windows", value: "windows" },
  { label: "Linux", value: "linux" },
  { label: "MacOS", value: "macos" },
  { label: "Harmony", value: "harmony" },
];

const AppList: React.FC = () => {
  const [apps, setApps] = useState<Application[]>([]);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const [previewImage, setPreviewImage] = useState<string>();

  const fetchApps = async () => {
    try {
      const response = await axios.get("/apps");
      setApps(response.data);
    } catch (error) {
      showMessage.error("获取应用列表失败");
    }
  };

  useEffect(() => {
    fetchApps();
  }, []);

  const uploadProps: UploadProps = {
    name: "file",
    action: import.meta.env.VITE_API_BASE_URL + "/files/upload",
    headers: {
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    // showUploadList: false,
    accept: "image/*",
    listType: "picture-card",
    maxCount: 1,
    onChange(info) {
      if (info.file.status === "done") {
        form.setFieldsValue({
          logo: info.file.response.path,
        });
        setPreviewImage(getFileUrl(info.file.response.path));
        // message.success("Logo上传成功");
      } else {
        setPreviewImage(undefined);
      }
    },
    beforeUpload: (file) => {
      const isImage = file.type.startsWith("image/");
      if (!isImage) {
        showMessage.error("只能上传图片文件!");
      }
      return isImage;
    },
  };

  const columns: ColumnsType<Application> = [
    {
      title: "Logo",
      dataIndex: "logo",
      key: "logo",
      render: (logo) => (logo ? <img src={getFileUrl(logo)} alt="logo" style={{ width: 40, height: 40 }} /> : null),
    },
    {
      title: "应用名称",
      dataIndex: "name",
      key: "name",
    },
    {
      title: "标识",
      dataIndex: "identifier",
      key: "identifier",
    },
    {
      title: "支持平台",
      dataIndex: "platforms",
      key: "platforms",
      render: (platforms: string[]) => platforms.join(", "),
    },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      key: "createdAt",
      render: (text) => new Date(text).toLocaleString(),
    },
    {
      title: "操作",
      key: "action",
      render: (_, record) => (
        <Button type="link" onClick={() => navigate(`/apps/${record.id}`)}>
          查看详情
        </Button>
      ),
    },
  ];

  const handleCreate = async (values: any) => {
    try {
      await axios.post("/apps", values);
      showMessage.success("创建应用成功");
      setIsModalVisible(false);
      form.resetFields();
      fetchApps();
    } catch (error) {
      showMessage.error("创建应用失败");
    }
  };

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
          创建应用
        </Button>
      </div>
      <Table columns={columns} dataSource={apps} rowKey="id" />

      <Modal
        title="创建应用"
        open={isModalVisible}
        onCancel={() => setIsModalVisible(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} onFinish={handleCreate} layout="vertical">
          <Form.Item name="name" label="应用名称" rules={[{ required: true, message: "请输入应用名称" }]}>
            <Input placeholder="请输入应用名称" />
          </Form.Item>

          <Form.Item name="identifier" label="应用标识" rules={[{ required: true, message: "请输入应用标识" }]}>
            <Input placeholder="请输入应用标识" />
          </Form.Item>

          <Form.Item name="description" label="应用描述">
            <Input.TextArea placeholder="请输入应用描述" />
          </Form.Item>

          <Form.Item label="Logo" required>
            <Space direction="vertical" style={{ width: "100%" }}>
              <Form.Item name="logo" noStyle>
                <Input hidden />
              </Form.Item>
              <Upload {...uploadProps}>
                {/* {previewImage !== undefined ? (
                  <Image
                    src={previewImage}
                    preview={{
                      visible: previewOpen,
                      onVisibleChange: (visible) => {
                        setPreviewOpen(visible);
                      },
                    }}
                  />
                ) : (
                )} */}

                {!previewImage && <UploadOutlined />}
              </Upload>
            </Space>
          </Form.Item>

          <Form.Item name="platforms" label="支持平台" rules={[{ required: true, message: "请选择支持的平台" }]}>
            <Select mode="multiple" placeholder="请选择支持的平台" options={platformOptions} />
          </Form.Item>
        </Form>
      </Modal>

      {/* {previewImage && (
        <Image
          style={{ display: "none" }}
          src={previewImage}
          preview={{
            visible: previewOpen,
            onVisibleChange: (visible) => {
              setPreviewOpen(visible);
            },
          }}
        />
      )} */}
    </div>
  );
};

export default AppList;
