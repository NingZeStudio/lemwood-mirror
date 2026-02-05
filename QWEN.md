# 柠枺镜像 (Lemwood Mirror) 项目指南

## 项目概述

柠枺镜像 (Lemwood Mirror) 是一个自动化的 GitHub 发布版本镜像工具，专门用于获取 Minecraft 启动器（如 FCL、Zalith 等）的最新发布版本，并将资产文件下载到本地存储，同时提供一个前端界面展示版本信息和下载链接。

### 主要特性

- **自动同步**: 通过浏览器模拟（colly）获取启动器的 GitHub 仓库地址
- **GitHub API 集成**: 使用 GitHub API（go-github v50）获取最新 release（仅最新，不取历史）
- **并发下载**: 支持并发下载，可通过配置限制并发数（默认为 3）
- **定时检查**: 每 10 分钟自动检查更新（可通过配置调整）
- **异步扫描**: 启动时执行异步初始扫描，不阻塞 Web 服务启动
- **文件组织**: 下载 release 资产到 `download/启动器名/版本号/`，并生成 `info.json`
- **数据统计**: 集成 SQLite 数据库，自动记录访问日志和下载统计
- **HTTP 服务**: 提供多种 API 端点和前端页面

### 技术栈

- **后端**: Go (1.24.0+)
- **前端**: Vue 3 + Vite + Tailwind CSS + ECharts
- **数据库**: SQLite (通过 modernc.org/sqlite)
- **依赖管理**: Go modules
- **包管理**: npm

## 项目架构

```
.
├── cmd/
│   └── mirror/           # 主程序入口
├── internal/
│   ├── browser/          # 浏览器模拟（colly）相关功能
│   ├── config/           # 配置文件加载和解析
│   ├── db/               # 数据库初始化和操作
│   ├── downloader/       # 文件下载逻辑
│   ├── github/           # GitHub API 交互
│   ├── server/           # HTTP 服务和路由
│   ├── stats/            # 统计数据收集和分析
│   └── storage/          # 存储相关功能
├── web/                  # 前端 Vue 项目
│   ├── public/           # 静态资源
│   ├── src/              # 源代码
│   ├── dist/             # 构建后的前端静态资源（由后端托管）
│   └── package.json      # 前端依赖
├── config.json           # 项目配置文件
├── go.mod, go.sum        # Go 依赖管理
└── README.md             # 项目说明文档
```

## 构建和运行

### 环境要求

- **Go**: 1.24.0 或更高版本（用于构建后端）
- **Node.js & npm**: 最新稳定版（用于构建前端）

### 构建步骤

#### 1. 构建前端

```bash
cd web
npm install
npm run build
```

构建产物将存放在 `web/dist` 目录中，后端会自动托管此目录。

#### 2. 构建后端

在项目根目录执行：

```bash
# Linux/macOS
go build -o mirror ./cmd/mirror

# Windows
go build -o mirror.exe ./cmd/mirror
```

### 配置

修改项目根目录下的 `config.json` 文件：

```json
{
  "server_address": "http://your-domain.com",
  "server_port": 8080,
  "download_url_base": "https://mirror.lemwood.icu",
  "check_cron": "*/10 * * * *",
  "storage_path": "download",
  "github_token": "your_github_token",
  "proxy_url": "",
  "asset_proxy_url": "",
  "xget_domain": "https://xget.xi-xu.me",
  "xget_enabled": true,
  "download_timeout_minutes": 40,
  "concurrent_downloads": 3,
  "launchers": [
    {
      "name": "fcl",
      "source_url": "https://github.com/FCL-Team/FoldCraftLauncher",
      "repo_selector": ""
    }
  ]
}
```

### 运行服务

```bash
# Linux/macOS
./mirror

# Windows
./mirror.exe
```

## API 接口

- `GET /` - 前端页面
- `GET /api/status` - 返回各启动器版本信息
- `GET /api/latest` - 返回所有启动器的最新稳定版本信息
- `GET /api/latest/{launcher_id}` - 返回指定启动器的最新稳定版本信息
- `POST /api/scan` - 触发一次手动扫描
- `GET /api/files?path=...` - 列出存储目录树
- `GET /download/...` - 提供下载静态文件

## 开发约定

### Go 代码规范

- 使用标准 Go 代码格式 (`gofmt`)
- 遵循 Go 语言最佳实践
- 使用 Go modules 管理依赖

### 前端开发规范

- 使用 Vue 3 Composition API
- 使用 Tailwind CSS 进行样式设计
- 使用 ECharts 进行数据可视化

### 错误处理

- 在 Go 代码中正确处理错误并记录日志
- 对外部 API 调用实现适当的重试机制
- 实现安全的文件路径处理以防止路径遍历攻击

## 数据库结构

项目使用 SQLite 数据库存储访问和下载统计信息：

- `visits` 表：记录访问信息（IP、路径、UA、地理位置等）
- `downloads` 表：记录下载信息（文件名、启动器、版本、IP、国家等）

## 安全措施

- 实现路径遍历防护（使用 `containsDotDot` 函数）
- 对用户输入进行验证和清理
- 使用安全的文件访问模式
- 实现适当的速率限制和错误处理

## 部署建议

- 使用反向代理（如 Nginx）进行 HTTPS 终止
- 配置 GitHub Token 以避免 API 速率限制
- 定期备份数据库文件
- 监控应用日志和性能指标