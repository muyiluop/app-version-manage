/**
 * 表格工具条：左侧筛选/搜索，右侧主操作，错误提示固定在最左。
 *
 * 统一顺序后，各列表页的"搜索在左、按钮在右"不再各写各的 Space。
 */
import type { ReactNode } from "react";

export interface TableToolbarProps {
  left?: ReactNode;
  right?: ReactNode;
  /** 列表加载失败的提示文案；列表 hook 返回 null 时也直接透传。 */
  error?: string | null;
}

export default function TableToolbar({ left, right, error }: TableToolbarProps) {
  return (
    <div className="table-toolbar">
      <div className="table-toolbar__left">
        {left}
        {error && <span className="table-toolbar__error">{error}</span>}
      </div>
      <div className="table-toolbar__right">{right}</div>
    </div>
  );
}
