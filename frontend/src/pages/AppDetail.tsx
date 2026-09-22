/**
 * 应用详情：信息卡片 + 版本/通道/分享/模板四个 feature 面板。
 *
 * 这里只负责装配数据与切换 Tab，具体交互逻辑都在 features/ 下，
 * 避免重新退化成一个近千行的巨型组件。
 */
import { useState } from "react";
import { useParams } from "react-router-dom";
import { Card, Result, Skeleton, Tabs, type TabsProps } from "antd";
import AppInfoCard from "../features/apps/AppInfoCard";
import PageContainer from "../components/PageContainer";
import ChannelPanel from "../features/channels/ChannelPanel";
import VersionPanel from "../features/versions/VersionPanel";
import SharePanel from "../features/shares/SharePanel";
import TemplatePanel from "../features/templates/TemplatePanel";
import { getApp } from "../api/apps";
import { listChannels } from "../api/channels";
import { useRequest } from "../hooks/useRequest";

export default function AppDetail() {
  const { id } = useParams<{ id: string }>();
  const appId = Number(id);
  const enabled = Number.isInteger(appId) && appId > 0;

  const appQuery = useRequest(["app", appId], () => getApp(appId), { enabled });
  const channelQuery = useRequest(["channels", appId], () => listChannels(appId), { enabled });
  const [activeKey, setActiveKey] = useState("versions");

  if (!enabled) {
    return <Result status="404" title="应用不存在" subTitle="请从应用列表重新进入" />;
  }

  if (appQuery.loading && !appQuery.data) {
    return (
      <Card>
        <Skeleton active paragraph={{ rows: 6 }} />
      </Card>
    );
  }

  if (appQuery.error || !appQuery.data) {
    return <Result status="error" title="加载应用失败" subTitle={appQuery.error ?? "应用不存在或已被删除"} />;
  }

  const app = appQuery.data;
  const channels = channelQuery.data ?? [];

  const items: TabsProps["items"] = [
    {
      key: "versions",
      label: "版本管理",
      children: <VersionPanel app={app} channels={channels} />,
    },
    {
      key: "channels",
      label: "通道管理",
      children: <ChannelPanel appId={app.id} />,
    },
    {
      key: "shares",
      label: "分享管理",
      children: <SharePanel appId={app.id} />,
    },
    {
      key: "templates",
      label: "模板管理",
      children: <TemplatePanel app={app} channels={channels} />,
    },
  ];

  return (
    <PageContainer surface={false}>
      <AppInfoCard app={app} onChanged={appQuery.refresh} />
      <div className="surface">
        <Tabs items={items} activeKey={activeKey} onChange={setActiveKey} />
      </div>
    </PageContainer>
  );
}
