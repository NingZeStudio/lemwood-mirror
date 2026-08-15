# lemwood-mirror 项目记忆

面向 Minecraft 启动器分发场景的 GitHub Release 镜像服务（Go 单体 + 内嵌前端）。入口 `cmd/mirror/main.go`，后端全部在 `internal/`。

## 构建与测试

- Go 1.24。测试/构建：`go test -count=1 ./...`、`go vet ./...`。CI 用 `go test ./... -count=1 -timeout 120s`。
- Termux 环境 go 不在默认 PATH，位于 `/data/data/com.termux/files/usr/lib/go/bin`（apt 的 golang 包），用前先 `export PATH=$PREFIX/lib/go/bin:$PATH`。
- 构建：`go build -o mirror ./cmd/mirror`；开发运行：`go run ./cmd/mirror`。
- 纯 Go 构建，`CGO_ENABLED=0`：SQLite 驱动是 `modernc.org/sqlite`（pure-Go），**不需要 cgo**，不要为 SQLite 切换 `mattn/go-sqlite3`。
- 版本号靠 `-ldflags "-s -w -X main.Version=..."` 注入：CI tag 构建注入 `github.ref_name`；Docker 需 `--build-arg VERSION=x.y.z`（Dockerfile ARG VERSION）；dev 构建为 `"dev"`，自更新检查时视为"有更新"。
- **构建产物不入库（2026-08-15 起）**：`mirror-linux-amd64` 曾误入库（66MB）已 `git rm --cached` 移除，.gitignore 含 `mirror-linux-amd64`。本地 `go build` 的二进制只用于本地验证/部署，**不要 `git add` 任何二进制**；正式产物由 CI 按 tag 构建发布。
- 前端构建：`cd frontend && pnpm build`，产物输出到 `web/default/`（git 跟踪的内嵌资源，改完 frontend/src 必须重新构建否则线上不生效）。
- 管理端构建：`cd admin-app && pnpm build`，产物输出到 `web/admin/`（同样被 git 跟踪，会被构建重写 hash 文件名）。
- CI（`.github/workflows/build.yml`）矩阵构建 windows/linux × amd64/arm64/x86；**仅 tag 推送时发 Release**（softprops/action-gh-release）。pnpm 版本固定 10.12.4。

## 后端架构（internal/ 包职责）

- `cmd/mirror/main.go` — 唯一入口。启动顺序：释放内嵌前端 → 加载配置 → InitDB → 流量 tracker → stats 写池（4 worker + 1000 缓冲）→ server.State → scanner（cron）→ selfupdate manager → cron 调度。`Scanner.scanLauncher` 是同步主循环，`ScanAll` 用 `scanMu.TryLock` 防重入。
- `internal/server/` — HTTP 路由 + SPA 托管 + 下载处理器。`server.go`（41KB，单体）是改动热点；`v2.go` 是公开 API v2 handler；`spafallback.go` SPA 回退；`http.go`/`utils.go` 小工具。**下载处理器是流量计数的唯一计数点**（见下"流量统计双口径"）。
- `internal/db/` — 数据库抽象（SQLite/MySQL），见下"数据库"。
- `internal/config/` — 配置加载/保存/迁移，见下"配置"。
- `internal/stats/` — 访问/下载统计，异步写池 + 快照预热。
- `internal/traffic/` — 单 IP 每日流量上限（防刷墙），预检 + 实际传输记账。
- `internal/github/` — go-github v50 封装，见下"GitHub 客户端"。
- `internal/selfupdate/` — GitHub Release 自更新，见下"自更新"。
- `internal/geoip/` — ip2region v4+v6 xdb 内嵌，本地查属地。
- `internal/version/` — 自研 SemVer-like 比较，被 launcher 索引和 selfupdate 共用。
- `internal/downloader/` — 版本索引生成与资产下载。
- `internal/download_authz/` — DB 授权状态表（`download_authorizations`）：43 字符 base64url token，DB 存 sha256 hash；Issue/Peek/Consume（atomic 单次消费）。见下"PoW 下载验证"。
- `internal/auth/` — 管理员认证 + TOTP，`CleanupTokens()` 后台清理。
- `internal/assets/` — 启动时把内嵌前端释放到项目根目录。
- `internal/blacklist/` / `internal/netutil/`（客户端 IP 解析）/ `internal/storage/` — 各自独立小包。

