# API v2 契约（refactor/v2）

后端基址：`http://<host>:9080`，所有接口前缀 `/api`。

## 1. 响应信封

除 **v1 兼容接口**（见第 6 节）外，所有接口统一返回：

```json
{ "code": "OK", "message": "success", "data": { }, "requestId": "uuid" }
```

- 成功：`code = "OK"`，HTTP 200 / 201
- 失败：`code` 为错误码，HTTP 状态码语义正确，`data` 省略

错误码：`INVALID_PARAM | UNAUTHORIZED | FORBIDDEN | NOT_FOUND | CONFLICT | TOO_MANY_REQUESTS | PAYLOAD_TOO_LARGE | PASSWORD_REQUIRED | INTERNAL`

分页数据统一为 `{ list, total, page, pageSize }`，查询参数 `page`（默认 1）、`pageSize`（默认 20，上限 200）。

## 2. 认证

- `Authorization: Bearer <accessToken>`
- access token 24h（`APPV_JWT_EXPIRE_HOURS`），refresh token 7 天
- 角色：`admin`（全部 + 用户管理）、`releaser`（业务读写）、`viewer`（只读）

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/v2/auth/login` | 公开 | body `{username,password}` → `{accessToken,refreshToken,expiresAt,user}` |
| POST | `/api/v2/auth/refresh` | 公开 | body `{refreshToken}` |
| GET | `/api/v2/auth/profile` | 登录 | 当前用户 |
| POST | `/api/v2/auth/change-password` | 登录 | body `{oldPassword,newPassword}`；成功后旧令牌全部失效 |

`user`：`{id,username,displayName,role,mustChangePassword,lastLoginAt}`。
`mustChangePassword=true` 时前端应引导改密。

## 3. 用户与审计（仅 admin）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v2/users` | 分页列表 |
| POST | `/api/v2/users` | `{username,password,displayName,role}` |
| PUT | `/api/v2/users/:id` | `{displayName?,role?,isActive?,password?}` |
| DELETE | `/api/v2/users/:id` | 不能删除自己/最后一个管理员 |
| GET | `/api/v2/audit-logs` | 参数 `action`,分页；条目含 actorName/action/targetType/summary/ip/createdAt |

## 4. 应用 / 通道 / 版本

### 应用

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v2/apps?keyword=&page=&pageSize=` | 登录 | 分页；`keyword` 匹配名称/标识 |
| POST | `/api/v2/apps` | 写 | `{name,identifier,logo?,description?,platforms[],defaultChannel?}` |
| GET | `/api/v2/apps/:id` | 登录 | |
| PUT | `/api/v2/apps/:id` | 写 | 同创建；已有版本的平台不可移除 |
| DELETE | `/api/v2/apps/:id` | admin | 级联软删版本，同时删除通道/分享/模板 |

`Application`：`{id,name,identifier,logo,description,platforms[],defaultChannel,createdAt,updatedAt}`
- `platforms` 取值：`android|ios|windows|macos|linux|harmony`
- `logo` 为存储对象键；展示图片用 `GET /api/static/logos/<key>`（仅当该键被应用引用）

### 通道

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v2/apps/:id/channels` | 返回数组 |
| POST | `/api/v2/apps/:id/channels` | `{key,name,sort}`，key 仅小写字母数字 `-_` |
| PUT | `/api/v2/apps/:id/channels/:channelId` | `{key,name,sort}` |
| DELETE | `/api/v2/apps/:id/channels/:channelId` | 默认通道或有版本时拒绝 |
| PUT | `/api/v2/apps/:id/default-channel` | `{key}` |

`Channel`：`{id,appId,key,name,isDefault,sort}`，新建应用自动含 `stable`（默认）与 `beta`。

