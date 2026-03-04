# Lemwood Mirror API 详细技术文档

本文档提供了 Lemwood Mirror 系统的完整 API 参考，涵盖了公共访问接口和受保护的后台管理接口。

---

## 1. 核心设计与安全规范

### 1.1 认证机制
- **认证方式**：基于 Bearer Token。
- **Token 获取**：通过 `POST /api/login` 接口，支持基于 TOTP 的两步验证 (2FA)。
- **两步验证 (TOTP)**：兼容 Microsoft Authenticator、Google Authenticator 等标准验证器。
- **安全加固**：建议在 **HTTPS** 环境下运行，以确保登录凭证和 TOTP 密钥的安全传输。
- **Token 有效期**：24 小时。
- **使用方式**：在请求头中携带 `Authorization: <token>`，或者在 Cookie 中携带 `admin_token=<token>`。

### 1.2 登录流程
1. **验证 2FA 状态** (可选)：客户端调用 `GET /api/auth/2fa/status` 检查是否启用了双重验证。
2. **提交凭据**：将用户名、密码及可选的 TOTP 验证码发送至 `POST /api/login`。
3. **后端验证**：服务器验证用户名并使用 **Bcrypt**（Cost: 14）对密码进行哈希比对。
4. **颁发令牌**：验证通过后，服务器返回认证 Token，客户端将其存储在 `localStorage` 或 Cookie 中。

### 1.3 安全中间件
所有 API 请求均经过安全中间件处理：
- **IP 黑名单**：拦截 `ip_blacklist` 表中的 IP。
- **路径遍历保护**：禁止包含 `..` 的路径请求。
- **CORS 支持**：支持跨域访问，允许 `GET, POST, OPTIONS` 方法。
- **访问统计**：自动记录所有有效请求的 IP、路径、UA 和地理位置。

---

## 2. 身份认证接口

### 2.1 管理员登录
- **端点**：`POST /api/login`
- **请求体**：
  ```json
  {
    "username": "admin",
    "password": "your_password",
    "otp_code": "123456" 
  }
  ```
- **响应示例** (200 OK)：
  ```json
  {
    "token": "4e7...a3f"
  }
  ```
- **安全特性**：
  - **暴力破解防护**：同一 IP 连续登录失败（包括密码错误和验证码错误）达到上限（默认 10 次）将被锁定。
  - **锁定时长**：锁定时间默认为 120 分钟（2 小时）。

### 2.2 获取 2FA 状态
- **端点**：`GET /api/auth/2fa/status`
- **功能**：查询当前系统是否启用了两步验证，供登录页 UI 适配。
- **响应示例**：
  ```json
  {
    "enabled": true
  }
  ```

---

## 3. 公共查询接口

### 3.1 获取所有启动器状态
- **端点**：`GET /api/status`
- **功能**：返回所有启动器的所有版本详细信息。

### 3.2 获取指定启动器状态
- **端点**：`GET /api/status/{launcher_id}`
- **功能**：返回指定启动器的所有版本详细信息。

### 3.3 获取所有启动器最新版本
- **端点**：`GET /api/latest`
- **功能**：返回所有启动器的最新稳定版本号。
- **响应头**：`X-Latest-Versions`

### 3.4 获取指定启动器最新版本
- **端点**：`GET /api/latest/{launcher_id}`
- **功能**：返回指定启动器的最新稳定版本号（纯文本）。
- **响应头**：`X-Latest-Version`

### 3.5 获取系统统计信息
- **端点**：`GET /api/stats`
- **功能**：返回系统访问量、下载量、运行时间及磁盘占用等统计数据。
- **响应格式**：
  ```json
  {
    "total_visits": 1500,        // 总访问量
    "total_downloads": 450,      // 总下载量
    "total_days": 15,            // 系统累计运行天数
    "last_30_visits": 300,       // 最近 30 天访问量
    "last_30_downloads": 80,     // 最近 30 天下载量
    "disk": {
      "total": 53687091200,      // 磁盘总空间 (Bytes)
      "free": 10737418240,       // 磁盘剩余空间 (Bytes)
      "used": 42949672960        // 磁盘已用空间 (Bytes)
    },
    "top_downloads": [...],      // 热门资源排行
    "geo_distribution": [...],   // 地理位置分布
    "daily_stats": [...]         // 每日趋势数据
  }
  ```

