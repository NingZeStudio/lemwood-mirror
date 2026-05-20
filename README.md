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

运行前需要编辑根目录的 `config.json`。
下面的示例保留了最常用字段，并使用当前代码中的实际配置结构。

```json
{
  "server_address": "",
  "server_port": 8080,
  "check_cron": "*/10 * * * *",
  "storage_path": "download",
  "github_token": "",
  "admin_user": "admin",
  "admin_password": "bcrypt-hash",
  "admin_enabled": true,
  "admin_max_retries": 10,
  "admin_lock_duration": 120,
  "proxy_url": "",
  "asset_proxy_url": "",
  "xget_domain": "https://xget.xi-xu.me",
  "xget_enabled": true,
  "download_timeout_minutes": 40,
  "concurrent_downloads": 3,
  "download_url_base": "https://mirror.example.com",
  "two_factor_enabled": false,
  "two_factor_secret": "",
  "captcha_enabled": true,
  "captcha_app_id": "your_captcha_id",
  "captcha_secret_key": "your_captcha_secret",
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

### 关键字段

- `download_url_base`：对外下载链接的基准地址，通常填你的反代域名或 CDN 域名。
- `github_token`：建议填写，用于降低 GitHub API 限流风险。
- `captcha_enabled`：开启后，浏览器下载会先走验证码验证流程。
- `traffic_limit_gb`：单 IP 每日下载流量上限，`0` 表示关闭该限制。
- `external_blacklist_url`：外部黑名单同步地址。
- `mysql_*`：填写后可切换到 MySQL；留空时默认使用 SQLite。

### 启动器字段

- `name`：启动器唯一标识，也会出现在 API 路径和文件目录中。
- `source_url`：GitHub 仓库地址，或可解析到仓库地址的来源页面。
- `repo_selector`：从 `source_url` 提取仓库地址时使用的选择器或规则。
- `include_prerelease`：是否把预发布版本也纳入扫描结果。
- `max_versions`：该启动器要拉取并保留的最近版本数量。

### `max_versions` 规则

- `max_versions > 0`：按配置值拉取并保留最近 N 个版本。
- `max_versions = 0`：使用默认值 `3`。

例如：

- `max_versions: 1` 表示只保留最近 1 个版本
- `max_versions: 2` 表示保留最近 2 个版本
- `max_versions: 0` 表示按默认值保留最近 3 个版本

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

- `download_token` 默认有效期为 5 分钟
- token 设计为短时一次性下载令牌
- `landing_url` 用于单独页面接力下载和回跳来源站点
- 非浏览器请求在验证码开启时访问 `/download/...`，会收到 JSON 错误而不是 HTML 页面

## 统计与风控

### 数据统计

- 访问记录：IP、路径、User-Agent、Referer、地区信息
- 下载记录：启动器、版本、文件名、来源 IP
- 聚合接口：总访问量、总下载量、最近 30 天数据、热门下载、地区分布、每日趋势

### 流量限制

- 支持单 IP 每日下载流量上限
- 超限后可自动封禁
- 可同步外部黑名单
- 封禁记录可写入公开文件，例如 `banned_ips.txt`

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
