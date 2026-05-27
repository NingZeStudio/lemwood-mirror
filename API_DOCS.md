# 柠枺镜像 API 文档

> 本文档覆盖所有公共查询接口、下载接口和扫描触发接口。
> 后台管理接口（`/api/admin/*`、`/api/login`）不在本文档范围内。

## 1. 基础约定

### 1.1 Base URL

- 站内调用使用相对路径：`/api/...`
- 外部调用拼接站点域名，例如：`https://mirror.example.com/api`

### 1.2 内容类型

| 接口 | 返回类型 |
|------|----------|
| `GET /api/status`、`/api/stats` 等查询接口 | `application/json` |
| `GET /api/latest/{launcher}` | `text/plain; charset=utf-8`（纯文本） |
| `GET /download/{token}/{file_path}` | `application/octet-stream`（文件流） |
| `POST /api/scan`、`/api/scan/launcher` | `text/plain` 或 `application/json` |

### 1.3 CORS

所有接口均返回以下 CORS 响应头：

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Expose-Headers: X-Latest-Version, X-Latest-Versions
```

`OPTIONS` 预检请求直接返回 `200 OK`，无需额外配置。

### 1.4 常见状态码

| 状态码 | 含义 |
|--------|------|
| `200 OK` | 请求成功 |
| `202 Accepted` | 异步任务已触发（扫描接口） |
| `400 Bad Request` | 缺少参数、参数格式错误、或验证码未启用 |
| `401 Unauthorized` | 后台接口需要登录 token |
| `403 Forbidden` | 下载令牌无效/过期、验证码校验失败、流量超限、IP 被封禁 |
| `404 Not Found` | 启动器不存在、文件不存在、或路径无效 |
| `405 Method Not Allowed` | 请求方法不正确（如用 GET 访问扫描接口） |
| `500 Internal Server Error` | 服务端执行失败 |
| `501 Not Implemented` | 接口尚未实现（`/api/files`） |

### 1.5 下载令牌

- 令牌为 64 字符十六进制随机字符串（32 字节），不包含任何用户信息。
- 默认有效期 **5 分钟**，超时自动失效。
- **两种操作模式：**
  - **`Peek`**：查看令牌对应的下载信息，不消耗令牌。`/api/download/landing` 使用此模式。
  - **`Validate`**：验证令牌并立即消费（删除）。`/download/{token}/...` 实际下载时使用此模式，令牌一次性有效。
- 令牌一旦被 `Validate` 消费，后续的 `Peek` 或再次 `Validate` 都会失败。
- 后台协程每 1 分钟清理一次过期令牌。

### 1.6 流量限制

- 配置项 `traffic_limit_gb` 控制单 IP 每日下载流量上限。
- `0` 表示**完全关闭**流量限制。
- 负值自动修正为 `5` GB。
- 下载接口（`/download/...`）在处理前会**预估**传输字节数（通过 `Range` 头计算），若预估已超限则直接返回 `403 Forbidden`。
- 实际传输完成后，精确字节数会被写入数据库；若当日累计超限，IP 会被自动加入本地黑名单。
- 触发流量封禁后，所有该 IP 的后续请求均返回 `403 Forbidden`。

---

## 2. 公共查询接口

### 2.1 获取所有启动器状态

```
GET /api/status
```

返回所有启动器的全部版本列表，每个启动器按版本从新到旧排序。

**响应示例：**

```json
{
  "fcl": [
    {
      "launcher": "fcl",
      "tag_name": "1.3.0.7",
      "name": "1.3.0.7",
      "published_at": "2025-01-01T00:00:00Z",
      "is_latest": true,
      "assets": [
        {
          "name": "FCL-release-1.3.0.7-all.apk",
          "url": "https://mirror.example.com/download/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
          "size": 12345678
        }
      ]
    }
  ],
  "zl": [],
  "zl2": []
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `launcher` | string | 启动器标识，与配置中的 `name` 一致 |
| `tag_name` | string | GitHub Release 的 tag 名称 |
| `name` | string | GitHub Release 的标题名称 |
| `published_at` | string | 发布时间（ISO 8601 / RFC 3339） |
| `is_latest` | bool | 是否为该启动器的当前最新版本 |
| `assets[].name` | string | 资源文件名 |
| `assets[].url` | string | 构造的下载地址（受 `download_url_base` / `server_address` 配置影响） |
| `assets[].size` | int | 资源文件大小（字节） |

> 注意：接口返回的是以 `launcher` 名称为 key 的对象，而非数组。每个 key 对应的值是版本数组，按发布时间降序排列。

### 2.2 获取指定启动器状态

```
GET /api/status/{launcher}
```

**路径参数：**

| 参数 | 说明 |
|------|------|
| `launcher` | 启动器标识，如 `fcl`、`zl`、`zl2` |

返回单个启动器的版本数组。

**响应示例：**

```json
[
  {
    "launcher": "fcl",
    "tag_name": "1.3.0.7",
    "name": "1.3.0.7",
    "published_at": "2025-01-01T00:00:00Z",
    "is_latest": true,
    "assets": [
      {
        "name": "FCL-release-1.3.0.7-all.apk",
        "url": "https://mirror.example.com/download/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
        "size": 12345678
      }
    ]
  }
]
```

**错误场景：**

- `404 Not Found`：`launcher` 不存在（在配置中未定义或无已扫描版本）

### 2.3 获取所有启动器最新版本

```
GET /api/latest
```

返回每个启动器的最新版本号。**结果同时以 JSON 响应体和自定义响应头两种形式返回**，方便不同场景读取。

**响应头：** `X-Latest-Versions: {"fcl":"1.3.0.7","zl":"141400","zl2":"2.4.4"}`

**响应示例：**

```json
{
  "fcl": "1.3.0.7",
  "zl": "141400",
  "zl2": "2.4.4"
}
```

**最新版本判定规则：**
1. 优先选择 `index.json` 中标记了 `is_latest: true` 的版本中版本号最大的
2. 若无 `is_latest` 标记，则选择稳定版本（排除版本号含 `alpha`、`beta`、`rc`、`snapshot`、`pre`、`dev` 的）中最新者
3. 若无稳定版本，回退到任意可用版本

### 2.4 获取指定启动器最新版本

```
GET /api/latest/{launcher}
```

**路径参数：**

| 参数 | 说明 |
|------|------|
| `launcher` | 启动器标识 |

返回**纯文本**版本号，非 JSON。

**响应头：** `X-Latest-Version: 1.3.0.7`

**响应示例：**

```text
1.3.0.7
```

**错误场景：**

- `404 Not Found`：`launcher` 不存在

### 2.5 获取站点统计信息

```
GET /api/stats
```

返回访问量、下载量、磁盘占用、热门资源和趋势数据。

**缓存：** 响应头 `Cache-Control: public, max-age=300`（5 分钟），服务端内部有同 TTL 的内存缓存。

**响应示例：**

```json
{
  "total_visits": 1500,
  "total_downloads": 450,
  "total_days": 15,
  "last_30_visits": 300,
  "last_30_downloads": 80,
  "disk": {
    "total": 53687091200,
    "free": 10737418240,
    "used": 42949672960
  },
  "top_downloads": [
    {
      "launcher": "fcl",
      "version": "1.3.0.7",
      "count": 120
    }
  ],
  "geo_distribution": [
    {
      "country": "China",
      "count": 300
    }
  ],
  "daily_stats": [
    {
      "date": "2026-05-20",
      "visit_count": 80,
      "download_count": 22
    }
  ]
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `total_visits` | int | 累计访问量（所有非下载请求） |
| `total_downloads` | int | 累计下载次数（仅 `200/206` 响应的下载请求） |
| `total_days` | int | 站点自首次启动至今的天数 |
| `last_30_visits` | int | 最近 30 天访问量 |
| `last_30_downloads` | int | 最近 30 天下载量 |
| `disk.total` | int | 存储路径所在磁盘总容量（字节） |
| `disk.free` | int | 剩余可用空间（字节） |
| `disk.used` | int | 已用空间（字节） |
| `top_downloads` | array | 下载排行 Top 10，按下载次数降序 |
| `top_downloads[].launcher` | string | 启动器标识 |
| `top_downloads[].version` | string | 版本号 |
| `top_downloads[].count` | int | 下载次数 |
| `geo_distribution` | array | 地区分布 Top 50，按访问量降序，排除本地和空白记录 |
| `geo_distribution[].country` | string | 国家/地区名 |
| `geo_distribution[].count` | int | 访问次数 |
| `daily_stats` | array | 最近 30 天每日统计 |
| `daily_stats[].date` | string | 日期（`YYYY-MM-DD`） |
| `daily_stats[].visit_count` | int | 当日访问量 |
| `daily_stats[].download_count` | int | 当日下载量 |

### 2.6 获取验证码配置

```
GET /api/captcha/config
```

前端发起浏览器下载前可先读取验证码配置，判断是否需要走验证流程。

**响应示例：**

```json
{
  "enabled": true,
  "app_id": "9fab8370f958912499555f6ce0cd5c56"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用验证码（对应配置 `captcha_enabled`） |
| `app_id` | string | 极验验证码初始化所需的 App ID |

### 2.7 获取两步验证状态

```
GET /api/auth/2fa/status
```

返回当前是否启用管理员两步验证（TOTP）。

**响应示例：**

```json
{
  "enabled": true
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否为后台登录启用了 TOTP 两步验证 |

### 2.8 触发全量扫描

```
POST /api/scan
```

触发对所有已配置启动器的全量扫描。扫描为**异步**执行，接口立即返回。

**响应：**

```text
Scan triggered
```

- 状态码：`202 Accepted`
- 说明：同一时间只允许一个全量扫描运行（内部互斥锁），若已有扫描在进行中，新的触发会被忽略。

### 2.9 触发单个启动器扫描

```
POST /api/scan/launcher
```

触发对指定启动器的扫描。扫描为**异步**执行。

**请求体：**

```json
{
  "launcher": "fcl"
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `launcher` | string | 是 | 启动器标识，必须与配置中的 `name` 一致 |

**成功响应：**

```json
{
  "status": "accepted",
  "message": "扫描已触发"
}
```

- 状态码：`202 Accepted`

**错误场景：**

- `400 Bad Request`：请求体解析失败，或 `launcher` 为空
- `405 Method Not Allowed`：使用了 GET 等非 POST 方法

---

## 3. 下载相关接口

### 3.1 准备下载（无验证码）

```
POST /api/download/prepare
```

在验证码**关闭**时，前端调用此接口生成下载令牌和下载地址。

**请求体：**

```json
{
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "",
  "source": "home-latest-download"
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_path` | string | 是 | 目标文件在 `storage_path` 下的相对路径，格式 `{launcher}/{version}/{filename}` |
| `return_url` | string | 否 | 下载完成后前端可用于跳转回来源页面 |
| `source` | string | 否 | 自定义来源标记，用于日志/统计 |

**成功响应：**

```json
{
  "download_token": "a1b2c3d4e5f6...（64字符十六进制字符串）",
  "download_url": "/download/a1b2c3.../fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/download/landing?token=a1b2c3..."
}
```

**错误场景：**

- `400 Bad Request`：缺少 `file_path`
- `403 Forbidden`：文件路径包含 `..` 等非法字符，或路径不在 `storage_path` 范围内
- `404 Not Found`：`file_path` 对应的文件在磁盘上不存在

### 3.2 获取下载引导信息

```
GET /api/download/landing?token={token}
```

**Query 参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `token` | string | 是 | `prepare` 或 `verify` 返回的 `download_token` |

前端下载引导页使用此接口读取下载地址、来源信息和文件详情。**此接口使用 `Peek` 模式读取令牌，不消耗令牌。**

**成功响应：**

```json
{
  "download_url": "/download/a1b2c3.../fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "https://example.com/back",
  "source": "home-latest-download",
  "file_name": "FCL-release-1.3.0.7-all.apk",
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "flow": "prepare"
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `download_url` | string | 真实文件下载路径（含 token） |
| `return_url` | string | 来源页面地址（由调用方在 `prepare`/`verify` 时传入） |
| `source` | string | 来源标记 |
| `file_name` | string | 文件名（从 `file_path` 中提取） |
| `file_path` | string | 文件相对路径 |
| `flow` | string | 令牌来源：`"prepare"`（无验证码流程）或 `"verify"`（验证码流程） |

**错误场景：**

- `400 Bad Request` — 缺少 `token`

```json
{
  "error": "missing_token",
  "message": "Missing token"
}
```

- `403 Forbidden` — 令牌不存在、已过期、或已被下载消费

```json
{
  "error": "expired_token",
  "message": "Download token is invalid or expired"
}
```

### 3.3 验证后生成下载令牌

```
POST /api/download/verify
```

在验证码**开启**时，前端完成极验验证后调用此接口获取下载令牌。

**请求体：**

```json
{
  "lot_number": "e2f0a767a0f74926bbc8daeed22e6f27",
  "captcha_output": "captcha_output_string",
  "pass_token": "pass_token_string",
  "gen_time": "1709551234",
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "",
  "source": "verify-download"
}
```

**字段说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `lot_number` | string | 是 | 极验 SDK 回调参数 |
| `captcha_output` | string | 是 | 极验 SDK 回调参数 |
| `pass_token` | string | 是 | 极验 SDK 回调参数 |
| `gen_time` | string | 是 | 极验 SDK 回调参数（时间戳字符串） |
| `file_path` | string | 是 | 目标文件相对路径 |
| `return_url` | string | 否 | 来源页面地址 |
| `source` | string | 否 | 来源标记 |

**成功响应：** 与 `prepare` 接口一致

```json
{
  "download_token": "a1b2c3d4e5f6...",
  "download_url": "/download/a1b2c3.../fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/download/landing?token=a1b2c3..."
}
```

**错误场景：**

- `400 Bad Request`
  - 验证码未启用（`captcha_enabled` 为 `false`）
  - 任一极验参数（`lot_number`、`captcha_output`、`pass_token`、`gen_time`）为空
  - 缺少 `file_path`
- `403 Forbidden` — 极验服务端验证不通过

```json
{
  "error": "verification_failed",
  "message": "极验返回的拒绝原因"
}
```

- `404 Not Found` — `file_path` 对应文件不存在
- `500 Internal Server Error` — 与极验服务端通信失败

```json
{
  "error": "verification_failed",
  "message": "Failed to verify captcha"
}
```

### 3.4 真实文件下载

```
GET /download/{token}/{file_path}
```

返回真实文件流。token 也可以放在 Query 参数中：`GET /download/{file_path}?token={token}`。

此接口使用 `Validate` 模式消费令牌——**每个令牌仅允许一次成功下载**。

**在验证码关闭时：**
- 无 token 的请求仍然可以访问 `/download/...`，但建议走 `prepare → landing → download` 标准流程以进行流量统计和来源追踪。

**在验证码开启时：**
- 浏览器请求无有效 token：返回验证码 HTML 页面（而非文件流）
- 非浏览器请求无有效 token：返回 JSON 错误

```json
{
  "error": "verification_required",
  "message": "Download requires captcha verification",
  "captcha": true,
  "app_id": "9fab8370f958912499555f6ce0cd5c56"
}
```

- Token 无效/过期（浏览器）：返回验证码页面
- Token 无效/过期（非浏览器）：

```json
{
  "error": "invalid_token",
  "message": "Download token is invalid or expired",
  "captcha": true,
  "app_id": "9fab8370f958912499555f6ce0cd5c56"
}
```

- Token 与请求的文件路径不匹配：

```json
{
  "error": "token_mismatch",
  "message": "Download token does not match requested file"
}
```

- 文件不存在：

```json
{
  "error": "file_not_found",
  "message": "Requested file not found"
}
```

- 路径包含 `..`：

```json
{
  "error": "invalid_path",
  "message": "Invalid file path"
}
```

**支持的 HTTP 特性：**
- `Range` 请求（断点续传）：响应 `206 Partial Content`
- 流量预估：根据 `Range` 头计算预估传输字节数，用于流量限制预检

---

## 4. 浏览器下载流程

### 4.1 验证码关闭时的流程

```
前端                      服务端
 │                          │
 │  POST /api/download/prepare
 │  { file_path, ... }      │
 │ ─────────────────────────>
 │                          │  生成 token（Flow: "prepare"）
 │  { download_token,       │
 │    download_url,         │
 │    landing_url }         │
 │ <─────────────────────────
 │                          │
 │  GET /api/download/landing?token=...
 │ ─────────────────────────>
 │                          │  Peek token（不消费）
 │  { download_url,         │
 │    file_name, flow, ... }│
 │ <─────────────────────────
 │                          │
 │  GET /download/{token}/{file_path}
 │ ─────────────────────────>
 │                          │  Validate token（消费）
 │                          │  流量预检
 │  <binary file stream>    │
 │ <─────────────────────────
 │                          │  记录下载统计
```

### 4.2 验证码开启时的流程

```
前端                      服务端                      极验
 │                          │                          │
 │  GET /api/captcha/config
 │ ─────────────────────────>
 │  { enabled, app_id }     │
 │ <─────────────────────────
 │                          │
 │  （用户完成极验验证）      │
 │ ────────────────────────────────────────────────────>
 │ <────────── lot_number, captcha_output, pass_token, gen_time
 │                          │
 │  POST /api/download/verify
 │  { lot_number, ... }     │
 │ ─────────────────────────>
 │                          │  ── POST /api/v2/validate
 │                          │ ──────────────────────────>
 │                          │ <── { result: "success" }
 │                          │  生成 token（Flow: "verify"）
 │  { download_token,       │
 │    download_url,         │
 │    landing_url }         │
 │ <─────────────────────────
 │                          │
 │  GET /api/download/landing?token=...
 │ ─────────────────────────>
 │                          │  Peek token（不消费）
 │  { download_url, ... }   │
 │ <─────────────────────────
 │                          │
 │  GET /download/{token}/{file_path}
 │ ─────────────────────────>
 │                          │  Validate token（消费）
 │  <binary file stream>    │
 │ <─────────────────────────
```

---

## 5. 错误码速查

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `missing_token` | 400 | 缺少 `token` 参数 |
| `missing_required_parameters` | 400 | 缺少必填字段（`file_path` 等） |
| `expired_token` | 403 | 令牌不存在、已过期、或已被消费 |
| `invalid_token` | 403 | 令牌无效（同 `expired_token`，用于下载接口） |
| `token_mismatch` | 403 | 令牌绑定的文件与当前请求路径不一致 |
| `verification_required` | 403 | 需要先完成验证码，无有效令牌 |
| `verification_failed` | 403 | 极验服务端验证不通过 |
| `file_not_found` | 404 | 请求的文件在磁盘上不存在 |
| `invalid_path` | 403/404 | 文件路径包含非法字符（如 `..`）或超出存储目录 |

---

## 6. 扫描流程说明

### 6.1 定时扫描

由 `check_cron` 表达式控制（默认每 10 分钟），自动对所有已配置启动器执行：
1. 从 `source_url` 解析 GitHub 仓库地址
2. 按 `include_prerelease` 和 `max_versions` 策略拉取 release 列表
3. 下载资源文件到 `storage_path/{launcher}/{version}/`
4. 写入 `index.json` 版本元数据
5. 清理超出 `max_versions` 的旧版本目录

### 6.2 手动扫描

- `POST /api/scan`：全量扫描所有启动器
- `POST /api/scan/launcher`：扫描指定启动器

两者均为异步执行，立即返回 `202 Accepted`。同一时间只允许一个全量扫描运行（互斥锁保护）。

### 6.3 版本保留规则

| `max_versions` 配置值 | 实际保留版本数 |
|----------------------|---------------|
| `N > 0` | 保留最近 N 个版本 |
| `N = 0` | 使用默认值 **3** |
| `N < 0` | 使用默认值 **3** |

---

## 7. 注意事项

- **`GET /api/latest/{launcher}` 返回纯文本**，不是 JSON。解析时不要调用 `JSON.parse()`，直接读取响应文本。
- **`/download/...` 接口对 token 的两种操作不同：**
  - `landing` 用 `Peek`（不消费，可多次调用）
  - 实际下载用 `Validate`（消费，一次性，用完即删）
- **流量限制在下载前做预估**（基于 `Range` 头），下载后按实际字节数精确记录。预估超限和实际超限都会导致封禁。
- **`return_url` 和 `source` 完全由调用方传入**，服务端不会自动推断来源站点。
- **`download_url_base` 为空时**，服务端会自动尝试使用 `server_address`，若也为空则通过 `ifconfig.me/ip` 获取公网 IP 作为下载地址 host。
- **`max_versions = 0` 等价于 `max_versions = 3`**（由 `NormalizeMaxVersions` 函数统一处理，≤0 均修正为 3）。
- **`traffic_limit_gb` 负值会自动修正为 5** GB，`0` 表示完全禁用。
- **扫描接口无认证保护**，建议在生产环境通过反向代理添加访问控制。
- 站内 API 速查页（`/api`）用于快速浏览与复制示例，**正式接入请以本文档为准**。