---

## 4. 后台管理接口 (需认证)

### 4.1 获取/更新系统配置
- **端点**：`GET/POST /api/admin/config`
- **GET**：返回脱敏后的系统配置（不包含密码哈希和 TOTP 密钥）。
- **POST**：更新系统配置。支持修改管理员密码、GitHub Token、TOTP 配置等。
- **TOTP 设置流程**：
  1. 生成新密钥：前端随机生成 Base32 字符串并显示二维码。
  2. 保存配置：用户确认后点击保存，密钥被持久化到服务器。
  3. 启用验证：勾选“启用两步验证”并保存，后续登录将强制要求验证码。

### 4.2 管理 IP 黑名单
- **端点**：`GET/POST/DELETE /api/admin/blacklist`
- **功能**：查询、添加或删除 IP 黑名单。

### 4.3 管理文件
- **端点**：`GET/DELETE /api/admin/files`
- **功能**：浏览或删除下载目录下的文件和文件夹。

### 4.4 文件下载
- **端点**：`GET /api/admin/files/download?path=...`
- **功能**：从管理后台直接下载服务器上的文件。

---

## 5. 下载验证接口

### 5.1 获取验证码配置
- **端点**：`GET /api/captcha/config`
- **功能**：获取极验验证码配置信息，用于前端初始化验证组件。
- **响应示例**：
  ```json
  {
    "enabled": true,
    "app_id": "9fab8370f958912499555f6ce0cd5c56"
  }
  ```
- **字段说明**：
  - `enabled`: 是否启用下载验证
  - `app_id`: 极验验证码 Captcha ID（前端初始化需要）

### 5.2 验证下载请求
- **端点**：`POST /api/download/verify`
- **功能**：验证用户完成的极验滑块验证，验证通过后返回临时下载令牌和下载链接。
- **请求体**：
  ```json
  {
    "lot_number": "e2f0a767a0f74926bbc8daeed22e6f27",
    "captcha_output": "...",
    "pass_token": "...",
    "gen_time": "1709551234",
    "file_path": "fcl/1.2.8.9/FCL-release-1.2.8.9-all.apk"
  }
  ```
- **响应示例** (验证成功)：
  ```json
  {
    "download_token": "abc123def456...",
    "download_url": "/download/abc123def456.../fcl/1.2.8.9/FCL-release-1.2.8.9-all.apk"
  }
  ```
- **响应示例** (验证失败)：
  ```json
  {
    "error": "verification_failed",
    "message": "pass_token expire"
  }
  ```
- **说明**：
  - `lot_number`, `captcha_output`, `pass_token`, `gen_time` 由极验前端 SDK `getValidate()` 方法返回
  - `download_token` 有效期 5 分钟，仅可使用一次
  - `download_url` 为完整的下载链接，可直接用于下载或复制给下载器使用
  - 下载链接格式：`/download/(token)/文件路径`

### 5.3 验证流程
1. 用户点击下载按钮
2. 前端调用 `GET /api/captcha/config` 检查是否启用验证
3. 若启用，加载极验 v4 SDK 并初始化验证
4. 用户完成滑块验证后，前端获取验证参数
5. 前端调用 `POST /api/download/verify` 提交验证
6. 后端验证通过后返回临时下载令牌和下载链接
7. 前端可选择直接下载或复制链接供下载器使用

---

## 6. 错误码说明

| 错误码 | 说明 |
|--------|------|
| `verification_required` | 需要完成验证码验证 |
| `invalid_token` | 下载令牌无效或已过期 |
| `token_mismatch` | 下载令牌与请求文件不匹配 |
| `verification_failed` | 验证码验证失败 |
| `file_not_found` | 请求的文件不存在 |
| `invalid_path` | 非法的文件路径 |
