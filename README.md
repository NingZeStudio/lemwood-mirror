# 柠枺镜像

柠枺镜像是一个面向启动器分发场景的 GitHub Release 镜像服务。
它会定时扫描上游仓库，下载并保留最近几个版本的资产文件，对外提供站点页面、下载链接、统计接口和下载验证流程。

## 项目能做什么

- 定时扫描 GitHub 仓库并同步 release 资产
- 按启动器保留最近几个版本，避免本地存储无限增长
- 生成结构化版本索引，供前端页面和 API 查询
- 提供公共查询接口、下载准备接口和验证码验证接口
- 记录访问量、下载量、热门资源、地区分布和每日趋势
- 支持单 IP 流量限制、黑名单和公开封禁记录
- 提供后台管理能力，用于配置、黑名单和文件管理

## 目录概览

- `cmd/mirror`：后端程序入口
- `internal/config`：配置加载与保存
- `internal/github`：GitHub API 封装
- `internal/downloader`：版本索引生成与资产下载
- `internal/server`：HTTP 路由、下载验证和静态站点托管
- `internal/stats`：访问与下载统计
- `internal/traffic`：流量限制与封禁逻辑
- `web`：用户站点前端源码
- `admin-app`：后台管理前端源码
- `download`：镜像文件、数据库和封禁记录的默认存储目录

## 快速开始

### 环境要求

- Go 1.21 或更高版本
- Node.js 18 或更高版本
- npm
- Windows 或 Linux

### 构建前端

用户站点和后台都需要先构建静态资源，后端启动后会直接托管它们。

```bash
cd web
npm install
npm run build
```

```bash
cd ../admin-app
npm install
npm run build
```

### 构建后端

在仓库根目录执行：

```powershell
# Windows
go build -o mirror.exe ./cmd/mirror

# Linux
go build -o mirror ./cmd/mirror
```

### 运行服务

```powershell
# Windows
.\mirror.exe

# Linux
chmod +x mirror
./mirror
```

也可以直接开发运行：

```powershell
go run ./cmd/mirror
```

## 配置说明

运行前需要编辑根目录的 `config.json`。以下为完整配置示例（敏感值已用占位符替换）：

```json
{
  "server_address": "",
  "server_port": 8080,
  "check_cron": "*/10 * * * *",
  "storage_path": "download",
  "github_token": "",
  "download_url_base": "https://mirror.example.com",
  "download_timeout_minutes": 40,
  "concurrent_downloads": 3,
  "proxy_url": "",
  "asset_proxy_url": "",
  "xget_domain": "https://xget.xi-xu.me",
  "xget_enabled": true,
  "admin_enabled": true,
  "admin_user": "admin",
  "admin_password": "<bcrypt-hash>",
  "admin_max_retries": 10,
  "admin_lock_duration": 120,
  "two_factor_enabled": false,
  "two_factor_secret": "",
  "captcha_enabled": false,
  "captcha_app_id": "",
  "captcha_secret_key": "",
  "traffic_limit_gb": 0,
  "ban_record_file": "banned_ips.txt",
  "external_blacklist_url": "",
  "appeal_contact": "QQ群 https://qm.qq.com/q/FOGt99aayY",
  "mysql_host": "",
  "mysql_port": 3306,
  "mysql_user": "",
  "mysql_password": "",
  "mysql_database": "",
  "mysql_migration": false,
  "launchers": [
    {
      "name": "fcl",
      "source_url": "https://github.com/FCL-Team/FoldCraftLauncher",
      "repo_selector": "",
      "include_prerelease": false,
      "max_versions": 2
    }
  ]
}
```

