# 柠枺镜像 API 文档

本文档只覆盖公共查询接口和下载相关接口。
后台管理接口、登录接口和手动扫描接口不在本文档范围内。

## 1. 基础约定

### 1.1 Base URL

- 站内调用通常使用相对路径：`/api/...`
- 外部调用可拼接你的站点域名，例如：`https://mirror.example.com/api`

### 1.2 内容类型

- 查询接口通常返回 `application/json`
- `GET /api/latest/{launcher}` 返回纯文本
- 真实文件下载由 `/download/...` 返回文件流

### 1.3 常见状态码

- `200 OK`：请求成功
- `400 Bad Request`：缺少参数或参数格式错误
- `403 Forbidden`：下载令牌无效、验证失败或触发流量限制
- `404 Not Found`：启动器或文件不存在
- `500 Internal Server Error`：服务端执行失败

### 1.4 下载令牌

- `download_token` 默认有效期为 5 分钟
- `landing` 接口只读取 token，不消耗 token
- `/download/...` 在验证码开启时会消费 token

## 2. 公共查询接口

### 2.1 获取所有启动器状态

- 方法：`GET`
- 路径：`/api/status`
- 说明：返回所有启动器的版本列表，按版本从新到旧排序。

响应示例：

```json
{
  "fcl": [
    {
      "launcher": "fcl",
      "tag_name": "1.3.0.7",
      "name": "1.3.0.7",
      "published_at": "2024-01-01T00:00:00Z",
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

### 2.2 获取指定启动器状态

- 方法：`GET`
- 路径：`/api/status/{launcher}`
- 路径参数：
  - `launcher`：启动器标识，例如 `fcl`、`zl`、`zl2`
- 说明：返回单个启动器的版本列表。

响应示例：

```json
[
  {
    "launcher": "fcl",
    "tag_name": "1.3.0.7",
    "name": "1.3.0.7",
    "published_at": "2024-01-01T00:00:00Z",
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

错误场景：

- `404 Not Found`：启动器不存在

### 2.3 获取所有启动器最新版本

- 方法：`GET`
- 路径：`/api/latest`
- 说明：返回每个启动器的最新版本号。
- 响应头：`X-Latest-Versions`

响应示例：

```json
{
  "fcl": "1.3.0.7",
  "zl": "141400",
  "zl2": "2.4.4"
}
```

### 2.4 获取指定启动器最新版本

- 方法：`GET`
- 路径：`/api/latest/{launcher}`
- 路径参数：
  - `launcher`：启动器标识
- 说明：返回纯文本版本号，而不是 JSON 对象。
- 响应头：`X-Latest-Version`

响应示例：

```text
1.3.0.7
```

错误场景：

- `404 Not Found`：启动器不存在

### 2.5 获取站点统计信息

- 方法：`GET`
- 路径：`/api/stats`
- 说明：返回访问量、下载量、磁盘占用、热门资源和趋势数据。
- 缓存：服务端会返回 `Cache-Control: public, max-age=300`

响应示例：

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

### 2.6 获取验证码配置

- 方法：`GET`
- 路径：`/api/captcha/config`
- 说明：前端在发起浏览器下载前可先读取验证码配置。

响应示例：

```json
{
  "enabled": true,
  "app_id": "your_captcha_id"
}
```

字段说明：

- `enabled`：是否启用验证码
- `app_id`：验证码前端初始化所需的应用 ID

## 3. 下载相关接口

### 3.1 准备下载

- 方法：`POST`
- 路径：`/api/download/prepare`
- 说明：在不需要验证码时，浏览器可先调用该接口生成下载 token 和 landing 地址。

请求体：

```json
{
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "https://example.com/back",
  "source": "home-latest-download"
}
```

字段说明：

- `file_path`：必填，目标文件相对路径
- `return_url`：可选，下载完成后前端可用于跳转来源页面
- `source`：可选，自定义来源标记

成功响应：

```json
{
  "download_token": "32-byte-random-token",
  "download_url": "/download/32-byte-random-token/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/download/landing?token=32-byte-random-token"
}
```

错误场景：

- `400 Bad Request`：缺少 `file_path`
- `403 Forbidden`：文件路径非法
- `404 Not Found`：文件不存在

### 3.2 获取下载引导信息

- 方法：`GET`
- 路径：`/api/download/landing`
- Query：
  - `token`：必填，下载 token
- 说明：前端单独页面可用它读取下载地址、来源信息和文件名。

请求示例：

```text
GET /api/download/landing?token=32-byte-random-token
```

成功响应：

```json
{
  "download_url": "/download/32-byte-random-token/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "https://example.com/back",
  "source": "home-latest-download",
  "file_name": "FCL-release-1.3.0.7-all.apk",
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "flow": "prepare"
}
```

`flow` 可能值：

- `prepare`：来自无需验证码的准备流程
- `verify`：来自验证码验证通过后的流程

错误场景：

- `400 Bad Request`

```json
{
  "error": "missing_token",
  "message": "Missing token"
}
```

- `403 Forbidden`

```json
{
  "error": "expired_token",
  "message": "Download token is invalid or expired"
}
```

### 3.3 验证后生成下载 token

- 方法：`POST`
- 路径：`/api/download/verify`
- 说明：在验证码开启时，前端完成验证后调用此接口。

请求体：

```json
{
  "lot_number": "e2f0a767a0f74926bbc8daeed22e6f27",
  "captcha_output": "captcha_output",
  "pass_token": "pass_token",
  "gen_time": "1709551234",
  "file_path": "fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "return_url": "https://example.com/back",
  "source": "verify-download"
}
```

字段说明：

- `lot_number`、`captcha_output`、`pass_token`、`gen_time`：验证码前端 SDK 返回的参数
- `file_path`：必填，目标文件相对路径
- `return_url`：可选
- `source`：可选

成功响应：

```json
{
  "download_token": "32-byte-random-token",
  "download_url": "/download/32-byte-random-token/fcl/1.3.0.7/FCL-release-1.3.0.7-all.apk",
  "landing_url": "/api/download/landing?token=32-byte-random-token"
}
```

错误场景：

- `400 Bad Request`
  - 验证码未启用
  - 缺少必填字段
- `403 Forbidden`

```json
{
  "error": "verification_failed",
  "message": "captcha validation failed reason"
}
```

- `500 Internal Server Error`

```json
{
  "error": "verification_failed",
  "message": "Failed to verify captcha"
}
```

### 3.4 真实文件下载

- 方法：`GET`
- 路径：`/download/{token}/{file_path}`
- 说明：返回真实文件流。

在验证码关闭时：

- 可直接访问真实文件路径
- 前端通常仍建议走 `prepare -> landing -> download` 流程

在验证码开启时：

- 浏览器直接访问无 token 的 `/download/...`，服务端会返回验证页面
- 非浏览器请求直接访问无 token 的 `/download/...`，服务端会返回 JSON 错误：

```json
{
  "error": "verification_required",
  "message": "Download requires captcha verification",
  "captcha": true,
  "app_id": "your_captcha_id"
}
```

无效 token 错误：

```json
{
  "error": "invalid_token",
  "message": "Download token is invalid or expired",
  "captcha": true,
  "app_id": "your_captcha_id"
}
```

token 与文件路径不匹配时：

```json
{
  "error": "token_mismatch",
  "message": "Download token does not match requested file"
}
```

## 4. 浏览器下载流程

### 4.1 验证码关闭

1. 前端发起 `POST /api/download/prepare`
2. 读取返回的 `landing_url`
3. 下载引导页调用 `GET /api/download/landing`
4. 页面自动触发 `/download/...`

### 4.2 验证码开启

1. 前端先调用 `GET /api/captcha/config`
2. 用户完成验证码
3. 前端调用 `POST /api/download/verify`
4. 下载引导页调用 `GET /api/download/landing`
5. 页面自动触发 `/download/...`

## 5. 错误码速查

| 错误码 | 含义 |
| --- | --- |
| `missing_required_parameters` | 缺少必填参数 |
| `missing_token` | 缺少下载 token |
| `expired_token` | token 已过期或不存在 |
| `verification_required` | 当前下载需要先完成验证码 |
| `verification_failed` | 验证码校验失败 |
| `invalid_token` | token 无效或已过期 |
| `token_mismatch` | token 对应文件与当前请求不一致 |
| `file_not_found` | 请求的文件不存在 |
| `invalid_path` | 请求路径非法 |

## 6. 注意事项

- `GET /api/latest/{launcher}` 返回纯文本，不是 JSON。
- `max_versions = 0` 的当前语义是“使用默认值 3”，不是“仅最新版本”。
- `return_url` 和 `source` 由前端显式传入，服务端不会自动推断来源站点。
- 站内 API 速查页用于快速浏览与复制示例，正式接入请以本文档为准。
