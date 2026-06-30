# 柠枺镜像 API 文档

> 本文档覆盖所有公共查询接口、下载接口、扫描触发接口以及管理后台接口。
> 所有 API 端点均以 `/api/v2/` 为前缀。

## 1. 基础约定

### 1.1 API 版本

所有 API 端点均以 `/api/v2/` 为前缀，项目仅保留 v2 一套 API，不存在版本选择开关。

### 1.2 Base URL

- 站内调用使用相对路径：`/api/v2/...`
- 外部调用拼接站点域名，例如：`https://beta.miawa.cn/api/v2`

### 1.3 内容类型

| 接口 | 返回类型 |
|------|----------|
| `GET /api/v2/launchers`、`/api/v2/stats` 等查询接口 | `application/json`（统一信封） |
| `GET /api/v2/latest/{launcher}` | `text/plain; charset=utf-8`（纯文本，不走信封） |
| `GET /download/{token}/{file_path}` | `application/octet-stream`（文件流） |
| `GET /repo/{launcher}.git/...` | Git 仓库静态文件（供 `git clone` / `git fetch` 使用） |
| `POST /api/v2/admin/scans`、`/api/v2/admin/scans/launcher` | `application/json`（统一信封） |
| `GET /api/v2/admin/files/download` | `application/octet-stream`（二进制流，不走信封） |

### 1.4 CORS

所有接口均返回以下 CORS 响应头：

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS
Access-Control-Expose-Headers: X-Latest-Version, X-Latest-Versions
```

`OPTIONS` 预检请求直接返回 `200 OK`，无需额外配置。

### 1.5 常见状态码

| 状态码 | 含义 |
|--------|------|
| `200 OK` | 请求成功 |
| `201 Created` | 资源创建成功（如黑名单新增） |
| `202 Accepted` | 异步任务已触发（扫描接口、自更新检查） |
| `304 Not Modified` | ETag 条件请求命中，内容未变 |
| `400 Bad Request` | 缺少参数、参数格式错误、或验证码未启用 |
| `401 Unauthorized` | 后台接口需要登录 token |
| `403 Forbidden` | 下载令牌无效/过期、验证码校验失败、流量超限、IP 被封禁、后台未启用 |
| `404 Not Found` | 启动器不存在、文件不存在、或路径无效 |
| `405 Method Not Allowed` | 请求方法不正确（如用 GET 访问扫描接口） |
| `500 Internal Server Error` | 服务端执行失败 |
| `501 Not Implemented` | 接口依赖的功能未配置（如自更新未启用） |
| `502 Bad Gateway` | 自更新检查/应用时与上游通信失败 |

### 1.6 下载令牌

- 令牌为 64 字符十六进制随机字符串（32 字节），不包含任何用户信息。
- 默认有效期 **5 分钟**，超时自动失效。
- **两种操作模式：**
  - **`Peek`**：查看令牌对应的下载信息，不消耗令牌。`/api/v2/downloads/landing` 使用此模式。
  - **`Validate`**：验证令牌并立即消费（删除）。`/download/{token}/...` 实际下载时使用此模式，令牌一次性有效。
- 令牌一旦被 `Validate` 消费，后续的 `Peek` 或再次 `Validate` 都会失败。
- 后台协程每 1 分钟清理一次过期令牌。

### 1.7 流量限制

- 配置项 `traffic_limit_gb` 控制单 IP 每日下载流量上限。
- `0` 表示**完全关闭**流量限制。
- 负值自动修正为 `5` GB。
- 下载接口（`/download/...`）与 Git 镜像接口（`/repo/...`）都会在处理前**预估**传输字节数（通过 `Range` 头计算），若预估已超限则直接返回 `403 Forbidden`。
- 两类流量**分开计算**：
  - `/download/...` 写入 `ip_daily_traffic`
  - `/repo/...` 写入 `repo_ip_daily_traffic`
- 实际传输完成后，精确字节数会被写入数据库；若对应类型的当日累计超限，IP 会被自动加入本地黑名单。
- 触发流量封禁后，所有该 IP 的后续请求均返回 `403 Forbidden`。

> **数据库 schema 版本追踪**：系统通过 `system_info` 表中 `key='schema_version'` 的行追踪数据库 schema 版本（缺失视为 v0）。服务启动时会自动应用未执行的版本化迁移，当前目标版本为 v2。迁移只支持 UP 方向，回滚需借助 `mysqldump` 备份恢复。

### 1.8 Git 仓库镜像

- 当 launcher 的 `mode` 为 `clone` 或 `all` 时，服务会同步 Git 镜像到项目根目录 `repo/{launcher}.git`。
- 标准克隆地址为：`GET /repo/{launcher}.git/...`，例如：`git clone https://beta.miawa.cn/repo/fcl.git`。
- `/repo/...` 仅支持只读访问，供 `git clone` / `git fetch` 使用。
- `/repo/...` 走**独立的 repo 流量计量与 repo 下载统计**，不与普通 `/download/...` 混算。
- `/repo/...` 不走下载验证码与下载令牌。