### 服务与网络

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `server_address` | string | `""` | 服务绑定地址，留空表示监听所有网卡（`0.0.0.0`）。**同时用于构造对外下载链接**（当 `download_url_base` 为空时作为 fallback） |
| `server_port` | int | — | 服务端口 |
| `download_url_base` | string | `""` | **对外下载链接的基准地址**。填入你的反代域名或 CDN 域名（含协议头），如 `"https://dl.mysite.com"`。为空时自动回退使用 `server_address`，再为空则通过 `ifconfig.me/ip` 获取公网 IP |
| `proxy_url` | string | `""` | HTTP 代理地址，用于扫描阶段下载 GitHub Release 资源 |
| `asset_proxy_url` | string | `""` | 资源下载地址前缀代理，会拼接在 GitHub 原始 URL 之前 |
| `xget_enabled` | bool | — | 是否启用 xget 代理加速下载 GitHub 资源 |
| `xget_domain` | string | — | xget 服务域名，替换 GitHub 下载 URL 中的 `github.com` 部分 |

### GitHub 与扫描

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `github_token` | string | `""` | GitHub Personal Access Token，**强烈建议填写**以降低 API 限流风险（未认证每小时仅 60 次）。也支持通过环境变量 `GITHUB_TOKEN` 覆盖 |
| `check_cron` | string | `"*/10 * * * *"` | 定时扫描 Cron 表达式（每分钟粒度），为空时自动使用默认值 |
| `download_timeout_minutes` | int | — | 单个资源文件下载超时（分钟） |
| `concurrent_downloads` | int | `3` | 并发下载数，≤0 时自动修正为 3 |

### 管理员

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `admin_enabled` | bool | — | 是否启用后台管理 |
| `admin_user` | string | — | 管理员用户名。若 `admin_enabled` 为 `true` 但此项为空，管理后台会被**自动禁用** |
| `admin_password` | string | — | bcrypt 哈希后的管理员密码。使用 `htpasswd -bnBC 14 "" <password> | tr -d ':\n'` 生成。若为空则自动禁用管理后台 |
| `admin_max_retries` | int | `10` | 登录失败次数上限，超出后 IP 被临时锁定。≤0 时自动修正为 10 |
| `admin_lock_duration` | int | `120` | IP 锁定持续时间（**分钟**）。≤0 时自动修正为 120（2 小时） |
| `two_factor_enabled` | bool | — | 是否为管理后台启用 TOTP 两步验证 |
| `two_factor_secret` | string | — | TOTP 共享密钥 |

### 验证码

| 字段 | 类型 | 说明 |
|------|------|------|
| `captcha_enabled` | bool | 是否启用下载验证码（极验 GeeTest）。开启后浏览器下载必须先完成人机验证 |
| `captcha_app_id` | string | 极验 App ID |
| `captcha_secret_key` | string | 极验 Secret Key（**敏感信息，请勿泄露**） |

### 流量控制与封禁

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `traffic_limit_gb` | int | — | 单 IP 每日下载流量上限（GB）。`0` 表示**完全禁用**流量限制。**负值自动修正为 `5`** |
| `ban_record_file` | string | `"banned_ips.txt"` | 封禁记录输出文件，存储在 `storage_path` 下 |
| `external_blacklist_url` | string | `""` | 外部 IP 黑名单同步地址（按行解析，跳注释行），定时同步 |
| `appeal_contact` | string | `"QQ群 https://qm.qq.com/q/FOGt99aayY"` | 被封禁用户看到的申诉联系方式 |

### 数据库（可选 MySQL）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `mysql_host` | string | `""` | MySQL 主机地址。**留空则使用 SQLite** |
| `mysql_port` | int | `3306` | MySQL 端口 |
| `mysql_user` | string | `""` | MySQL 用户名 |
| `mysql_password` | string | `""` | MySQL 密码 |
| `mysql_database` | string | `""` | MySQL 数据库名 |
| `mysql_migration` | bool | `false` | 是否启用 MySQL 迁移模式 |

