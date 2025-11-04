# 启动器自动镜像程序

本项目实现自动从 GitHub 获取指定启动器（fcl、zl、zl2）的最新 release，并将资产文件下载到本地存储结构，同时提供一个简单的黑白风格前端页面展示版本信息与下载链接，并具备基本文件浏览功能。

## 功能概述
- 通过浏览器模拟（colly）获取启动器的 GitHub 仓库地址。
- 使用 GitHub API（go-github v50）获取最新 release（仅最新，不取历史）。
- 支持并发下载，可通过配置限制并发数（默认为 3）。
- 每 10 分钟自动检查更新（可通过配置调整）。
- 启动时执行异步初始扫描，不阻塞 Web 服务启动。
- 下载 release 资产到 `download/启动器名/版本号/`，并生成 `info.json`。
- 提供 HTTP 服务：
  - `GET /` 前端页面。
  - `GET /api/status` 返回各启动器版本信息。
  - `POST /api/scan` 触发一次手动扫描。
  - `GET /api/files?path=...` 列出存储目录树。
  - `GET /download/...` 提供下载静态文件。

## 目录结构
- `cmd/mirror`：主程序入口。
- `internal/...`：配置、浏览器模拟、GitHub 交互、下载、存储、HTTP 服务。
- `web/static`：前端 HTML/CSS/JS。
- `download`：下载文件根目录（默认）。
- `.github/workflows`：GitHub Actions 工作流，用于自动构建。

## 配置

通过修改 `config.json` 文件来自定义程序的行为。

- `github_token`: 你的 GitHub Personal Access Token，用于提高 API 请求速率限制。
- `storage_path`: 下载文件的存储目录，默认为 `download`。
- `server_address`: 用于生成 `index.json` 中资源下载链接的服务器地址（IP 或域名），不应包含端口号，例如 `http://127.0.0.1`。如果留空，程序将自动获取并使用服务器的公共 IP 地址。
- `server_port`: HTTP 服务的监听端口，默认为 8080。此端口也会用于生成 `index.json` 中的下载链接。
- `check_cron`: 自动检查更新的 cron 表达式，默认为每 10 分钟检查一次 (`*/10 * * * *`)。
- `proxy_url`: 用于网络请求的 HTTP/HTTPS 代理地址，例如 `http://127.0.0.1:7890`。
- `asset_proxy_url`: 用于加速 GitHub Release 资源下载的代理地址，会作为前缀拼接到下载链接前。
- `xget_domain`: Xget 服务域名，用于加速 GitHub 仓库的访问和下载。
- `xget_enabled`: 是否启用 Xget 加速，`true` 或 `false`。
- `download_timeout_minutes`: 下载单个文件的超时时间（分钟），默认为 40。
- `concurrent_downloads`: 并发下载数，默认为 3。
- `launchers`: 要镜像的启动器列表。
  - `name`: 启动器名称。
  - `source_url`: 包含 GitHub 仓库链接的官方页面地址。
  - `repo_selector`: 用于从页面中提取 GitHub 仓库链接的 CSS 选择器。

## 构建与运行

### 手动构建

确保你已安装 Go (>=1.21)。

```powershell
# 在项目根目录执行
go build -o .\mirror.exe .\cmd\mirror
```

### 自动构建

每次向 `main` 分支推送代码时，GitHub Actions 会自动构建 Windows 和 Linux 的二进制文件、配置文件和前端资源，并打包为 `mirror-windows.zip` 和 `mirror-linux.tar.gz`。你可以在仓库的 Actions 页面下载这些构建产物。

### 运行

```powershell
# 可选：设置 GitHub Token
$env:GITHUB_TOKEN = "<your token>"
# 启动服务
./mirror.exe
# 访问 http://localhost:8080
```

## 使用说明
- 前端首页显示各启动器最新版本信息、文件路径提示与下载链接。
- 点击“手动刷新”将触发一次扫描更新。
- 文件浏览可输入相对路径（例如 `.`、`fcl/`、`fcl/v1.2.3/`）查看结构。

## API 集成

其他网站或服务可以通过访问 `/api/status` 端点来获取镜像的最新版本信息。该端点返回一个 JSON 对象，其中包含每个启动器的详细信息。

### 请求

```http
GET /api/status
```

#### 获取特定启动器信息

你也可以通过在路径中指定启动器 ID 来获取特定启动器的数据。

```http
GET /api/status/{launcher_id}
```

### 响应示例

```json
{
  "fcl": {
    "version": "1.2.6.3",
    "download_path": "download/fcl/1.2.6.3",
    "assets": [
      {
        "name": "FCL-release-1.2.6.3-all.apk",
        "size": 123456,
        "download_url": "http://your-server-address:8080/download/fcl/1.2.6.3/FCL-release-1.2.6.3-all.apk"
      }
    ]
  },
  "zl": {
    "version": "141000",
    "download_path": "download/zl/141000",
    "assets": [
      {
        "name": "ZalithLauncher-1.4.1.0.apk",
        "size": 234567,
        "download_url": "http://your-server-address:8080/download/zl/141000/ZalithLauncher-1.4.1.0.apk"
      }
    ]
  }
}
```

- **version**: 启动器的最新版本号。
- **download_path**: 存储该版本文件的相对路径。
- **assets**: 一个包含所有已下载资产文件的数组。
  - **name**: 资产文件名。
  - **size**: 文件大小（字节）。
  - **download_url**: 文件的完整下载链接。

## 认证与限流
- 建议在配置或环境变量中提供 `GITHUB_TOKEN`，提升 API 配额。
- 代码在遇到 403/配额耗尽时会按照响应的重置时间进行退避等待（有限）。

## 并发安全与资源清理
- 下载采用原子写入（.partial -> rename）。
- 使用上下文超时控制网络请求。
- 在内存状态更新和索引维护处使用锁保证并发安全。
