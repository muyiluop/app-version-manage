import { describe, expect, it } from "vitest";
import { horizontalScroll } from "./table";

describe("horizontalScroll", () => {
  // 回归：antd 设了 scroll.x 后，空表也会渲染一条横向滚动条（列表看起来像坏了）
  it("无数据时不返回滚动配置", () => {
    expect(horizontalScroll(1000, [])).toBeUndefined();
    expect(horizontalScroll(1000, undefined)).toBeUndefined();
    expect(horizontalScroll(1000, null)).toBeUndefined();
  });

  it("有数据时返回给定的最小宽度", () => {
    expect(horizontalScroll(1000, [{ id: 1 }])).toEqual({ x: 1000 });
    expect(horizontalScroll(1200, ["a", "b"])).toEqual({ x: 1200 });
  });
});