### 版本

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v2/versions?appId=&platform=&channel=&status=&keyword=&page=&pageSize=` | 登录 | `appId` 必填 |
| POST | `/api/v2/versions` | 写 | 见下 |
| GET | `/api/v2/versions/:id` | 登录 | |
| PUT | `/api/v2/versions/:id` | 写 | 任意子集字段 |
| PUT | `/api/v2/versions/:id/status` | 写 | `{status:"published"|"archived"|"draft"}` |
| DELETE | `/api/v2/versions/:id` | 写 | 软删除 |
| GET | `/api/v2/versions/:id/download` | 登录 | 直接下载产物 |

发布请求：
```json
{
  "appId": 1, "platform": "windows", "channel": "stable", "version": "1.2.3",
  "fileKey": "ab/cd/abcd..._setup.exe",
  "fileName": "setup.exe", "fileSize": 123, "fileSha256": "...", "contentType": "...",
  "changelog": "...", "ext": "{\"k\":\"v\"}", "forceUpdate": false,
  "minSupportedVersion": "1.0.0", "status": "published"
}
```
- `fileKey` 必填且必须已存在于文件库；`fileName/fileSize/fileSha256/contentType` 可由服务端从文件记录补全
- 同一 `(app, platform, channel, version)` 重复 → 409
- `ext` 必须是合法 JSON 对象

`Version`：`{id,appId,platform,channel,version,prerelease,fileKey,fileName,fileSize,fileSha256,contentType,changelog,ext,forceUpdate,status,minSupportedVersion,publishedAt,downloadCount,createdAt,updatedAt}`

## 5. 文件 / 分享 / 模板

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/v2/files/upload` | 写 | multipart 字段名 `file` → `{file:{...},deduplicated}`（相同 sha256 秒传） |
| GET | `/api/v2/files?keyword=&page=&pageSize=` | 登录 | |
| DELETE | `/api/v2/files/:id` | 写 | 仍被引用时 409 |
| POST | `/api/v2/files/clean` | 写 | 清理未被引用的孤儿文件 → `{removed}` |
| GET | `/api/v2/files/:id/download` | 登录 | |

`File`：`{id,name,key,size,contentType,sha256,md5,storage,refCount,createdAt}`

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v2/apps/:id/shares` | 登录 | |
| POST | `/api/v2/apps/:id/shares` | 写 | `{password?,expiresInDays?}` → `{id,token,hasPassword,expiresAt,isActive,createdAt}` |
| PUT | `/api/v2/apps/:id/shares/:shareId` | 写 | `{password?,expiresInDays?}`（`password:""` 清除密码，`expiresInDays:0` 永久） |
| PUT | `/api/v2/apps/:id/shares/:shareId/deactivate` | 写 | |
| DELETE | `/api/v2/apps/:id/shares/:shareId` | 写 | |

分享前台地址：`/share/<token>`（前端路由）

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v2/apps/:id/templates` | 登录 | |
| POST | `/api/v2/apps/:id/templates` | 写 | `{name,description?,content}` |
| PUT | `/api/v2/apps/:id/templates/:templateId` | 写 | |
| DELETE | `/api/v2/apps/:id/templates/:templateId` | 写 | |
| POST | `/api/v2/apps/:id/templates/:templateId/preview` | 登录 | `{platform?,channel?}` → `{contentType,content,platform,version}` |

模板变量：`{{.app.*}}`、`{{.ver.*}}`（`ver.filePath` 为下载令牌）、`{{.ext.*}}`。
创建/更新时会做语法校验，语法错误返回 400。

## 6. v1 兼容接口（结构与字段名保持不变）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/login` | → `{token,user:{id,username,role}}` |
| GET | `/api/open/latest?identifier=&platform=&channel=&format=` | 扁平 JSON：`appName,appId,identifier,version,platform,channel,changelog,isForce,fileName,filePath,fileSize,createdAt`；`filePath` 为下载令牌；`format` 命中模板时按模板 Content-Type 输出 |
| GET | `/api/open/changelog?identifier=&platform=&channel=` | 数组：`{createdAt,version,platform,changelog}` |
| GET | `/api/open/check?identifier=&platform=&channel=&currentVersion=` | 扁平：`hasUpdate,isForce,belowMinimumVersion,minSupportedVersion,version,changelog,fileName,filePath,fileSize,createdAt` |
| GET | `/api/open/download/:token` | 传统下载，支持 302 跳转（S3） |
| POST | `/api/share/:token` | body `{password?}`；无密码→`{app,versions,currentPlatform}`；需密码 → **HTTP 209** `{error,requirePassword:true}` |
| POST | `/api/share/:token/versions` | 同上鉴权，返回 `{total,list,page,pageSize}`，`list[].filePath` 为下载令牌 |

### v2 检测更新（推荐新客户端使用）

`GET /api/v2/check?identifier=&platform=&channel=&currentVersion=` →
`{identifier,platform,channel,currentVersion,hasUpdate,forceUpdate,belowMinimumVersion,minSupportedVersion,latest:{version,channel,platform,changelog,forceUpdate,publishedAt,fileName,fileSize,sha256,downloadUrl}}`

规则：`channel` 缺省用应用默认通道；`currentVersion` 缺省视为需要更新；`belowMinimumVersion` 为真时同时置 `forceUpdate=true`。

## 7. 其他

- `GET /healthz`、`GET /readyz`
- 图片类静态资源：`GET /api/static/logos/<key>`（仅被应用引用的键可访问，版本产物一律走签名下载）
- 所有请求响应带 `X-Request-Id`
- 登录限流（默认 10 次/分钟/IP）、分享校验 20 次/分钟、开放接口 600 次/分钟，超限 429
