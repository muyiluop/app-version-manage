/**
 * 模板预览弹窗：调用 preview 接口并高亮渲染返回内容。
 *
 * 用 React 节点拼装高亮结果而不是 innerHTML，避免模板内容里的 HTML 被当作脚本执行。
 */
import { useEffect, useState, type ReactNode } from "react";
import { Alert, Button, Empty, Modal, Select, Space, Spin } from "antd";
import * as templatesApi from "../../api/templates";
import { useRequest } from "../../hooks/useRequest";
import type { Application, Channel, Platform, Template } from "../../types/api";
import { platformLabel } from "../../utils/format";

interface Props {
  open: boolean;
  app: Application;
  channels: Channel[];
  /** 需要预览的模板。 */
  template: Template | null;
  onClose: () => void;
}

/** 依次匹配：模板变量、JSON 字符串、布尔/空值、数字。 */
const TOKEN_PATTERN = /(\{\{[^{}]*\}\})|("[^"]*")|(\btrue\b|\bfalse\b|\bnull\b)|(-?\d+(?:\.\d+)?)/g;

const VARIABLE_STYLE = { background: "#fffbe6", color: "#d46b08", padding: "0 2px", borderRadius: 2 };
const STRING_STYLE = { color: "#389e0d" };
const LITERAL_STYLE = { color: "#d46b08" };
const NUMBER_STYLE = { color: "#c41d7f" };

function highlight(content: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  const pattern = new RegExp(TOKEN_PATTERN.source, "g");
  let lastIndex = 0;
  let key = 0;
  let match = pattern.exec(content);
  while (match !== null) {
    if (match.index > lastIndex) {
      nodes.push(content.slice(lastIndex, match.index));
    }
    const style = match[1] ? VARIABLE_STYLE : match[2] ? STRING_STYLE : match[3] ? LITERAL_STYLE : NUMBER_STYLE;
    nodes.push(
      <span key={key} style={style}>
        {match[0]}
      </span>
    );
    key += 1;
    lastIndex = match.index + match[0].length;
    match = pattern.exec(content);
  }
  if (lastIndex < content.length) {
    nodes.push(content.slice(lastIndex));
  }
  return nodes;
}

/** JSON 响应美化后再高亮，读起来更直观。 */
function prettify(content: string, contentType: string): string {
  if (!contentType.includes("json")) return content;
  try {
    return JSON.stringify(JSON.parse(content), null, 2);
  } catch {
    return content;
  }
}

export default function TemplatePreviewModal({ open, app, channels, template, onClose }: Props) {
  const [platform, setPlatform] = useState<Platform | undefined>(app.platforms[0]);
  const [channel, setChannel] = useState<string | undefined>(app.defaultChannel || undefined);

  useEffect(() => {
    if (!open) return;
    setPlatform(app.platforms[0]);
    setChannel(app.defaultChannel || undefined);
  }, [open, app]);

  const { data, loading, error } = useRequest(
    ["template-preview", app.id, template?.id ?? 0, platform ?? "", channel ?? ""],
    () => templatesApi.previewTemplate(app.id, template?.id ?? 0, { platform, channel }),
    { enabled: open && template !== null }
  );

  const text = data ? prettify(data.content, data.contentType) : "";

  return (
    <Modal title={`模板预览 · ${template?.name ?? ""}`} open={open} onCancel={onClose} footer={null} width={840}>
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          style={{ width: 180 }}
          placeholder="选择平台"
          value={platform}
          onChange={(value: Platform) => setPlatform(value)}
          options={app.platforms.map((item) => ({ label: platformLabel(item), value: item }))}
        />
        <Select
          allowClear
          style={{ width: 180 }}
          placeholder="选择通道（默认通道）"
          value={channel}
          onChange={(value: string | undefined) => setChannel(value)}
          options={channels.map((item) => ({ label: item.name || item.key, value: item.key }))}
        />
        {data && <span style={{ color: "#8c8c8c" }}>Content-Type：{data.contentType}</span>}
        {data && <span style={{ color: "#8c8c8c" }}>渲染版本：{data.version || "-"}</span>}
      </Space>

      {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 12 }} />}

      <Spin spinning={loading}>
        <div
          style={{
            background: "#f6f8fa",
            border: "1px solid #f0f0f0",
            borderRadius: 6,
            padding: 16,
            maxHeight: 460,
            overflow: "auto",
          }}
        >
          {text ? (
            <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-word", fontFamily: "monospace" }}>
              {highlight(text)}
            </pre>
          ) : (
            <Empty description={loading ? "加载中..." : "暂无预览内容"} />
          )}
        </div>
      </Spin>

      <div style={{ marginTop: 12, textAlign: "right" }}>
        <Button onClick={onClose}>关闭</Button>
      </div>
    </Modal>
  );
}