## 数据库

- 全局 `db.DB *sql.DB` + `db.isMySQL` 标志。**SQLite 默认**（`<storage>/stats.db`），MySQL 可选（`mysql_host` 非空即启用）。
- SQLite：`SetMaxOpenConns(1)`（单写连接，避免 SQLite 锁冲突），开 WAL + `busy_timeout=10000`。MySQL：连接池 25/10，`ConnMaxLifetime=1h`。
- **迁移系统**在 `internal/db/migrations.go`：`CurrentSchemaVersion` 常量（当前 = 4）。新增迁移 = 往 `migrations` 切片末尾追加一项 + 递增该常量。每个 `Up` **必须幂等**（用 `CREATE TABLE IF NOT EXISTS` / `INSERT IGNORE`（MySQL）或 `INSERT OR IGNORE`（SQLite））。`system_info` 表的 `schema_version` 是提交点，每个迁移成功后立即写入。MySQL 用 `ON DUPLICATE KEY UPDATE`，SQLite 用 `INSERT OR REPLACE` 写 schema_version。
- `mysql_migration: true` 且存在 `stats.db` → 一次性 SQLite→MySQL 迁移，旧库改名为 `.bak`。
- 表清单：`visits, downloads, ip_blacklist, ip_daily_traffic, daily_traffic, daily_completed_traffic, download_authorizations, download_events, system_info`。建表在 `createTables()`，按 `isMySQL` 分支用各自方言。
- 注：repo 镜像功能已移除，`repo_*` 表不再迁移；`launcher.mode` 的 `clone`/`all` 已废弃（Git 镜像功能已移除），仅为兼容旧配置保留，`ShouldSyncRelease` 对 `release`/`all` 返回 true、对 `clone` 返回 false。

## 配置

- `config.yaml` 和旧 `config.json` **都在 .gitignore（含 secrets，绝不提交）**。仓库里 `internal/config/default.yaml` 是提交的嵌入式默认模板（经 `embedded.go` 内嵌）。
- `LoadConfig` 行为：无 yaml 但存在旧 `config.json` → 自动迁移到 yaml 并删除旧 json；两者都没有 → 释放内嵌 `default.yaml` 写盘。
- **后台保存会从 `defaultConfigTemplate`（text/template）整体重写 config.yaml**，不要指望手写在 yaml 里加的自定义注释能在后台保存后保留。
- `GITHUB_TOKEN` 环境变量覆盖 yaml 里的 `github_token`。
- `NormalizeConfig` 不变量：`traffic_limit_gb < 0` → 5；`max_versions ≤ 0` → 3；`admin_enabled` 但 user/password 空 → 自动禁用后台；`check_cron` 空 → `*/10 * * * *`。
- 密码用 bcrypt 哈希：`htpasswd -bnBC 14 "" <password> | tr -d ':\n'`。

## 流量统计双口径（2026-08 起）

- `daily_traffic` = served（服务器写出字节，含中止传输），防刷墙专用。
- `daily_completed_traffic` = 完整传输字节（`counter.Total >= EstimateTransferBytes(...)` 且状态 200/206），stats 展示口径。
- 唯一计数点：`internal/server/server.go` 下载处理器（`responseWriterCounter` 包装 ResponseWriter，~line 528-556）；DB 读写：`internal/db/db.go`（`RecordTraffic` served / `RecordCompletedTraffic` completed）；schema v3 迁移负责建表回填。
- stats 展示口径取 `daily_completed_traffic`（无 IP 维度聚合）；防刷墙取 `ip_daily_traffic`（IP 维度，仅留当日，每小时清理 24h 外记录）。
- `stats.InitWritePool(4, 1000)`：访问/下载记录异步落库，**不要在请求路径里同步写 DB 统计**。`/api/v2/stats` 走 `RefreshSnapshot` 预热（启动 + 每 10m 刷新）的快照，避免每次跑聚合查询。

