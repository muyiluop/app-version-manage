/**
 * 全局视觉主题（antd 设计令牌）。
 *
 * 为什么集中在这里：以前颜色、圆角、间距散落在 100 多处内联 style 里，
 * 改一次风格要翻遍所有页面。统一收敛到令牌后，整体风格只需改这一处，
 * 且组件库所有默认态（hover/disabled/dark 弹层）都会自动跟随。
 */
import { theme, type ThemeConfig } from "antd";

/** 品牌主色与中性色阶。 */
export const brand = {
  primary: "#1677ff",
  siderBg: "#0f172a",
  siderBgHover: "#1e293b",
  layoutBg: "#f5f7fa",
};

/** antd 主题配置。 */
export const appTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: {
    colorPrimary: brand.primary,
    colorInfo: brand.primary,
    colorBgLayout: brand.layoutBg,
    colorLink: brand.primary,

    borderRadius: 8,
    borderRadiusLG: 12,
    borderRadiusSM: 6,

    fontSize: 14,
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Microsoft YaHei", sans-serif',

    // 控件略高一点，视觉更舒展
    controlHeight: 36,
    controlHeightLG: 42,

    // 更克制的阴影，避免"浮起来"的廉价感
    boxShadowTertiary:
      "0 1px 2px rgba(16, 24, 40, 0.04), 0 1px 3px rgba(16, 24, 40, 0.06)",
  },
  components: {
    Layout: {
      headerHeight: 64,
      headerPadding: "0 24px",
      headerBg: "#ffffff",
      bodyBg: brand.layoutBg,
      siderBg: brand.siderBg,
      triggerBg: brand.siderBgHover,
    },
    Menu: {
      itemBorderRadius: 8,
      itemHeight: 42,
      itemMarginInline: 10,
      itemMarginBlock: 4,
      iconSize: 16,
      // 深色侧边栏
      darkItemBg: "transparent",
      darkSubMenuItemBg: "transparent",
      darkItemColor: "rgba(255, 255, 255, 0.72)",
      darkItemHoverBg: brand.siderBgHover,
      darkItemSelectedBg: brand.primary,
      darkItemSelectedColor: "#ffffff",
      darkItemHoverColor: "#ffffff",
    },
    Card: {
      borderRadiusLG: 12,
      paddingLG: 24,
      headerFontSize: 16,
      headerHeight: 52,
    },
    Table: {
      headerBg: "#fafbfc",
      headerColor: "rgba(0, 0, 0, 0.72)",
      headerSplitColor: "transparent",
      rowHoverBg: "#f7faff",
      cellPaddingBlock: 14,
      borderColor: "#eef1f5",
    },
    Button: {
      fontWeight: 500,
      primaryShadow: "none",
      defaultShadow: "none",
      dangerShadow: "none",
    },
    Tabs: {
      horizontalItemPadding: "12px 0",
      titleFontSize: 15,
    },
    Descriptions: {
      labelBg: "#fafbfc",
    },
    Modal: {
      borderRadiusLG: 12,
      titleFontSize: 17,
    },
    Tag: {
      borderRadiusSM: 6,
    },
    Segmented: {
      itemSelectedBg: "#ffffff",
    },
  },
};