### 1.9 内嵌前端资源

- 用户前端（`web/default`）和后台前端（`web/admin`）会被构建进二进制。
- 服务启动时会自动释放 `web/default`、`web/admin` 到项目目录。
- 每次启动都会重新释放；内容未变化的文件会跳过写入以减少 IO，确保二进制内嵌的前端版本总是即时生效。

### 1.10 统一响应信封

所有 JSON 接口返回统一信封结构，客户端只需一套解析逻辑：

```json
{
  "data": "<业务载荷，失败时为 null>",
  "error": {
    "code": "error_code",
    "message": "人类可读说明",
    "details": {}
  },
  "meta": {
    "version": "v2",
    "timestamp": "2026-06-18T12:00:00Z",
    "request_id": "a1b2c3d4e5f6g7h8",
    "cached": false
  }
}
```

- **成功**：`data` 有值，`error` 为 `null`，HTTP 200（扫描/自更新检查接口 202，黑名单新增 201）
- **失败**：`data` 为 `null`，`error` 有值，HTTP 状态码反映错误类型（400/401/403/404/405/500/501/502）
- `meta.request_id` 便于日志追踪
- `meta.cached` 标识是否命中服务端缓存

**不走信封的例外：**

| 接口 | 返回形式 |
|------|----------|
| `GET /api/v2/latest/{launcher}` | 纯文本版本号（`text/plain`） |
| `GET /api/v2/admin/files/download` | 二进制文件流（`application/octet-stream`） |

> 本文后续章节的响应示例仅展示 `data` 字段内容（业务载荷），错误示例仅展示 `error` 字段内容，完整响应请按上述信封结构包裹。

### 1.11 性能特性

#### ETag 条件请求

GET 查询接口（`/launchers`、`/latest`、`/stats` 等）返回 `ETag` 弱标签头。客户端可在后续请求中携带 `If-None-Match` 头，若内容未变则服务端返回 `304 Not Modified`（无响应体），大幅减少带宽和解析开销。

```http
GET /api/v2/launchers
→ 200 OK
  ETag: W/"a1b2c3d4e5f6g7h8"
  Cache-Control: public, max-age=300
  { "data": ..., "error": null, "meta": {...} }

GET /api/v2/launchers
If-None-Match: W/"a1b2c3d4e5f6g7h8"
→ 304 Not Modified
```

#### gzip 压缩

当客户端发送 `Accept-Encoding: gzip` 且响应体大于 1KB 时，服务端自动 gzip 压缩并返回 `Content-Encoding: gzip` 头。

#### 统一缓存

所有 GET 查询接口返回 `Cache-Control: public, max-age=300`（5 分钟），服务端内部对 `/stats` 有同 TTL 的内存缓存。

---

## 2. 公共查询接口

> 本节所有 JSON 响应均使用 1.10 节定义的统一信封包裹，以下示例仅展示 `data` 字段内容。

### 2.1 获取所有启动器状态

```
GET /api/v2/launchers
```

返回所有启动器的全部版本列表，每个启动器按版本从新到旧排序。

**响应示例（`data` 字段）：**

```json
{
  "fcl": [
    {
      "launcher": "fcl",
      "tag_name": "1.3.0.7",
      "name": "1.3.0.7",
      "published_at": "2025-01-01T00:00:00Z",
      "is_latest": true,
      "clone_url": "https://beta.miawa.cn/repo/fcl.git",
      "assets": [
        {
          "name": "FCL-release-1.3.0.7-all.apk",
          "url": "https://beta.miawa.cn/download/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
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
| `clone_url` | string | 当对应 launcher 的 `mode` 为 `clone` / `all` 且当前有 release 数据时返回的 Git 克隆地址 |
| `assets[].name` | string | 资源文件名 |
| `assets[].url` | string | 构造的下载地址（受 `download_url_base` / `server_address` 配置影响） |
| `assets[].size` | int | 资源文件大小（字节） |

> 注意：接口返回的是以 `launcher` 名称为 key 的对象，而非数组。每个 key 对应的值是版本数组，按发布时间降序排列。

### 2.2 获取指定启动器状态

```
GET /api/v2/launchers/{launcher}
```

**路径参数：**

| 参数 | 说明 |
|------|------|
| `launcher` | 启动器标识，如 `fcl`、`zl`、`zl2` |

返回单个启动器的版本数组。

**响应示例（`data` 字段）：**

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
        "url": "https://beta.miawa.cn/download/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
        "size": 12345678
      }
    ]
  }
]
```

