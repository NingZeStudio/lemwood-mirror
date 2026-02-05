# Lemwood Mirror API 详细技术文档

本文档提供了 Lemwood Mirror 系统的完整 API 参考，涵盖了公共访问接口和受保护的后台管理接口。

---

## 1. 核心设计与安全规范

### 1.1 认证机制
- **认证方式**：基于 Bearer Token。
- **Token 获取**：通过 `POST /api/login` 接口。
- **Token 有效期**：24 小时。
- **使用方式**：在请求头中携带 `Authorization: <token>`，或者在 Cookie 中携带 `admin_token=<token>`。

### 1.2 安全中间件
所有 API 请求均经过安全中间件处理：
- **IP 黑名单**：拦截 `ip_blacklist` 表中的 IP。
- **路径遍历保护**：禁止包含 `..` 的路径请求。
- **CORS 支持**：支持跨域访问，允许 `GET, POST, OPTIONS` 方法。
- **访问统计**：自动记录所有有效请求的 IP、路径、UA 和地理位置。

### 1.3 响应头
- `X-Latest-Versions`: 仅在 `/api/latest` 返回，包含所有启动器最新版本的 JSON。
- `X-Latest-Version`: 仅在 `/api/latest/{id}` 返回，包含该启动器的最新版本号。

---

## 2. 身份认证接口

### 2.1 管理员登录
- **端点**：`POST /api/login`
- **请求体**：
  ```json
  {
    "username": "admin",
    "password": "your_password"
  }
  ```
- **响应示例** (200 OK)：
  ```json
  {
    "token": "4e7...a3f"
  }
  ```
- **错误码**：
  - `400 Bad Request`: 请求体格式错误。
  - `401 Unauthorized`: 用户名或密码错误。
  - `500 Internal Server Error`: 服务器配置错误或 Token 生成失败。

---

## 3. 公共查询接口

### 3.1 获取所有启动器状态
- **端点**：`GET /api/status`
- **功能**：返回所有启动器的所有版本详细信息，按版本号降序排列。
- **响应示例**：
  ```json
  {
    "fcl": [
      {
        "tag_name": "1.2.7.1",
        "launcher": "fcl",
        "name": "1.2.7.1",
        "published_at": "2025-11-21T12:00:00Z",
        "assets": [
          {
            "name": "FCL-release-1.2.7.1-all.apk",
            "size": 238526949,
            "url": "https://mirror.lemwood.icu/download/fcl/1.2.7.1/FCL-release-1.2.7.1-all.apk"
          }
        ]
      }
    ]
  }
  ```

### 3.2 获取指定启动器状态
- **端点**：`GET /api/status/{launcher_id}`
- **示例**：`GET /api/status/fcl`
- **响应示例**：返回该启动器的版本数组（结构同上）。

### 3.3 获取所有最新版本
- **端点**：`GET /api/latest`
- **响应示例**：
  ```json
  {
    "fcl": "1.2.7.1",
    "hmcl": "v3.8.1",
    "zl": "141100"
  }
  ```
- **响应头**：包含 `X-Latest-Versions`。

### 3.4 获取指定启动器最新版本
- **端点**：`GET /api/latest/{launcher_id}`
- **响应内容**：纯文本版本号，如 `1.2.7.1`。
- **响应头**：包含 `X-Latest-Version`。

### 3.5 获取系统统计数据
- **端点**：`GET /api/stats`
- **功能**：返回访问量、下载量、排行、地域分布等。
- **响应示例**：
  ```json
  {
    "total_visits": 18223,
    "total_downloads": 2690,
    "top_downloads": [{"file_name": "...", "count": 120}],
    "geo_distribution": [{"city": "Beijing", "count": 500}],
    "daily_stats": [{"date": "2026-02-05", "visits": 100, "downloads": 20}]
  }
  ```

---

## 4. 后台管理接口 (需认证)

### 4.1 配置文件管理
- **获取配置**：`GET /api/admin/config`
  - 响应：返回 `config.json` 的所有字段（`admin_password` 会被清空以保安全）。
- **更新配置**：`POST /api/admin/config`
  - 请求体：完整的配置 JSON。如果 `admin_password` 不为空，则会重新进行 Bcrypt 哈希存储。

### 4.2 IP 黑名单管理
- **获取黑名单**：`GET /api/admin/blacklist`
  - 响应：`[{"ip": "1.2.3.4", "reason": "...", "created_at": "..."}]`
- **添加黑名单**：`POST /api/admin/blacklist`
  - 请求体：`{"ip": "1.1.1.1", "reason": "恶意攻击"}`
- **移除黑名单**：`DELETE /api/admin/blacklist?ip=1.1.1.1`

### 4.3 文件系统管理
- **列表查询**：`GET /api/admin/files?path=launcher/fcl`
  - 响应：`[{"name": "v1.0.0", "is_dir": true, "size": 0, "mod_time": "..."}]`
- **上传文件**：`POST /api/admin/files?path=relative/path/to/file.exe`
  - **参数**：`path` 为相对于下载根目录的路径。
  - **请求体**：使用 `multipart/form-data` 格式，文件字段名为 `file`。
  - **特性**：如果文件已存在，将自动**覆盖**；如果父目录不存在，将自动**递归创建**。
- **删除文件/目录**：`DELETE /api/admin/files?path=relative/path`
  - **注意**：目录将递归删除所有内容。禁止删除下载根目录。
- **安全下载**：`GET /api/admin/files/download?path=relative/path/to/file`
  - 响应：带 `Content-Disposition: attachment` 的文件流。

---

## 5. 版本判定逻辑说明

系统自动判定“最新稳定版”的规则如下：
1. **显式标记**：如果版本信息的 `index.json` 中 `is_latest` 字段为 `true`，则优先选中。
2. **稳定性优先**：排除包含 `alpha, beta, rc, snapshot, pre, dev` 的版本。
3. **版本号比较**：
   - 移除前缀 `v`。
   - 按点分隔符 `.` 拆分段落。
   - 每一段尝试作为整数比较。
   - 如果数字相同，按字符串逐位比较。
   - 例如：`1.2.10` > `1.2.2`；`2.0.0` > `2.0.0_beta`。

---

## 6. 状态码参考

| 状态码 | 含义 | 常见场景 |
| :--- | :--- | :--- |
| 200 | OK | 请求成功，通常伴随 JSON 数据 |
| 201 | Created | 资源（如黑名单 IP）创建成功 |
| 400 | Bad Request | 请求参数缺失或格式不正确 |
| 401 | Unauthorized | 未登录或 Token 已过期 |
| 403 | Forbidden | 权限不足（如试图访问系统目录）或 IP 被封禁 |
| 404 | Not Found | 资源不存在（启动器 ID 错误或路径无效） |
| 500 | Internal Error | 服务器内部错误（数据库异常或文件系统故障） |
| 501 | Not Implemented | 功能尚未实现 |
