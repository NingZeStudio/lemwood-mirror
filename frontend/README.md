# 柠枺镜像状态 (Lemwood Mirror Status)

基于 **Vue 3 + Vite** 的镜像服务状态展示前端项目。

## 技术栈

- **框架**: Vue 3 (使用 `<script setup>` SFC)
- **构建工具**: Vite 5
- **路由**: Vue Router 4 (History 模式)
- **UI**: Tailwind CSS + Radix Vue
- **HTTP**: Axios

## 开发指南

### 安装依赖

```bash
npm install
```

### 启动开发服务器

```bash
npm run dev
```

### 生产构建

```bash
npm run build
```

### 预览构建结果

```bash
npm run preview
```

## 部署说明

### 服务器配置要求

本项目使用 Vue Router 的 **History 模式** (`createWebHistory`)，需要在服务器端配置 URL 重写规则，确保所有非文件/文件夹请求都交给 `index.html` 处理。

#### Nginx 配置示例

```nginx
server {
    listen 80;
    server_name your-domain.com;
    root /path/to/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    # API 代理 (可选)
    location /api {
        proxy_pass http://backend-api;
    }
}
```

#### Apache 配置示例

在 `dist/` 目录下创建 `.htaccess` 文件:

```apache
<IfModule mod_rewrite.c>
  RewriteEngine On
  RewriteBase /
  RewriteRule ^index\.html$ - [L]
  RewriteCond %{REQUEST_FILENAME} !-f
  RewriteCond %{REQUEST_FILENAME} !-d
  RewriteRule . /index.html [L]
</IfModule>
```

#### Vercel 配置

创建 `vercel.json`:

```json
{
  "rewrites": [
    { "source": "/:path*", "destination": "/index.html" }
  ]
}
```

#### Netlify 配置

创建 `public/_redirects` 文件:

```
/*    /index.html   200
```

### 环境变量

构建前设置环境变量:

```bash
VITE_API_BASE_URL=https://your-api.com/api npm run build
```

或创建 `.env.production` 文件:

```
VITE_API_BASE_URL=https://your-api.com/api
```

## 项目结构

```
web/
├── src/
│   ├── assets/            # 静态资源
│   ├── components/        # Vue 组件
│   ├── layouts/           # 布局组件
│   ├── lib/               # 工具函数
│   ├── router/            # 路由配置
│   ├── services/          # API 服务
│   ├── views/             # 页面视图
│   ├── App.vue            # 根组件
│   ├── main.js            # 入口文件
│   └── style.css          # 全局样式
├── admin/                 # 管理后台
├── public/                # 公共静态资源
├── index.html             # HTML 模板
├── vite.config.js         # Vite 配置
├── tailwind.config.js     # Tailwind 配置
└── package.json           # 依赖配置
```

## 路由

| 路径 | 说明 |
|------|------|
| `/` | 首页/状态概览 |
| `/files` | 文件列表 |
| `/files/:launcherName` | 启动器文件 |
| `/files/:launcherName/:versionName` | 版本文件 |
| `/stats` | 统计信息 |
| `/api` | API 文档 |
| `/about` | 关于页面 |

## 许可证

MIT
