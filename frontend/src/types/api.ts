/**
 * 与后端 v2 契约（docs/api-v2.md）严格对齐的类型定义。
 *
 * 约定：v2 接口统一返回 { code, message, data, requestId }，分页数据为
 * { list, total, page, pageSize }；v1 兼容接口不套信封，单独在 open.ts 里描述。
 */

/** 客户端平台，取值与后端 model.AllPlatforms 一致。 */
export type Platform = "android" | "ios" | "windows" | "macos" | "linux" | "harmony";

/** 后端 PlatformList 的展示顺序。 */
export const PLATFORMS: readonly Platform[] = ["android", "ios", "windows", "macos", "linux", "harmony"];

/** 用户角色。 */
export type UserRole = "admin" | "releaser" | "viewer";

/** 版本状态。 */
export type VersionStatus = "draft" | "published" | "archived";

/** v2 统一响应信封。 */
export interface ApiEnvelope<T> {
  code: string;
  message: string;
  data?: T;
  requestId?: string;
}

/** 分页结果。 */
export interface Paginated<T> {
  list: T[];
  total: number;
  page: number;
  pageSize: number;
}

/** 分页查询参数（page 默认 1，pageSize 默认 20、上限 200）。 */
export interface PageQuery {
  page?: number;
  pageSize?: number;
}

/** 消息型响应（后端部分接口返回 { message }）。 */
export interface MessageResult {
  message: string;
}

/** 修改密码结果：附带更新后的用户信息，便于前端同步 mustChangePassword。 */
export interface ChangePasswordResult extends MessageResult {
  user: UserProfile;
}

// ---------- 认证 ----------

/** 当前登录用户信息，对应 GET /v2/auth/profile。 */
export interface UserProfile {
  id: number;
  username: string;
  displayName: string;
  role: UserRole;
  mustChangePassword: boolean;
  lastLoginAt: string | null;
}

/** 登录/刷新结果。 */
export interface LoginResult {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  user: UserProfile;
}

// ---------- 应用 ----------

/** 应用。 */
export interface Application {
  id: number;
  name: string;
  identifier: string;
  logo: string;
  description: string;
  platforms: Platform[];
  defaultChannel: string;
  createdAt: string;
  updatedAt: string;
}

/** 应用创建/更新入参。 */
export interface AppInput {
  name: string;
  identifier: string;
  logo?: string;
  description?: string;
  platforms: Platform[];
  defaultChannel?: string;
}

// ---------- 通道 ----------

/** 发布通道。 */
export interface Channel {
  id: number;
  appId: number;
  key: string;
  name: string;
  isDefault: boolean;
  sort: number;
  createdAt: string;
  updatedAt: string;
}

/** 通道创建/更新入参，key 仅允许小写字母数字与 -_ 。 */
export interface ChannelInput {
  key: string;
  name: string;
  sort: number;
}

// ---------- 版本 ----------

/** 版本。 */
export interface Version {
  id: number;
  appId: number;
  platform: Platform;
  channel: string;
  version: string;
  prerelease: string;
  /** 存储层对象键，发布时必填且必须已存在于文件库。 */
  fileKey: string;
  fileName: string;
  fileSize: number;
  fileSha256: string;
  contentType: string;
  changelog: string;
  /** JSON 文本形式的扩展信息。 */
  ext: string;
  forceUpdate: boolean;
  status: VersionStatus;
  minSupportedVersion: string;
  publishedAt: string | null;
  downloadCount: number;
  createdAt: string;
  updatedAt: string;
}

/** 发布版本入参，fileKey 必填，其余文件字段服务端可补全。 */
export interface PublishVersionInput {
  appId: number;
  platform: Platform;
  channel?: string;
  version: string;
  fileKey: string;
  fileName?: string;
  fileSize?: number;
  fileSha256?: string;
  contentType?: string;
  changelog?: string;
  ext?: string;
  forceUpdate?: boolean;
  minSupportedVersion?: string;
  status?: VersionStatus;
}

/** 更新版本入参，任意子集。 */
export type UpdateVersionInput = Partial<Omit<PublishVersionInput, "appId">> & { status?: VersionStatus };

// ---------- 文件 ----------