## PoW 下载验证 + 状态表授权/流量（2026-08-12 起，替代极验）

- **背景**：移除极验（`internal/captcha` 已删）与内存 `download_token`（`internal/download_token` 已删）。改为 PoW 自动验证客户端正规性 + DB 状态表承载授权与流量。参考 PoW实现.md 与 MapleMirror（AGPL，仅参考数据模型形状/流程，**从零实现不复制源码**）。单节点、改 v2（不新建 v3）。前端已迁移（2026-08-12）。
- **PoW（`internal/pow`）**：ALTCHA 风格 PBKDF2-SHA256（自实现，无 altcha 依赖——因 altcha-lib-go/v2 传递性要求 go 1.25，本仓 CI 为 go 1.24）。挑战**内存态**（`reserved→open→issuing→consumed`，不落库，对齐 PoW实现.md §1.10/MapleMirror §5.1），TTL 默认 2m。`pow.Solve(p, max)` 供测试/CLI。`servePowPage`（server.go）是浏览器直连验证页：Web Crypto PBKDF2 求解 → POST authorize → 跳转，无 CDN。
- **授权（`internal/download_authz` + `download_authorizations` 表）**：token = 32 字节无填充 base64url（43 字符），DB 只存 `token_hash=sha256(token)` hex。`Issue/Peek/Consume`（`ConsumeDownloadAuthorization` atomic：`issued 且未过期 → consumed`，单次防重放）。
- **事件/流量（`download_events` 表）**：每次下载一行（`authorization_id/file/launcher/version/client_ip/country/bytes_served/completed/status_code/date`）。**全切口径**：served=bytes_served（含中止），completed=bytes_served WHERE completed=1。防刷墙按 IP 当日 served 读 `download_events`（`GetDailyServedByIPFromEventsToday`，注入 `traffic.InitTracker`）；`daily_traffic`/`daily_completed_traffic` **冻结为只读历史基线**（不再写入，schema v4 迁移从 `downloads` 回填事件行 bytes=0）。
- **v2 端点**：`POST /api/v2/downloads/prepare`（CLI/API 直发授权，无 PoW，保留）、`GET /landing`（Peek）、`GET /downloads/challenge` + `POST /downloads/authorize`（浏览器 PoW，替代极验 verify）、`GET /api/v2/pow/config`。`/download/{path}`：token 取 `?token=` 或 `Authorization: Bearer`；无 token+浏览器→`servePowPage`，无 token+非浏览器→403 `verification_required`，有 token→`authz.Consume`+文件绑定校验+`RecordDownloadEvent`+`FinalizeTraffic`（顺序：先 event 后 finalize，防刷墙才读得到本次字节）。
- **stats**：`applyDownloadAndTrafficStats`（stats.go）合并：下载次数/top 从 `download_events`（含回填）；流量字节 = 冻结基线 SUM + 事件 SUM（按日 union）；visits/geo 不变（仍读 `visits`）。**注意（2026-08-14 修复）**：每日 `DailyStats[].DownloadCount` 与 `TrafficBytes` 一并合并事件口径，`queryDailyStats` 里的 `downloads` 表查询只作兜底，否则迁移后当日下载计数恒为 0（`downloads` 不再写入）。
- **config**：删 `captcha_*`；加 `pow_enabled`(默认 true)/`pow_algorithm`("PBKDF2-SHA256")/`pow_cost`(5000)/`pow_key_length`(32)/`pow_difficulty`(14)/`pow_challenge_ttl`("2m")/`pow_hmac_secret`(空→启动随机生成，挑战内存态无需持久)/`download_token_ttl`("5m")。
- **前端**（`frontend/src`）：`globalConfig.api.endpoints` 用 `powConfig:'/pow/config'` + `downloadChallenge:'/downloads/challenge'` + `downloadAuthorize:'/downloads/authorize'`；`api.js` 移除 `getCaptchaConfig`/`verifyDownload`，新增 `getPowConfig`/`createDownloadChallenge`/`authorizeDownload`。`VersionList.vue`/`FilesView.vue` 的 `handleDownload`：`powConfig.enabled` 时路由 `/verify?file=...`（VerifyView 现在是 PoW 求解页：Web Crypto PBKDF2 → challenge → solve → authorize → `/download-started?token=`），否则 `prepareDownload`（CLI/API 路径无 PoW）。`DownloadStartedView` 不变（landing 契约未变）。构建：`cd frontend && pnpm build` → `web/default/`（git 跟踪，改完必须重新构建）。
- **PoW derivedKey 编码坑**：客户端（浏览器/CLI）`derivedKey` 用**无填充** base64url，服务端 `verifySolution` 须用 `base64.RawURLEncoding.DecodeString(strings.TrimRight(derivedKey,"="))` 解码后与重算字节做 `ConstantTimeCompare`，不要用带填充的 `URLEncoding.EncodeToString` 直接比字符串。
- **DATETIME 读取坑**：modernc.org/sqlite 把 DATETIME 列读回时解析为 time.Time 再以 RFC3339 回传给 `Scan(&string)`。`download_authz.isExpired` 用 `parseAuthzTime` 多布局兼容（AuthzTimeFormat + RFC3339*）。DB 内 SQL `expires_at > ?` 字符串比较仍按 "2006-01-02 15:04:05" 文本字典序工作。

