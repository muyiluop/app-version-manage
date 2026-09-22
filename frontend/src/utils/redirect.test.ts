import { describe, expect, it } from "vitest";
import { safeRedirect } from "./redirect";

describe("safeRedirect", () => {
  it("保留站内普通路径（含查询串）", () => {
    expect(safeRedirect("/apps")).toBe("/apps");
    expect(safeRedirect("/apps/12?tab=versions")).toBe("/apps/12?tab=versions");
  });

  it("空值回落到应用列表", () => {
    expect(safeRedirect(null)).toBe("/apps");
    expect(safeRedirect(undefined)).toBe("/apps");
    expect(safeRedirect("")).toBe("/apps");
  });

  it("拒绝站外与协议相对地址，避免开放重定向", () => {
    expect(safeRedirect("https://evil.example.com")).toBe("/apps");
    expect(safeRedirect("//evil.example.com")).toBe("/apps");
    expect(safeRedirect("javascript:alert(1)")).toBe("/apps");
  });

  // 回归：改密成功后不应被送回改密页（曾导致"改完密码仍提示改密"）
  it("拒绝把登录/改密流程页作为回跳目标", () => {
    expect(safeRedirect("/change-password")).toBe("/apps");
    expect(safeRedirect("/change-password?from=login")).toBe("/apps");
    expect(safeRedirect("/login")).toBe("/apps");
    expect(safeRedirect("/login?redirect=/apps")).toBe("/apps");
  });

  it("不影响与流程页同前缀的其它路径", () => {
    expect(safeRedirect("/logins")).toBe("/logins");
    expect(safeRedirect("/change-password-history")).toBe("/change-password-history");
  });
});
