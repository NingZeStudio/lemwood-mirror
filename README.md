# 柠枺镜像 (Lemwood Mirror)

本项目实现自动从 GitHub 获取指定启动器（fcl、zl、zl2）的最新 release，并将资产文件下载到本地存储结构，同时提供一个简单的黑白风格前端页面展示版本信息与下载链接，并具备基本文件浏览功能。

## 🌐 在线演示
- **官方镜像站**: [https://mirror.lemwood.icu/](https://mirror.lemwood.icu/)

## 功能概述
- 通过浏览器模拟（colly）获取启动器的 GitHub 仓库地址。
- 使用 GitHub API（go-github v50）获取最新 release（仅最新，不取历史）。
- 支持并发下载，可通过配置限制并发数（默认为 3）。
- 每 10 分钟自动检查更新（可通过配置调整）。
- 启动时执行异步初始扫描，不阻塞 Web 服务启动。
- 下载 release 资产到 `download/启动器名/版本号/`，并生成 `info.json`。
- 集成 SQLite 数据库，自动记录访问日志和下载统计。
- 提供详细的数据统计功能，包括访问量、下载排行、地域分布和每日趋势图表。
- 提供完善的 HTTP API 接口和后台管理功能（详见 [API 文档](API_DOCS.md)）。

## 目录结构
- `cmd/mirror`：主程序入口。
- `internal/...`：配置、浏览器模拟、GitHub 交互、下载、存储、HTTP 服务。
- `web/`：前端 Vue 项目源码。
- `web/dist`：构建后的前端静态资源（由后端托管）。
- `download`：下载文件根目录（默认）。
- `.github/workflows`：GitHub Actions 工作流，用于自动构建。

## 部署说明

### 1. 环境准备
- **Go**: 1.21 或更高版本（用于构建后端）。
- **Node.js & npm**: 最新稳定版（用于构建前端）。
- **操作系统**: Windows Server 2022 / Linux。  
如果你在GitHub Actions中下载了构建好的二进制文件，无需安装Go和Node.js等环境，直接解压运行即可。

### 2. 构建项目

#### 前端构建
进入 `web` 目录进行构建：
```bash
cd web
npm install
npm run build
```
构建产物将存放在 `web/dist` 目录中，后端会自动托管此目录。

#### 后端构建
在项目根目录执行：
```powershell
# Windows
go build -o mirror.exe ./cmd/mirror

# Linux
go build -o mirror ./cmd/mirror
```

### 3. 配置文件 (config.json)

在运行前，请根据实际情况修改项目根目录下的 `config.json`：

```json
{
  "server_address": "http://your-domain.com", // 服务器访问地址，用于生成 index.json 中的链接
  "server_port": 8080,                        // HTTP 服务监听端口
  "download_url_base": "https://mirror.lemwood.icu", // 外部下载链接的基准地址（如 CDN 或反代地址）
  "check_cron": "*/10 * * * *",               // 定时任务表达式，默认每 10 分钟扫描一次
  "storage_path": "download",                 // 下载文件和数据库的存储路径
  "github_token": "your_github_token",        // GitHub PAT 令牌，用于解除 API 请求频率限制
  "proxy_url": "",                            // 全局 HTTP 代理地址
  "asset_proxy_url": "",                      // GitHub Release 资产下载加速代理前缀
  "xget_domain": "https://xget.xi-xu.me",      // Xget 加速服务域名
  "xget_enabled": true,                       // 是否启用 Xget 加速
  "download_timeout_minutes": 40,             // 单个文件下载超时时间（分钟）
  "concurrent_downloads": 3,                  // 同时进行的下载任务数量
  "launchers": [                              // 需要镜像的启动器配置列表
    {
      "name": "fcl",                          // 启动器唯一标识名称
      "source_url": "https://github.com/FCL-Team/FoldCraftLauncher", // 官方页面或仓库 URL
      "repo_selector": ""                     // CSS 选择器或正则，用于从 source_url 提取仓库地址
    }
  ]
}
```
**关键配置项：**
- `github_token`: 建议配置以避免 GitHub API 频率限制。
- `download_url_base`: 外部访问的基准 URL，用于生成 `info.json` 中的下载链接。

### 4. 运行服务

#### 直接运行
```powershell
# Windows
./mirror.exe

# Linux
chmod +x mirror
./mirror
```

#### 使用环境变量 (可选)
可以通过环境变量覆盖配置：
```powershell
$env:GITHUB_TOKEN = "your_token"
./mirror.exe
```

### 5. 反向代理 (推荐)
建议使用 Nginx 进行反向代理，并开启 HTTPS：
```nginx
server {
    listen 443 ssl;
    server_name mirror.lemwood.icu;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 使用说明
- **前端首页**: 显示各启动器最新版本信息、下载量统计与下载链接。
- **手动刷新**: 点击“手动刷新”或访问 `POST /api/scan` 将立即触发一次版本检查。
- **文件浏览**: 访问 `/files` 可视化浏览存储目录结构。

## 数据统计
系统内置了基于 SQLite 的数据统计功能，自动记录用户的访问和下载行为。数据文件存储在 `storage_path` 下的 `stats.db` 中。
- **访问统计**: 记录 IP、User-Agent、地理位置等信息。
- **下载统计**: 记录具体下载的启动器、版本和文件名。
- **可视化面板**: 前端提供直观的每日趋势、下载分布图表。

## 📖 API 文档
详细的 API 接口说明请参阅 [API 文档](API_DOCS.md)。

---
*Powered by Lemwood Mirror Team*