## GitHub 客户端

- `internal/github/client.go`，go-github v50。`BackoffIfRateLimited`：当 `resp.Rate.Remaining == 0` 时 sleep 到 reset+2s（上限 15 分钟）。
- Token 可选：无 token 60 req/h，有 token 5000/h。`GITHUB_TOKEN` env 优先于配置。
- 代理：`proxy_url` 非空 → 显式 `http.ProxyURL`；为空 → 回退 `http.DefaultTransport`（即尊重 `HTTP_PROXY`/`HTTPS_PROXY` 环境变量）。运行时切代理用 `SetProxy`（atomic.Pointer 安全并发）。
- `ListReleasesByPolicy` 分页拉取并按 `include_prerelease` 过滤；`GetLatestRelease` API 不返回 pre-release，含 pre-release 时走 `ListReleases` 取首个。

## GeoIP

- `internal/geoip/`：ip2region v4 + v6 xdb 通过 `//go:embed` 内嵌（合计约 48MB，是二进制体积主因），本地查属地**不联网**。`sync.Once` 懒初始化。

## 版本比较

- `internal/version/version.go` 自研 SemVer-like `Compare`，被 launcher 索引排序和 selfupdate 更新检查**共用**。pre-release 后缀（`-` 后部分）排名低于同核心版本。`IsParseable(""/"dev"/"latest")` = false → 自更新检查时视为"有更新"。

## 自更新

