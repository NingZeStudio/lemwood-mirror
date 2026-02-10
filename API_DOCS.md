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
1. **常规登录**：客户端将用户名、密码及可选的 TOTP 验证码发送至 `POST /api/login`。
2. **两步验证 (2FA)**：如果系统启用了 2FA，登录时必须提供由验证器应用（如 Microsoft Authenticator）生成的 6 位动态验证码。

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
- **功能**：返回系统访问量、下载量等统计数据。

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
