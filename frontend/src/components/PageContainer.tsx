/**
 * 页面骨架：标题区（标题/描述/操作） + 主体。
 *
 * 为什么需要它：以前每个页面各自写 margin/padding/背景，间距和层级都不一致。
 * 统一到这里后，新页面只需要给标题和内容。
 */
import type { ReactNode } from "react";

export interface PageContainerProps {
  title?: ReactNode;
  description?: ReactNode;
  /** 右上角主操作区。 */
  extra?: ReactNode;
  /** 主体是否套白色容器；页面内已自带 Card（如应用详情）时传 false。 */
  surface?: boolean;
  children: ReactNode;
}

export default function PageContainer({
  title,
  description,
  extra,
  surface = true,
  children,
}: PageContainerProps) {
  return (
    <div className="page">
      {(title || extra) && (
        <div className="page__header">
          <div className="page__heading">
            {title && <h1 className="page__title">{title}</h1>}
            {description && <p className="page__desc">{description}</p>}
          </div>
          {extra && <div className="page__extra">{extra}</div>}
        </div>
      )}
      {surface ? <div className="surface">{children}</div> : children}
    </div>
  );
}