**错误场景：**

- `404 Not Found`（`error.code = not_found`）：`launcher` 不存在（在配置中未定义或无已扫描版本）

### 2.3 获取所有启动器最新版本

```
GET /api/v2/latest
```

返回每个启动器的最新版本号。**结果同时以 JSON 响应体和自定义响应头两种形式返回**，方便不同场景读取。

**响应头：** `X-Latest-Versions: {"fcl":"1.3.0.7","zl":"141400","zl2":"2.4.4"}`

**响应示例（`data` 字段）：**

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
GET /api/v2/latest/{launcher}
```

**路径参数：**

| 参数 | 说明 |
|------|------|
| `launcher` | 启动器标识 |

返回**纯文本**版本号，非 JSON，**不走统一信封**。

**响应头：** `X-Latest-Version: 1.3.0.7`

**响应示例：**

```text
1.3.0.7
```

**错误场景：**

- `404 Not Found`（`error.code = not_found`）：`launcher` 不存在（错误响应仍走信封）

### 2.5 获取站点统计信息

```
GET /api/v2/stats
```

返回访问量、下载量、Repo 拉取量、流量统计、磁盘占用、热门资源和趋势数据。

**缓存：** 响应头 `Cache-Control: public, max-age=300`（5 分钟），服务端内部有同 TTL 的内存缓存，命中时 `meta.cached = true`。

**响应示例（`data` 字段）：**

```json
{
  "total_visits": 1500,
  "total_downloads": 450,
  "total_repo_downloads": 120,
  "total_days": 15,
  "last_30_visits": 300,
  "last_30_downloads": 80,
  "last_30_repo_downloads": 26,
  "total_traffic_bytes": 5368709120,
  "total_repo_traffic_bytes": 1073741824,
  "last_30_traffic_bytes": 805306368,
  "last_30_repo_traffic_bytes": 268435456,
  "disk": {
    "total": 53687091200,
    "free": 10737418240,
    "used": 42949672960
  },
  "top_downloads": [
    {
      "launcher": "fcl",
      "count": 120
    }
  ],
  "top_repo_downloads": [
    {
      "repo_name": "fcl.git",
      "count": 66
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
      "download_count": 22,
      "repo_download_count": 7,
      "traffic_bytes": 83886080,
      "repo_traffic_bytes": 10485760
    }
  ]
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `total_visits` | int | 累计访问量（所有非下载请求） |
| `total_downloads` | int | 累计下载次数（仅 `200/206` 响应的普通下载请求） |
| `total_repo_downloads` | int | 累计 Repo 拉取次数（仅 `200/206` 响应的 `/repo/...` 请求） |
| `total_days` | int | 站点自首次启动至今的天数 |
| `last_30_visits` | int | 最近 30 天访问量 |
| `last_30_downloads` | int | 最近 30 天普通下载量 |
| `last_30_repo_downloads` | int | 最近 30 天 Repo 拉取量 |
| `total_traffic_bytes` | int | 累计普通下载流量（字节） |
| `total_repo_traffic_bytes` | int | 累计 Repo 拉取流量（字节） |
| `last_30_traffic_bytes` | int | 最近 30 天普通下载流量（字节） |
| `last_30_repo_traffic_bytes` | int | 最近 30 天 Repo 拉取流量（字节） |
| `disk.total` | int | 存储路径所在磁盘总容量（字节） |
| `disk.free` | int | 剩余可用空间（字节） |
| `disk.used` | int | 已用空间（字节） |
| `top_downloads` | array | 普通下载排行 Top 10，按启动器聚合，下载次数降序 |
| `top_downloads[].launcher` | string | 启动器标识 |
| `top_downloads[].count` | int | 下载次数 |
| `top_repo_downloads` | array | Repo 拉取排行 Top 10，按拉取次数降序 |
| `top_repo_downloads[].repo_name` | string | 仓库名（如 `fcl.git`） |
| `top_repo_downloads[].count` | int | 拉取次数 |
| `geo_distribution` | array | 地区分布 Top 50，按访问量降序，排除本地和空白记录 |
| `geo_distribution[].country` | string | 国家/地区名 |
| `geo_distribution[].count` | int | 访问次数 |
| `daily_stats` | array | 最近 30 天每日统计 |
| `daily_stats[].date` | string | 日期（`YYYY-MM-DD`） |
| `daily_stats[].visit_count` | int | 当日访问量 |
| `daily_stats[].download_count` | int | 当日普通下载量 |
| `daily_stats[].repo_download_count` | int | 当日 Repo 拉取量 |
| `daily_stats[].traffic_bytes` | int | 当日普通下载流量（字节） |
| `daily_stats[].repo_traffic_bytes` | int | 当日 Repo 拉取流量（字节） |

### 2.6 获取验证码配置

```
GET /api/v2/captcha/config
```

前端发起浏览器下载前可先读取验证码配置，判断是否需要走验证流程。

**响应示例（`data` 字段）：**

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
GET /api/v2/auth/2fa/status
```

返回当前是否启用管理员两步验证（TOTP）。

**响应示例（`data` 字段）：**

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
POST /api/v2/admin/scans
```

触发对所有已配置启动器的全量扫描。扫描为**异步**执行，接口立即返回。**需要 Admin 认证。**

**响应（`data` 字段）：**

```json
{
  "status": "accepted",
  "message": "Scan triggered"
}
```

- 状态码：`202 Accepted`
- 说明：同一时间只允许一个全量扫描运行（内部互斥锁），若已有扫描在进行中，新的触发会被忽略。
- 若扫描功能不可用（`scanAllFunc` 未注入），返回 `501 Not Implemented`（`error.code = scan_unavailable`）。

### 2.9 触发单个启动器扫描

```
POST /api/v2/admin/scans/launcher
```

触发对指定启动器的扫描。扫描为**异步**执行。**需要 Admin 认证。**

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

**成功响应（`data` 字段）：**

```json
{
  "status": "accepted",
  "message": "扫描已触发"
}
```

- 状态码：`202 Accepted`

**错误场景：**

- `400 Bad Request`（`bad_request` / `missing_required_parameters`）：请求体解析失败，或 `launcher` 为空
- `405 Method Not Allowed`（`method_not_allowed`）：使用了 GET 等非 POST 方法
- `501 Not Implemented`（`scan_unavailable`）：扫描功能不可用

---

## 3. 下载相关接口

> 本节成功响应使用统一信封包裹，以下示例仅展示 `data` 字段内容；错误示例仅展示 `error` 字段内容。

### 3.1 准备下载（无验证码）

```
POST /api/v2/downloads/prepare
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

**成功响应（`data` 字段）：**

```json
{
  "download_token": "a1b2c3d4e5f6...（64字符十六进制字符串）",
  "download_url": "/download/a1b2c3.../fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/v2/downloads/landing?token=a1b2c3..."
}
```

**错误场景：**

- `400 Bad Request`（`missing_required_parameters`）：缺少 `file_path`
- `403 Forbidden`（`invalid_path`）：文件路径包含 `..` 等非法字符，或路径不在 `storage_path` 范围内
- `404 Not Found`（`file_not_found`）：`file_path` 对应的文件在磁盘上不存在
- `500 Internal Server Error`（`token_generation_failed`）：令牌生成失败

### 3.2 获取下载引导信息

```
GET /api/v2/downloads/landing?token={token}
```

**Query 参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `token` | string | 是 | `prepare` 或 `verify` 返回的 `download_token` |

前端下载引导页使用此接口读取下载地址、来源信息和文件详情。**此接口使用 `Peek` 模式读取令牌，不消耗令牌。**

**成功响应（`data` 字段）：**

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

- `400 Bad Request` — 缺少 `token`（`error` 字段）：

```json
{
  "code": "missing_token",
  "message": "Missing token"
}
```

- `403 Forbidden` — 令牌不存在、已过期、或已被下载消费：

```json
{
  "code": "expired_token",
  "message": "Download token is invalid or expired"
}
```

### 3.3 验证后生成下载令牌

```
POST /api/v2/downloads/verify
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

**成功响应（`data` 字段）：** 与 `prepare` 接口一致

```json
{
  "download_token": "a1b2c3d4e5f6...",
  "download_url": "/download/a1b2c3.../fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/v2/downloads/landing?token=a1b2c3..."
}
```

**错误场景：**

- `400 Bad Request`
  - 验证码未启用（`captcha_not_enabled`）
  - 任一极验参数（`lot_number`、`captcha_output`、`pass_token`、`gen_time`）或 `file_path` 为空（`missing_required_parameters`）
  - 请求体解析失败（`bad_request`）
- `403 Forbidden` — 极验服务端验证不通过（`verification_failed`）：

```json
{
  "code": "verification_failed",
  "message": "极验返回的拒绝原因"
}
```

- `404 Not Found`（`file_not_found`）— `file_path` 对应文件不存在
- `500 Internal Server Error`（`verification_failed` / `token_generation_failed`）— 与极验服务端通信失败或令牌生成失败

```json
{
  "code": "verification_failed",
  "message": "Failed to verify captcha"
}
```

### 3.4 真实文件下载

```
GET /download/{token}/{file_path}
```

返回真实文件流。token 也可以放在 Query 参数中：`GET /download/{file_path}?token={token}`。

此接口使用 `Validate` 模式消费令牌——**每个令牌仅允许一次成功下载**。该接口直接返回文件流，不走统一信封。

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

> 注：`/download/...` 的错误响应为兼容浏览器下载场景，使用扁平 `{error, message, ...}` 结构，不走 v2 统一信封。

**支持的 HTTP 特性：**
- `Range` 请求（断点续传）：响应 `206 Partial Content`
- 流量预估：根据 `Range` 头计算预估传输字节数，用于流量限制预检

---

## 4. 浏览器下载流程

### 4.1 验证码关闭时的流程

```
前端                      服务端
 │                          │
 │  POST /api/v2/downloads/prepare
 │  { file_path, ... }      │
 │ ─────────────────────────>
 │                          │  生成 token（Flow: "prepare"）
 │  { download_token,       │
 │    download_url,         │
 │    landing_url }         │
 │ <─────────────────────────
 │                          │
 │  GET /api/v2/downloads/landing?token=...
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
 │  GET /api/v2/captcha/config
 │ ─────────────────────────>
 │  { enabled, app_id }     │
 │ <─────────────────────────
 │                          │
 │  （用户完成极验验证）      │
 │ ────────────────────────────────────────────────────>
 │ <────────── lot_number, captcha_output, pass_token, gen_time
 │                          │
 │  POST /api/v2/downloads/verify
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
 │  GET /api/v2/downloads/landing?token=...
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

统一信封中 `error.code` 字段使用以下标准化错误码：

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `bad_request` | 400 | 请求体解析失败 |
| `missing_token` | 400 | 缺少 `token` 参数 |
| `missing_required_parameters` | 400 | 缺少必填字段（`file_path` 等） |
| `missing_param` | 400 | 缺少查询参数（如 `ip`、`path`） |
| `captcha_not_enabled` | 400 | 验证码未启用 |
| `invalid_config` | 400 | 配置校验不通过 |
| `unauthorized` | 401 | 未提供或提供了无效的管理员 token |
| `invalid_credentials` | 401 | 用户名或密码错误 |
| `otp_required` | 401 | 需要两步验证码 |
| `invalid_otp` | 401 | 两步验证码错误 |
| `expired_token` | 403 | 下载令牌不存在、已过期、或已被消费 |
| `invalid_token` | 403 | 令牌无效（下载接口，扁平错误体） |
| `token_mismatch` | 403 | 令牌绑定的文件与当前请求路径不一致 |
| `verification_required` | 403 | 需要先完成验证码，无有效令牌（下载接口，扁平错误体） |
| `verification_failed` | 403 | 极验服务端验证不通过 |
| `invalid_path` | 403 | 文件路径包含非法字符（如 `..`）或超出存储目录 |
| `forbidden` | 403 | 路径越界或无权限访问 |
| `admin_disabled` | 403 | 管理后台未启用 |
| `account_locked` | 403 | 登录失败次数过多，账号已锁定 |
| `file_not_found` | 404 | 请求的文件在磁盘上不存在 |
| `not_found` | 404 | 资源不存在（启动器、目录等） |
| `method_not_allowed` | 405 | 请求方法不正确 |
| `internal_error` | 500 | 服务端内部错误 |
| `token_generation_failed` | 500 | 下载令牌生成失败 |
| `hash_failed` | 500 | 密码哈希失败 |
| `save_failed` | 500 | 配置保存失败 |
| `restart_failed` | 500 | 进程重启失败 |
| `admin_not_configured` | 500 | 管理员账号未配置 |
| `scan_unavailable` | 501 | 扫描功能不可用 |
| `not_configured` | 501 | 自更新/重启功能未配置 |
| `check_failed` | 502 | 自更新检查失败 |
| `apply_failed` | 502 | 自更新应用失败（`error.details.status` 携带最新状态） |

---

## 6. 扫描流程说明

### 6.1 定时扫描

由 `check_cron` 表达式控制（默认每 10 分钟），自动对所有已配置启动器执行：
1. 从 `source_url` 解析 GitHub 仓库地址
2. 根据 `mode` 决定同步内容：
   - `release`：仅同步 Release 版本与资源文件
   - `clone`：仅同步 Git 镜像到 `repo/{launcher}.git`
   - `all`：同时执行 Release 同步与 Git 镜像同步
3. `release` / `all` 模式下，按 `include_prerelease` 和 `max_versions` 策略拉取 release 列表
4. `release` / `all` 模式下，下载资源文件到 `storage_path/{launcher}/{version}/`
5. `release` / `all` 模式下，写入 `index.json` 版本元数据并清理旧版本目录
6. `clone` / `all` 模式下，执行 `git update-server-info`，供 HTTP clone/fetch 使用

### 6.2 手动扫描

- `POST /api/v2/admin/scans`：全量扫描所有启动器
- `POST /api/v2/admin/scans/launcher`：扫描指定启动器

两者均为异步执行，立即返回 `202 Accepted`。同一时间只允许一个全量扫描运行（互斥锁保护）。手动扫描同样遵循 launcher 的 `mode` 配置。两个接口均需要 Admin 认证。

### 6.3 版本保留规则

| `max_versions` 配置值 | 实际保留版本数 |
|----------------------|---------------|
| `N > 0` | 保留最近 N 个版本 |
| `N = 0` | 使用默认值 **3** |
| `N < 0` | 使用默认值 **3** |

---

## 7. 注意事项

- **`GET /api/v2/latest/{launcher}` 返回纯文本**，不是 JSON，不走统一信封。解析时不要调用 `JSON.parse()`，直接读取响应文本。
- **所有其它 JSON 接口均使用 `{data, error, meta}` 统一信封**，解析时先判断 `error` 是否为 `null`。
- **`/download/...` 接口对 token 的两种操作不同：**
  - `landing` 用 `Peek`（不消费，可多次调用）
  - 实际下载用 `Validate`（消费，一次性，用完即删）
- **`/download/...` 的错误响应使用扁平 `{error, message, ...}` 结构**，不走统一信封，以兼容浏览器下载场景。
- **流量限制在下载前做预估**（基于 `Range` 头），下载后按实际字节数精确记录。预估超限和实际超限都会导致封禁。
- **`return_url` 和 `source` 完全由调用方传入**，服务端不会自动推断来源站点。
- **`download_url_base` 为空时**，服务端会自动尝试使用 `server_address`，若也为空则通过 `ifconfig.me/ip` 获取公网 IP 作为下载地址 host。
- **`max_versions = 0` 等价于 `max_versions = 3`**（由 `NormalizeMaxVersions` 函数统一处理，≤0 均修正为 3）。
- **`traffic_limit_gb` 负值会自动修正为 5** GB，`0` 表示完全禁用。
- **扫描接口与管理后台接口需要 Admin 认证**，调用前需先通过 `/api/v2/auth/login` 登录获取 token。
- 站内 API 速查页（`/api`）用于快速浏览与复制示例，**正式接入请以本文档为准**。

---

## 8. 管理后台接口

> 本节所有 JSON 响应均使用 1.10 节定义的统一信封包裹，以下示例仅展示 `data` 字段内容；错误示例仅展示 `error` 字段内容。

### 8.1 认证说明

所有 `/api/v2/admin/*` 接口（以及 `/api/v2/admin/scans*` 扫描接口）均需通过 Admin 认证，认证流程如下：

1. **登录获取 token：**

```
POST /api/v2/auth/login
```

**请求体：**

```json
{
  "username": "admin",
  "password": "your_password",
  "otp_code": "123456"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 管理员用户名 |
| `password` | string | 是 | 管理员密码 |
| `otp_code` | string | 否 | 两步验证码（启用 2FA 时必填） |

**成功响应（`data` 字段）：**

```json
{
  "token": "a1b2c3d4e5f6...（管理员 token）"
}
```

**错误场景：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `bad_request` | 400 | 请求体解析失败 |
| `invalid_credentials` | 401 | 用户名或密码错误 |
| `otp_required` | 401 | 启用了 2FA 但未提供 `otp_code` |
| `invalid_otp` | 401 | 两步验证码错误 |
| `account_locked` | 403 | 登录失败次数过多，账号已锁定（锁定时长由 `admin_lock_duration` 控制） |
| `admin_not_configured` | 500 | 管理员账号未配置 |
| `token_generation_failed` | 500 | token 生成失败 |

2. **携带 token 访问后台接口：** 后续请求通过以下任一方式携带 token：
   - 请求头：`Authorization: Bearer <token>`（或直接 `Authorization: <token>`）
   - Cookie：`admin_token=<token>`

   token 无效或缺失时返回 `401 Unauthorized`（`error.code = unauthorized`）。

3. **后台开关：** 若配置 `admin_enabled = false`，所有后台接口（含登录）返回 `403 Forbidden`（`error.code = admin_disabled`）。

### 8.2 配置管理

```
GET  /api/v2/admin/config     # 获取当前配置
POST /api/v2/admin/config     # 更新配置
```

**GET 响应（`data` 字段）：** 返回完整配置对象（结构与 `config.yaml` 一致），其中 `admin_password` 字段始终为空字符串（不返回密码哈希）。

**POST 请求体：** 完整配置对象（结构与 `config.yaml` 一致）。

- 若 `admin_password` 为空，保持原密码不变；
- 若 `admin_password` 非空，则视为新密码，服务端会自动哈希后保存；
- 配置会经过 `NormalizeConfig` 校验与归一化，保存到 `config.yaml` 并热更新到运行时（含自更新模块配置）。

> **配置文件迁移**：服务启动时若检测到旧版 `config.json`（5月及之前使用的格式），会自动迁移至 `config.yaml` 并删除旧文件。迁移过程中会自动补全新增字段的默认值（如 `launcher.mode`、`self_update_*` 等）。已废弃的 `api_version` 字段会在迁移时从文件中移除。

**POST 成功响应（`data` 字段）：**

```json
{
  "message": "Config updated"
}
```

**错误场景：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `bad_request` | 400 | 请求体解析失败 |
| `invalid_config` | 400 | 配置校验不通过（`error.message` 含具体原因） |
| `hash_failed` | 500 | 密码哈希失败 |
| `save_failed` | 500 | 配置保存失败 |
| `method_not_allowed` | 405 | 非 GET/POST 方法 |

### 8.3 黑名单管理

```
GET    /api/v2/admin/blacklist           # 获取黑名单列表
POST   /api/v2/admin/blacklist           # 新增黑名单条目
DELETE /api/v2/admin/blacklist?ip={ip}   # 移除黑名单条目
```

**GET 响应（`data` 字段）：** 黑名单数组，按 `created_at` 降序。

```json
[
  {
    "ip": "1.2.3.4",
    "reason": "流量超限",
    "source": "traffic",
    "ban_type": "local",
    "created_at": "2026-06-18 12:00:00"
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `ip` | string | 被封禁的 IP |
| `reason` | string | 封禁原因 |
| `source` | string | 来源（如 `traffic`、`manual`、`external`） |
| `ban_type` | string | 封禁类型 |
| `created_at` | string | 封禁时间 |

**POST 请求体：**

```json
{
  "ip": "1.2.3.4",
  "reason": "手动封禁"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ip` | string | 是 | 待封禁的 IP |
| `reason` | string | 否 | 封禁原因 |

**POST 成功响应（`data` 字段）：** 状态码 `201 Created`，并同步封禁记录到内存。

```json
{
  "message": "added"
}
```

**DELETE：** 通过 Query 参数 `ip` 指定待移除的 IP，成功后同步解封记录。

**DELETE 成功响应（`data` 字段）：**

```json
{
  "message": "removed"
}
```

**错误场景：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `bad_request` | 400 | 请求体解析失败（POST） |
| `missing_param` | 400 | DELETE 缺少 `ip` 参数 |
| `internal_error` | 500 | 数据库操作失败 |
| `method_not_allowed` | 405 | 非 GET/POST/DELETE 方法 |

### 8.4 文件管理

```
GET    /api/v2/admin/files?path={path}   # 列出目录内容
POST   /api/v2/admin/files?path={path}   # 上传文件
DELETE /api/v2/admin/files?path={path}   # 删除文件/目录
```

`path` 为相对于 `storage_path` 的相对路径。服务端会校验解析后的绝对路径必须位于 `storage_path` 之下，禁止路径穿越。

**GET 响应（`data` 字段）：** 指定目录下的条目数组。

```json
[
  {
    "name": "fcl",
    "is_dir": true,
    "size": 0,
    "mod_time": "2026-06-18T12:00:00Z"
  },
  {
    "name": "FCL-release-1.3.0.7-all.apk",
    "is_dir": false,
    "size": 12345678,
    "mod_time": "2026-06-18T12:00:00Z"
  }
]
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 条目名称 |
| `is_dir` | bool | 是否为目录 |
| `size` | int | 字节数（目录通常为 0） |
| `mod_time` | string | 最后修改时间（RFC 3339） |

**POST：** 以 `multipart/form-data` 上传，表单字段名为 `file`。`path` 指定上传目标完整路径（含文件名），服务端会自动创建所需父目录。

**POST 成功响应（`data` 字段）：**

```json
{
  "message": "File uploaded"
}
```

**DELETE：** 删除 `path` 指定的文件或目录（递归删除）。

**DELETE 成功响应（`data` 字段）：**

```json
{
  "message": "deleted"
}
```

**错误场景：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `missing_param` | 400 | 缺少 `path` 参数 |
| `bad_request` | 400 | 上传时获取文件失败 |
| `forbidden` | 403 | 路径越界（不在 `storage_path` 下） |
| `not_found` | 404 | 目录不存在（GET） |
| `internal_error` | 500 | 文件系统操作失败 |
| `method_not_allowed` | 405 | 非 GET/POST/DELETE 方法 |

### 8.5 文件下载

```
GET /api/v2/admin/files/download?path={path}
```

下载 `storage_path` 下的指定文件，**返回二进制流，不走统一信封**。

- 响应头：`Content-Type: application/octet-stream`、`Content-Disposition: attachment; filename="<文件名>"`
- 支持 `Range` 请求（断点续传）
- `path` 必须指向文件（非目录），且解析后绝对路径必须位于 `storage_path` 之下

**错误场景（错误响应走统一信封）：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `missing_param` | 400 | 缺少 `path` 参数 |
| `forbidden` | 403 | 路径越界 |
| `not_found` | 404 | 文件不存在或为目录 |
| `method_not_allowed` | 405 | 非 GET 方法 |

### 8.6 自更新管理

```
GET  /api/v2/admin/self-update/status    # 查询自更新状态
POST /api/v2/admin/self-update/check     # 检查是否有可用更新
POST /api/v2/admin/self-update/apply     # 应用已下载的更新
POST /api/v2/admin/self-update/restart   # 重启进程以应用更新
POST /api/v2/admin/self-update           # 触发异步更新检查（后台运行）
```

**GET status 响应（`data` 字段）：**

```json
{
  "enabled": true,
  "repo_url": "https://github.com/owner/repo",
  "channel": "stable",
  "current_version": "1.0.0",
  "latest_version": "1.1.0",
  "has_update": true,
  "can_apply": true,
  "pending_restart": false,
  "last_checked_at": "2026-06-18T12:00:00Z",
  "last_applied_at": "2026-06-18T11:00:00Z",
  "last_check_error": "",
  "last_apply_error": "",
  "last_apply_message": "",
  "available_versions": [
    {
      "name": "v1.1.0",
      "stable": true,
      "published": "2026-06-17T00:00:00Z"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | bool | 是否启用自更新 |
| `repo_url` | string | 更新源仓库地址 |
| `channel` | string | 更新通道（如 `stable`、`beta`） |
| `current_version` | string | 当前运行版本 |
| `latest_version` | string | 最新可用版本 |
| `has_update` | bool | 是否存在可用更新 |
| `can_apply` | bool | 是否可以应用更新（已下载完毕） |
| `pending_restart` | bool | 是否等待重启以生效 |
| `last_checked_at` | string | 上次检查时间 |
| `last_applied_at` | string | 上次应用时间 |
| `last_check_error` | string | 上次检查错误信息（无则省略） |
| `last_apply_error` | string | 上次应用错误信息（无则省略） |
| `last_apply_message` | string | 上次应用附加信息（无则省略） |
| `available_versions` | array | 可用版本列表（无则省略） |
| `available_versions[].name` | string | 版本标签 |
| `available_versions[].stable` | bool | 是否为稳定版 |
| `available_versions[].published` | string | 发布时间 |

**POST check：** 同步检查上游是否有新版本，返回最新的 `status` 对象。

**POST apply：** 应用已下载的更新包，返回最新的 `status` 对象。

**POST restart：** 重启当前进程以使更新生效。

```json
{
  "status": "accepted",
  "message": "重启请求已触发"
}
```

**POST /api/v2/admin/self-update：** 异步触发一次更新检查，立即返回 `202 Accepted`。

```json
{
  "status": "accepted",
  "message": "Self update check triggered"
}
```

**错误场景：**

| 错误码 | HTTP 状态码 | 含义 |
|--------|------------|------|
| `method_not_allowed` | 405 | 请求方法不正确（status 仅 GET，其余仅 POST） |
| `not_configured` | 501 | 自更新/重启功能未配置（`selfUpdate` 为空） |
| `check_failed` | 502 | 检查更新失败（`error.message` 含原因） |
| `apply_failed` | 502 | 应用更新失败（`error.details.status` 携带最新状态） |
| `restart_failed` | 500 | 重启进程失败 |
