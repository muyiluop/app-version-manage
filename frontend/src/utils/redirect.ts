/**
 * 登录回跳地址校验。
 *
 * 为什么单独成文件：这是安全（开放重定向）与流程正确性（改密后不能被送回改密页）
 * 的双重关键点，抽出来才能被单元测试覆盖。
 */

/** 登录/改密这类「流程页」不能作为回跳目标。 */
const AUTH_FLOW_PATHS = ["/login", "/change-password"];

/**
 * 只允许站内相对路径回跳。
 *
 * 排除流程页的原因：改密成功后 logout() 会让守卫重渲染，而 React Router v7 的路由
 * 切换是 transition（低优先级），于是守卫可能抢先把 /change-password 写进
 * ?redirect= —— 用户重新登录后又被送回改密页，看起来就像"改完密码还提示改密"。
 */
export function safeRedirect(raw: string | null | undefined): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//")) return "/apps";
  const path = raw.split("?")[0].split("#")[0];
  if (AUTH_FLOW_PATHS.includes(path)) return "/apps";
  return raw;
}