/** 文件库条目。 */
export interface StoredFile {
  id: number;
  name: string;
  /** 存储对象键。 */
  key: string;
  size: number;
  contentType: string;
  sha256: string;
  md5: string;
  storage: string;
  refCount: number;
  createdAt: string;
}

/** 上传结果，deduplicated 为 true 表示命中 sha256 秒传。 */
export interface UploadResult {
  file: StoredFile;
  deduplicated: boolean;
}

/** 清理未使用文件的结果。 */
export interface CleanResult {
  removed: number;
  message: string;
}

// ---------- 分享 ----------

/** 分享链接。 */
export interface Share {
  id: number;
  token: string;
  hasPassword: boolean;
  expiresAt: string | null;
  isActive: boolean;
  accessCount: number;
  createdAt: string;
}

/** 创建分享入参，expiresInDays 为 0 表示永久。 */
export interface CreateShareInput {
  password?: string;
  expiresInDays?: number;
}

/** 更新分享入参：password 传空字符串清除密码，expiresInDays 传 0 表示永久。 */
export interface UpdateShareInput {
  password?: string;
  expiresInDays?: number;
}

// ---------- 模板 ----------

/** 输出模板。 */
export interface Template {
  id: number;
  appId: number;
  name: string;
  description: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

/** 模板创建/更新入参。 */
export interface TemplateInput {
  name: string;
  description?: string;
  content: string;
}

/** 模板预览结果。 */
export interface TemplatePreview {
  contentType: string;
  content: string;
  platform: Platform;
  version: string;
}

// ---------- 用户与审计 ----------

/** 创建用户入参。 */
export interface CreateUserInput {
  username: string;
  password: string;
  displayName?: string;
  role: UserRole;
}

/** 更新用户入参。 */
export interface UpdateUserInput {
  displayName?: string;
  role?: UserRole;
  isActive?: boolean;
  password?: string;
}

/** 审计日志条目。 */
export interface AuditLog {
  id: number;
  actorId: number;
  actorName: string;
  action: string;
  targetType: string;
  targetId: string;
  summary: string;
  detail: string;
  ip: string;
  userAgent: string;
  success: boolean;
  createdAt: string;
}

// ---------- v1 兼容 / 开放接口 ----------

/** v1 版本结构，字段名与历史接口保持一致。 */
export interface LegacyVersion {
  id: number;
  appId: number;
  platform: Platform;
  version: string;
  /** 下载令牌（v1 语义），需拼到 /open/download/ 之后使用。 */
  filePath: string;
  fileName: string;
  fileSize: number;
  changelog: string;
  ext: string;
  forceUpdate: boolean;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

/** 分享门户中的应用信息（v1 结构，没有 id 字段）。 */
export interface LegacySharedApp {
  name: string;
  identifier: string;
  logo: string;
  description: string;
  platforms: string[];
}

/** POST /share/:token 的返回。 */
export interface ShareAccessResult {
  app: LegacySharedApp;
  versions: LegacyVersion[];
  currentPlatform: string;
}

/** 分享需要密码时的响应（HTTP 209），v1 兼容结构。 */
export interface SharePasswordRequired {
  error: string;
  requirePassword: true;
}

/** 开放接口 /open/latest 的扁平结构。 */
export interface OpenLatest {
  appName: string;
  appId: number;
  identifier: string;
  version: string;
  platform: Platform;
  channel: string;
  changelog: string;
  isForce: boolean;
  fileName: string;
  filePath: string;
  fileSize: number;
  createdAt: string;
}

/** 开放接口 /open/changelog 的条目。 */
export interface OpenChangelogItem {
  createdAt: string;
  version: string;
  platform: Platform;
  changelog: string;
}

/** v2 检测更新返回的最新版本信息。 */
export interface CheckVersionInfo {
  version: string;
  channel: string;
  platform: Platform;
  changelog: string;
  forceUpdate: boolean;
  publishedAt: string | null;
  fileName: string;
  fileSize: number;
  sha256?: string;
  downloadUrl: string;
}

/** GET /v2/check 的返回。 */
export interface CheckResult {
  identifier: string;
  platform: Platform;
  channel: string;
  currentVersion: string;
  hasUpdate: boolean;
  forceUpdate: boolean;
  belowMinimumVersion: boolean;
  minSupportedVersion?: string;
  latest: CheckVersionInfo;
}
