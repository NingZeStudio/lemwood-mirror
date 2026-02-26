# QWEN.md - 项目上下文指南

## 项目概述

这是一个基于 **Vue 3 + Vite** 的前端项目，名为 "柠枺镜像状态" (Lemwood Mirror Status)。该项目是一个镜像服务状态展示平台，提供文件浏览、统计信息展示和 API 文档等功能。

### 技术栈

| 类别 | 技术 |
|------|------|
| **框架** | Vue 3 (使用 `<script setup>` SFC) |
| **构建工具** | Vite 5 |
| **路由** | Vue Router 4 (History 模式) |
| **HTTP 客户端** | Axios |
| **UI 框架** | Tailwind CSS 3 + Radix Vue |
| **图表** | Chart.js + ECharts |
| **工具库** | VueUse, class-variance-authority, lucide-vue-next |
| **样式** | Sass, PostCSS, Autoprefixer |

### 项目结构

```
web/
├── src/                    # 源代码目录
│   ├── assets/            # 静态资源
│   ├── components/        # Vue 组件
│   │   ├── layout/        # 布局组件
│   │   ├── ui/            # UI 基础组件
│   │   ├── Announcements.vue
│   │   ├── CookiesConsent.vue
│   │   └── VersionList.vue
│   ├── layouts/           # 页面布局
│   ├── lib/               # 工具库
│   │   ├── launcher-info.ts
│   │   └── utils.js       # cn() 类名合并工具
│   ├── router/            # 路由配置
│   ├── services/          # API 服务层
│   │   └── api.js         # Axios 实例和 API 方法
│   ├── views/             # 页面视图
│   │   ├── HomeView.vue   # 首页
│   │   ├── FilesView.vue  # 文件浏览
│   │   ├── StatsView.vue  # 统计信息
│   │   ├── ApiDocsView.vue # API 文档
│   │   └── AboutView.vue  # 关于页面
│   ├── App.vue            # 根组件
│   ├── main.js            # 入口文件
│   └── style.css          # 全局样式
├── admin/                  # 管理后台 (独立)
├── public/                 # 公共静态资源
├── scripts/                # 脚本工具
├── _legacy/                # 旧版本代码
├── dist/                   # 构建输出目录
├── index.html              # HTML 模板
├── package.json            # 依赖配置
├── vite.config.js          # Vite 配置
├── tailwind.config.js      # Tailwind 配置
├── postcss.config.js       # PostCSS 配置
└── .env.production         # 生产环境变量
```

## 构建与运行

### 开发模式

```bash
npm run dev
```

启动 Vite 开发服务器，支持热模块替换 (HMR)。

### 生产构建

```bash
npm run build
```

使用 Vite 构建生产版本，输出到 `dist/` 目录。

### 预览构建

```bash
npm run preview
```

本地预览生产构建结果。

### 环境配置

生产环境 API 地址配置在 `.env.production`:

```
VITE_API_BASE_URL=https://mirror.lemwood.icu/api
```

开发环境可在项目根目录创建 `.env` 或 `.env.local` 文件覆盖此配置。

## 路由配置

项目使用 **History 模式** 路由 (`createWebHistory`)，需要服务器配置 URL 重写。

每个路由都有独立的页面标题，通过路由守卫自动更新 `document.title`。

| 路径 | 名称 | 组件 | Page Title |
|------|------|------|-----------|
| `/` | home | HomeView | 首页 - 柠枺镜像状态 |
| `/files` | files | FilesView | 文件列表 - 柠枺镜像状态 |
| `/files/:launcherName` | files-launcher | FilesView | 文件列表 - 柠枺镜像状态 |
| `/files/:launcherName/:versionName` | files-version | FilesView | 文件列表 - 柠枺镜像状态 |
| `/stats` | stats | StatsView | 统计信息 - 柠枺镜像状态 |
| `/api` | api | ApiDocsView | API 文档 - 柠枺镜像状态 |
| `/about` | about | AboutView | 关于 - 柠枺镜像状态 |
| `/*` | not-found | HomeView | 页面未找到 - 柠枺镜像状态 |

## API 服务

API 服务层位于 `src/services/api.js`, 提供以下方法:

```javascript
getStatus()      // GET /status - 获取服务状态
getLatest()      // GET /latest - 获取最新版本信息
getStats()       // GET /stats  - 获取统计数据
getFiles(path)   // GET /files?path= - 获取文件列表
scan()           // POST /scan - 触发扫描
```

## 开发规范

### 代码风格

- **Vue 组件**: 使用 `<script setup>` 语法糖
- **路径别名**: 使用 `@/` 指向 `src/` 目录
- **类名合并**: 使用 `cn()` 工具函数 (基于 `clsx` + `tailwind-merge`)

### UI 组件

项目使用了 **Radix Vue** 作为基础 UI 组件库，配合 Tailwind CSS 进行样式定制。主题色采用 CSS 变量定义 (HSL 格式),支持深色模式。

### 注意事项

1. 路由使用 History 模式，需要服务器配置 URL 重写（所有非文件请求交给 `index.html`）
2. 构建时 `base` 配置为 `/`, 部署在根路径
3. 项目包含 TypeScript 文件 (`.ts`), 但主要为 JavaScript 项目
4. 页面标题通过 `router.beforeEach` 守卫自动更新
5. 每个页面都有独立的 SEO meta 信息更新逻辑

## SEO 优化

### 全局 Meta 标签

`index.html` 中配置了完整的 SEO meta 信息:

- **基础信息**: title, description, keywords, author
- **Open Graph**: og:type, og:url, og:title, og:description, og:image
- **Twitter Card**: twitter:card, twitter:url, twitter:title, twitter:description, twitter:image

### 动态 Meta 更新

每个视图组件在 `onMounted` 钩子中更新 meta 信息:

| 页面 | Title | Description |
|------|-------|-------------|
| HomeView | 首页 - 柠枺镜像状态 | 提供多种 Minecraft 启动器版本的镜像下载服务 |
| FilesView | 动态标题 (根据路径) | 根据浏览层级动态更新 |
| StatsView | 统计信息 - 柠枺镜像状态 | 查看柠枺镜像站的访问统计、下载统计和地理分布数据 |
| ApiDocsView | API 文档 - 柠枺镜像状态 | 柠枺镜像站 API 接口文档 |
| AboutView | 关于 - 柠枺镜像状态 | 了解柠枺镜像站背后的团队、技术栈和项目故事 |

### FilesView 动态标题规则

- 根目录：`文件列表 - 柠枺镜像状态`
- 启动器目录：`{启动器名称} - 柠枺镜像状态`
- 版本目录：`{版本号} - {启动器名称} - 柠枺镜像状态`

## 相关目录

- **admin/**: 独立的管理后台，使用传统 HTML/JS/CSS 结构
- **_legacy/**: 旧版本代码备份，包含 `404.html`, `app.js`, `index.html` 等
