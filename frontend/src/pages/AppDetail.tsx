import React, { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { Card, Tabs, Table, Button, Modal, Form, Input, Select, Upload, Space, Descriptions, Tag } from "antd";
import { PlusOutlined, UploadOutlined, EditOutlined, ShareAltOutlined } from "@ant-design/icons";
import { showMessage } from "../utils/message";
import type { TabsProps } from "antd";
import type { UploadProps } from "antd";
import type { Application, Version, Template } from "../types";
import axios from "axios";
import { getFileUrl } from "../utils/file";

const { Option } = Select;
const { TextArea } = Input;

// 平台类型定义
const SUPPORTED_PLATFORMS = ["android", "ios", "windows", "linux", "macos", "harmony"] as const;

const AppDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [app, setApp] = useState<Application | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(false);
  const [isVersionModalVisible, setIsVersionModalVisible] = useState(false);
  const [isTemplateModalVisible, setIsTemplateModalVisible] = useState(false);
  const [isTemplateEditVisible, setIsTemplateEditVisible] = useState(false);
  const [isTemplatePreviewVisible, setIsTemplatePreviewVisible] = useState(false);
  const [previewContent, setPreviewContent] = useState<string>("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewPlatform, setPreviewPlatform] = useState<string>("");
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null);
  const [versionForm] = Form.useForm();
  const [templateForm] = Form.useForm();
  const [templateEditForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [selectedPlatform, setSelectedPlatform] = useState<string>("");
  const [selectedVersion, setSelectedVersion] = useState<Version | null>(null);
  const [isVersionDetailVisible, setIsVersionDetailVisible] = useState(false);
  const [isEditModalVisible, setIsEditModalVisible] = useState(false);

  const loadVersions = async (page: number = 1, pageSize: number = 10) => {
    try {
      const response = await axios.get(`/versions`, {
        params: {
          appId: id,
          page,
          pageSize,
          platform: selectedPlatform || undefined,
        },
      });
      setVersions(response.data.list);
      setPagination({
        current: response.data.page,
        pageSize: response.data.pageSize,
        total: response.data.total,
      });
    } catch (error) {
      showMessage.error("加载版本列表失败");
    }
  };

  const loadAppData = async () => {
    try {
      setLoading(true);
      const [appResponse, templatesResponse] = await Promise.all([
        axios.get(`/apps/${id}`),
        axios.get(`/templates?appId=${id}`),
      ]);
      setApp(appResponse.data);
      setTemplates(templatesResponse.data);
      await loadVersions();
    } catch (error) {
      showMessage.error("加载应用信息失败");
    } finally {
      setLoading(false);
    }
  };

  // 监听平台筛选变化
  useEffect(() => {
    if (id) {
      loadVersions(1, pagination.pageSize);
    }
  }, [selectedPlatform, id]);

  useEffect(() => {
    if (id) {
      loadAppData();
    }
  }, [id]);
  const uploadProps: UploadProps = {
    name: "file",
    action: `${import.meta.env.VITE_API_BASE_URL}/files/upload`,
    headers: {
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    onChange(info) {
      if (info.file.status === "done") {
        const { path, name, size } = info.file.response;
        if (isEditModalVisible) {
          editForm.setFieldsValue({
            logo: path,
          });
        } else {
          versionForm.setFieldsValue({
            filePath: path,
            fileName: name,
            fileSize: size,
          });
        }
      }
    },
  };
  const handleCreateVersion = async (values: any) => {
    try {
      await axios.post("/versions", {
        ...values,
        appId: Number(id),
      });
      showMessage.success("创建版本成功");
      setIsVersionModalVisible(false);
      versionForm.resetFields();
      loadAppData();
    } catch (error) {
      showMessage.error("创建版本失败");
    }
  };

  const handleCreateTemplate = async (values: any) => {
    try {
      await axios.post("/templates", {
        ...values,
        appId: Number(id),
      });
      showMessage.success("创建模板成功");
      setIsTemplateModalVisible(false);
      templateForm.resetFields();
      loadAppData();
    } catch (error) {
      showMessage.error("创建模板失败");
    }
  };

  const handleDeactivateVersion = async (versionId: number) => {
    try {
      await axios.put(`/versions/${versionId}/deactivate`);
      showMessage.success("下架版本成功");
      loadAppData();
    } catch (error) {
      showMessage.error("下架版本失败");
    }
  };

  const handleDeleteVersion = async (versionId: number) => {
    try {
      await axios.delete(`/versions/${versionId}`);
      showMessage.success("删除版本成功");
      loadAppData();
    } catch (error) {
      showMessage.error("删除版本失败");
    }
  };

  const handleTableChange = (newPagination: any) => {
    loadVersions(newPagination.current, newPagination.pageSize);
  };

  const handleViewVersion = (version: Version) => {
    setSelectedVersion(version);
    setIsVersionDetailVisible(true);
  };

  const handleEditTemplate = (template: Template) => {
    setSelectedTemplate(template);
    templateEditForm.setFieldsValue({
      content: template.content,
    });
    setIsTemplateEditVisible(true);
  };
  const handlePreviewTemplate = async (template: Template) => {
    setSelectedTemplate(template);
    setPreviewPlatform("");
    setIsTemplatePreviewVisible(true);
  };
  const loadPreviewContent = async () => {
    if (!selectedTemplate || !app || !previewPlatform) return;

    try {
      setPreviewLoading(true);
      const response = await axios.get(`/open/latest`, {
        params: {
          identifier: app.identifier,
          format: selectedTemplate.name,
          platform: previewPlatform || undefined,
        },
      });
      // 如果是字符串直接预览，如果是json对象则先转为字符串
      if (typeof response.data === "string") {
        setPreviewContent(response.data);
      } else {
        setPreviewContent(JSON.stringify(response.data, null, 2));
      }
    } catch (error: any) {
      setPreviewContent("");
    } finally {
      setPreviewLoading(false);
    }
  };
  // 监听平台选择变化自动刷新预览
  useEffect(() => {
    if (isTemplatePreviewVisible) {
      loadPreviewContent();
    }
  }, [previewPlatform, selectedTemplate?.id]);

  const handleUpdateTemplate = async (values: any) => {
    if (!selectedTemplate) return;

    try {
      await axios.put(`/templates/${selectedTemplate.id}`, {
        ...selectedTemplate,
        content: values.content,
      });
      showMessage.success("更新模板成功");
      setIsTemplateEditVisible(false);
      templateEditForm.resetFields();
      loadAppData();
    } catch (error) {
      showMessage.error("更新模板失败");
    }
  };

  const handleUpdateApp = async (values: any) => {
    try {
      await axios.put(`/apps/${id}`, values);
      showMessage.success("更新应用成功");
      setIsEditModalVisible(false);
      editForm.resetFields();
      loadAppData();
    } catch (error: any) {
      showMessage.error(error.response?.data?.error || "更新应用失败");
    }
  };

  const versionColumns = [
    {
      title: "版本号",
      dataIndex: "version",
      key: "version",
    },
    {
      title: "平台",
      dataIndex: "platform",
      key: "platform",
    },
    {
      title: "强制更新",
      dataIndex: "forceUpdate",
      key: "forceUpdate",
      render: (text: boolean) => (text ? "是" : "否"),
    },
    {
      title: "状态",
      dataIndex: "isActive",
      key: "isActive",
      render: (text: boolean) => (text ? "已发布" : "已下架"),
    },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      key: "createdAt",
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: Version) => (
        <Space>
          <Button type="link" onClick={() => handleViewVersion(record)}>
            查看
          </Button>
          {record.isActive && (
            <Button type="link" onClick={() => handleDeactivateVersion(record.id)}>
              下架
            </Button>
          )}
          <Button type="link" danger onClick={() => handleDeleteVersion(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const templateColumns = [
    {
      title: "模板名称",
      dataIndex: "name",
      key: "name",
    },
    {
      title: "内容",
      dataIndex: "content",
      key: "content",
      ellipsis: true,
    },
    {
      title: "创建时间",
      dataIndex: "createdAt",
      key: "createdAt",
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: Template) => (
        <Space>
          <Button type="link" onClick={() => handleEditTemplate(record)} icon={<EditOutlined />}>
            编辑
          </Button>
          <Button type="link" onClick={() => handlePreviewTemplate(record)} loading={previewLoading}>
            预览
          </Button>
        </Space>
      ),
    },
  ];

  const items: TabsProps["items"] = [
    {
      key: "1",
      label: "版本管理",
      children: (
        <>
          <div style={{ marginBottom: 16, display: "flex", justifyContent: "space-between" }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsVersionModalVisible(true)}>
              发布版本
            </Button>
            <Select
              style={{ width: 200 }}
              placeholder="选择平台筛选"
              allowClear
              value={selectedPlatform}
              onChange={(value) => setSelectedPlatform(value)}
              options={app?.platforms.map((platform) => ({ label: platform, value: platform }))}
            />
          </div>
          <Table
            columns={versionColumns}
            dataSource={versions}
            rowKey="id"
            pagination={pagination}
            onChange={handleTableChange}
          />
        </>
      ),
    },
    {
      key: "2",
      label: "模板管理",
      children: (
        <>
          <div style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsTemplateModalVisible(true)}>
              创建模板
            </Button>
          </div>
          <Table columns={templateColumns} dataSource={templates} rowKey="id" />
        </>
      ),
    },
  ];

  const isJson = (value: string) => {
    try {
      JSON.parse(value);
      return true;
    } catch (e) {
      return false;
    }
  };
  return (
    <div>
      <Card loading={loading}>
        <div style={{ display: "flex", alignItems: "flex-start", gap: "24px" }}>
          {app?.logo && (
            <div>
              <img
                src={getFileUrl(app.logo)}
                alt={`${app.name} logo`}
                style={{
                  maxHeight: "120px",
                  maxWidth: "200px",
                  objectFit: "contain",
                  borderRadius: "4px",
                }}
              />
            </div>
          )}
          <div style={{ flex: 1 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
              <div>
                <h2>{app?.name}</h2>
                <p>应用标识：{app?.identifier}</p>
                <p>支持平台：{app?.platforms.join(", ")}</p>
                {app?.description && <p>应用描述：{app.description}</p>}
              </div>
              <Space>
                <Button
                  type="primary"
                  icon={<EditOutlined />}
                  onClick={() => {
                    editForm.setFieldsValue({
                      name: app?.name,
                      logo: app?.logo,
                      description: app?.description,
                      platforms: app?.platforms,
                    });
                    setIsEditModalVisible(true);
                  }}
                >
                  编辑应用
                </Button>
                <Button
                  type="primary"
                  icon={<ShareAltOutlined />}
                  onClick={async () => {
                    try {
                      const response = await axios.post(`/apps/${id}/share`);
                      const shareUrl = `${window.location.origin}/share/${response.data.shareToken}`;
                      Modal.success({
                        title: "分享链接已生成",
                        content: (
                          <div>
                            <p>分享链接有效期为30天，请复制下方链接分享：</p>
                            <Input.TextArea
                              value={shareUrl}
                              autoSize
                              readOnly
                              onClick={(e) => e.currentTarget.select()}
                            />
                          </div>
                        ),
                      });
                    } catch (error) {
                      showMessage.error("生成分享链接失败");
                    }
                  }}
                >
                  分享应用
                </Button>
              </Space>
            </div>
          </div>
        </div>
      </Card>
      <div style={{ marginTop: 24 }}>
        <Tabs items={items} />
      </div>

      <Modal
        title="发布版本"
        open={isVersionModalVisible}
        onOk={versionForm.submit}
        onCancel={() => setIsVersionModalVisible(false)}
        destroyOnHidden
      >
        <Form form={versionForm} onFinish={handleCreateVersion} layout="vertical">
          <Form.Item name="version" label="版本号" rules={[{ required: true, message: "请输入版本号" }]}>
            <Input placeholder="请输入版本号" />
          </Form.Item>
          <Form.Item name="platform" label="发布平台" rules={[{ required: true, message: "请选择发布平台" }]}>
            <Select placeholder="请选择发布平台">
              {app?.platforms.map((platform) => (
                <Option key={platform} value={platform}>
                  {platform}
                </Option>
              ))}
            </Select>
          </Form.Item>{" "}
          <Form.Item label="版本文件" required>
            <Form.Item name="filePath" hidden>
              <Input />
            </Form.Item>
            <Form.Item name="fileName" hidden>
              <Input />
            </Form.Item>
            <Form.Item name="fileSize" hidden>
              <Input />
            </Form.Item>
            <Upload {...uploadProps}>
              <Button icon={<UploadOutlined />}>上传文件</Button>
            </Upload>
          </Form.Item>
          <Form.Item name="changelog" label="更新日志">
            <TextArea rows={4} placeholder="请输入更新日志" />
          </Form.Item>
          <Form.Item
            name="ext"
            label="扩展信息"
            rules={[
              {
                validateTrigger: ["onBlur"],
                validator: (_, value) => {
                  console.log("value:", value);
                  if (value == "") {
                    return Promise.resolve();
                  }
                  if (isJson(value)) {
                    return Promise.resolve();
                  } else {
                    return Promise.reject("请输入正确的json字符串");
                  }
                },
              },
            ]}
            extra="可以输入json格式的key-value对用于存储自定义信息，将会在获取版本信息时返回"
          >
            <TextArea rows={4} placeholder={'示例：{"key": "value"}'} />
          </Form.Item>
          <Form.Item name="forceUpdate" label="强制更新" valuePropName="checked">
            <Select>
              <Option value={false}>否</Option>
              <Option value={true}>是</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="创建模板"
        open={isTemplateModalVisible}
        onOk={templateForm.submit}
        onCancel={() => setIsTemplateModalVisible(false)}
        destroyOnHidden
      >
        <Form form={templateForm} onFinish={handleCreateTemplate} layout="vertical">
          <Form.Item name="name" label="模板名称" rules={[{ required: true, message: "请输入模板名称" }]}>
            <Input placeholder="请输入模板名称" />
          </Form.Item>
          <Form.Item name="content" label="模板内容" rules={[{ required: true, message: "请输入模板内容" }]}>
            <TextArea rows={6} placeholder="请输入模板内容，支持使用 {{.app.*}} {{.ver.*}} {{.ext.*}} 变量" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="版本详情"
        open={isVersionDetailVisible}
        onCancel={() => setIsVersionDetailVisible(false)}
        footer={null}
        width={720}
      >
        {selectedVersion && (
          <Descriptions column={2} bordered>
            <Descriptions.Item label="版本号" span={2}>
              {selectedVersion.version}
            </Descriptions.Item>
            <Descriptions.Item label="平台">{selectedVersion.platform}</Descriptions.Item>
            <Descriptions.Item label="状态">
              {selectedVersion.isActive ? <Tag color="green">已发布</Tag> : <Tag color="red">已下架</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="强制更新">
              {selectedVersion.forceUpdate ? <Tag color="red">是</Tag> : <Tag color="green">否</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="文件名称">{selectedVersion.fileName}</Descriptions.Item>
            <Descriptions.Item label="文件大小" span={2}>
              {(selectedVersion.fileSize / 1024 / 1024).toFixed(2)} MB
              <Button type="link" onClick={() => (window.location.href = getFileUrl(selectedVersion.filePath))}>
                下载
              </Button>
            </Descriptions.Item>
            {selectedVersion.changelog && (
              <Descriptions.Item label="更新日志" span={2}>
                <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>{selectedVersion.changelog}</pre>
              </Descriptions.Item>
            )}
            <Descriptions.Item label="创建时间">
              {new Date(selectedVersion.createdAt).toLocaleString()}
            </Descriptions.Item>
            <Descriptions.Item label="更新时间">
              {new Date(selectedVersion.updatedAt).toLocaleString()}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      <Modal
        title={`编辑模板 - ${selectedTemplate?.name}`}
        open={isTemplateEditVisible}
        onOk={templateEditForm.submit}
        onCancel={() => setIsTemplateEditVisible(false)}
        width={800}
      >
        <Form
          form={templateEditForm}
          onFinish={handleUpdateTemplate}
          layout="vertical"
          initialValues={{ content: selectedTemplate?.content }}
        >
          <Form.Item
            name="content"
            label="模板内容"
            rules={[{ required: true, message: "请输入模板内容" }]}
            extra="支持使用 {{.app.*}}、{{.ver.*}}和{{.ext.*}} 变量"
          >
            <TextArea rows={12} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="预览模板输出"
        open={isTemplatePreviewVisible}
        onCancel={() => setIsTemplatePreviewVisible(false)}
        footer={null}
        width={800}
      >
        <div
          style={{
            backgroundColor: "#f5f5f5",
            padding: "16px",
            borderRadius: "4px",
            maxHeight: "600px",
            overflow: "auto",
          }}
        >
          <Select
            style={{ width: 200, marginBottom: 16 }}
            placeholder="选择平台预览"
            allowClear
            value={previewPlatform}
            onChange={(value) => setPreviewPlatform(value)}
            options={app?.platforms.map((platform) => ({ label: platform, value: platform }))}
          />
          <Button type="primary" onClick={loadPreviewContent} loading={previewLoading} style={{ marginBottom: 16 }}>
            刷新预览
          </Button>
          <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordWrap: "break-word" }}>{previewContent}</pre>
        </div>
      </Modal>

      <Modal
        title="编辑应用"
        open={isEditModalVisible}
        onOk={editForm.submit}
        onCancel={() => setIsEditModalVisible(false)}
        destroyOnHidden
      >
        <Form form={editForm} onFinish={handleUpdateApp} layout="vertical">
          <Form.Item name="name" label="应用名称" rules={[{ required: true, message: "请输入应用名称" }]}>
            <Input placeholder="请输入应用名称" />
          </Form.Item>
          <Form.Item label="应用图标" required>
            <Form.Item name="logo" hidden>
              <Input />
            </Form.Item>
            <Upload {...uploadProps}>
              <Button icon={<UploadOutlined />}>上传图标</Button>
            </Upload>
            {app?.logo && <img src={getFileUrl(app.logo)} alt="logo" style={{ marginTop: 8, maxHeight: 100 }} />}
          </Form.Item>
          <Form.Item
            name="platforms"
            label="支持平台"
            rules={[{ required: true, message: "请选择支持平台" }]}
            tooltip="已有版本的平台无法移除"
          >
            <Select mode="multiple" placeholder="请选择支持平台">
              {SUPPORTED_PLATFORMS.map((platform) => (
                <Option key={platform} value={platform}>
                  {platform}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="description" label="应用描述">
            <TextArea rows={4} placeholder="请输入应用描述" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default AppDetail;
