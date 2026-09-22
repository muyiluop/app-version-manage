/**
 * 表格横向滚动配置。
 *
 * 为什么需要它：antd 只要设置了 `scroll.x`，即便当前一行数据都没有，也会渲染出
 * 一条横向滚动条（空表占位行与表头的宽度计算不一致），看起来像是列表坏了。
 * 这里在无数据时不返回 scroll 配置，从根上避免这条空滚动条。
 *
 * 用法：`scroll={horizontalScroll(1000, rows)}`
 */
export function horizontalScroll<T>(x: number, rows?: readonly T[] | null) {
  return rows && rows.length > 0 ? { x } : undefined;
}