### 启动器字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `name` | string | — | 启动器唯一标识，同时用于 API 路径（`/api/status/{name}`）和文件目录名 |
| `source_url` | string | — | GitHub 仓库地址（如 `https://github.com/owner/repo`）或可从中提取仓库地址的任意网页 URL |
| `repo_selector` | string | `""` | 从 `source_url` 页面提取 GitHub 仓库地址的规则：<br>• 留空：匹配页面中第一个 `github.com` 链接<br>• 以 `"regex:"` 开头：用后续正则表达式匹配 `<a href>`<br>• 其他：视为 CSS 选择器匹配元素<br>若 `source_url` 本身已是 GitHub 仓库地址，则忽略此字段 |
| `include_prerelease` | bool | `false` | 是否将预发布版本（Pre-release）纳入扫描和保留范围 |
| `max_versions` | int | `0` (=`3`) | 保留的最大版本数。`> 0` 时按配置值保留；`≤ 0` 时自动修正为 **3** |

### `max_versions` 示例

| 配置值 | 实际保留版本数 |
|--------|---------------|
| `1` | 只保留最近 1 个版本 |
| `2` | 保留最近 2 个版本 |
| `0` | 使用默认值，保留最近 3 个版本 |
| `-1` | 同上，被 `NormalizeMaxVersions` 修正为 3 |

## 下载流程

项目当前的浏览器下载链路分为两种情况。

### 验证码关闭

1. 前端调用 `POST /api/download/prepare`
2. 服务端返回 `download_token`、`download_url`、`landing_url`
3. 前端进入下载引导页，调用 `GET /api/download/landing?token=...`
4. 引导页触发真实下载 `/download/...`

### 验证码开启

1. 前端调用 `GET /api/captcha/config` 获取验证码配置
2. 用户完成验证后，前端调用 `POST /api/download/verify`
3. 服务端返回 `download_token`、`download_url`、`landing_url`
4. 前端进入下载引导页，调用 `GET /api/download/landing?token=...`
5. 引导页触发真实下载 `/download/...`

### 额外说明

- `download_token` 默认为 64 字符十六进制随机串，有效期 5 分钟。
- `landing` 接口通过 `Peek` 模式**只读取不消费**令牌，可多次调用；实际下载接口通过 `Validate` 模式**一次性消费**令牌。
- `landing_url` 用于下载引导页接力，支持读取 `return_url` 以实现下载后回跳来源站点。
- 非浏览器请求在验证码开启时访问 `/download/...`，会收到 JSON 错误而不是 HTML 验证页面。

## 统计与风控

### 数据统计

- **访问记录**：IP、路径、User-Agent、Referer、地区（通过 IP 地理位置库解析）
- **下载记录**：启动器、版本、文件名、来源 IP（仅 `200`/`206` 响应计入）
- **聚合接口**：总访问量、总下载量、最近 30 天数据、Top 10 热门下载、Top 50 地区分布、最近 30 天每日趋势
- **缓存策略**：统计接口 `Cache-Control: public, max-age=300`（5 分钟）
- 访问和下载记录通过异步 worker 池写入（4 worker + 1000 缓冲队列），不阻塞请求

### 流量限制

- 支持单 IP 每日下载流量上限（GB 级粒度）
- **预估机制**：下载前解析 `Range` 头预估传输量做预检，超限直接拒绝
- **精确记录**：下载完成后按实际传输字节数写入数据库
- 超限后**自动封禁** IP，写入本地黑名单和封禁记录文件
- 封禁记录格式：`IP | 封禁时间 | 封禁理由 | 当日流量(GB)`
- 可同步**外部黑名单**（按行解析，跳过 `#` 注释行），定时更新

## 部署建议

生产环境建议使用 Nginx 或其他反向代理，并开启 HTTPS。

```nginx
server {
    listen 443 ssl;
    server_name mirror.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 验证是否工作正常

- 打开首页，确认用户站点能够展示启动器版本列表
- 访问 `/api/status`，确认能返回版本索引
- 访问 `/api/latest`，确认能返回每个启动器的最新版本号
- 访问 `/api/stats`，确认统计接口正常
- 执行一次实际下载，确认下载引导页和真实下载链路可用

## 文档入口

- 公共 API 文档：`API_DOCS.md`
- 站内 API 速查页：用户站点中的 `/api`

## 许可

本项目使用仓库中的 `LICENSE`。