- 二进制路径用 `os.Executable()+EvalSymlinks` 解析（`cmd/mirror/main.go resolveBinaryPath`），不要回退到 `os.Args[0]`。
- 容器内自更新被禁止（检测 `/.dockerenv` / `container` env），Status 里给 `apply_blocked_reason`。
- 状态持久化在 `<storage>/selfupdate_state.json`，重启对账靠其中的 `applied_version`。
- 重启用 `syscall.Exec`（unix，`restart_unix.go`）原地替换进程；Windows 走 `restart_other.go`（os.StartProcess）。`SetOnRestart` 在 main.go 注入闭包，需绝对路径（解析失败才回退 `os.Args[0]`）。
- 频道：`notify` / `release` / `preview`（`NormalizeSelfUpdateChannel`）。
- **下载链接内置（2026-08-14 重构）**：`Apply()` 不再调 `GetReleaseByTag` API，改为 `buildUpdateCandidates` 按 CI 资产命名规则（`mirror-{goos}-{label}{ext}`）直接构造 `https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}`。候选列表：先裸二进制，404 回退压缩包（`.tar.gz`/`.zip`，兼容旧版 alpha Release 仅含压缩包的情况），解压用 `extractFromTarGz`/`extractFromZip`。代理链：`httpClient`（`buildHTTPClient`）处理 HTTP 代理（`proxy_url` / `asset_proxy_url` 是代理时），`buildUpdateCandidates` 处理镜像前缀拼接（`asset_proxy_url` 不是代理时）——两条路径互补，不要漏。
- **自动更新默认开启（2026-08-14）**：`DefaultConfig` 改为 `SelfUpdateEnabled=true`/`Channel="release"`/`AutoRestart=true`/`CheckCron="0 */6 * * *"`。cron 回调不再仅 `Check()`，`CanApply` 为 true 时自动调 `Apply()`（main.go）。
- **配置项缺失自动补充（2026-08-14）**：`LoadConfig` 加载后用 `cfg.renderYAML()` 对比磁盘文件，不一致则重写（新版本新增字段自动补齐）。`Save` 重构提取 `renderYAML` 复用。

## 嵌入前端释放

- `embedded_files.go`：`//go:embed all:web/default all:web/admin`。启动时 `assets.SyncEmbedded` 把内嵌前端释放到项目根 `web/default`、`web/admin`，**每次启动都释放**（内容相同的文件跳过写入减 IO，无 manifest 短路，确保前端随二进制即时生效）。`deprecatedBundles` 列表里的历史遗留目录（如 `web/default_v2`）会被整体删除。

## 下载 URL 生成（CDN 优先，2026-08-14 加）

- 配置 `cdn_base_url`（如 `https://cdn.foldcraftlauncher.cn`，空 = 纯相对路径，行为与旧版完全一致）。`download_url_base` 另有用（`FixAssetURLs` 重写启动器 index.json 里的资产 URL），**不要混用**。
- `s.buildDownloadURLs(token, filePath)`（server.go）生成候选数组：配置了 CDN → `[CDN绝对URL, 同源相对路径]`；未配置 → `[相对路径]`。`downloadTokenResponse` 新增 `download_urls` 字段（prepare/authorize/landing 三处都带），`download_url` 仍是单值 = 首选候选（兼容旧客户端）。
- 前端降级策略（DownloadStartedView.vue，PoW/CLI 链路的最终落点）：`pickDownloadUrl` 按序 HEAD 探测候选（剥离 query 的下载路径，5s 超时 AbortController），CDN 不通自动退回同源直连；按钮 href 用探测后的 `downloadTarget`。**HEAD 探测分支（server.go）不校验 token、不记账、不写 download_events**，探测请求零成本、零污染；不要改成带 token 的 GET/Range 探测（会写事件+防刷墙记账）。
- **CDN 缓存头（2026-08-15 加）**：配置 `cdn_cache_max_age`（秒，默认 604800，负值归一）后，配了 `cdn_base_url` 的下载 GET 响应改发 `Cache-Control: public, max-age=N`（server.go 下载处理器）；否则维持 `private, no-store`。路径含 `launcher/version/file` 缓存键稳定，配合 CDN（EdgeOne）忽略 query string 按路径缓存即可让大文件由边缘供流。**代价**：缓存命中不回源，PoW 校验/防刷墙/download_events 计数失效，下载次数/流量只能从 CDN 日志拉；HEAD 探测分支不设缓存头（本就不回源记账）。
