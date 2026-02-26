import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { title: '首页 - 柠枺镜像状态' }
    },
    {
      path: '/files',
      name: 'files',
      component: () => import('@/views/FilesView.vue'),
      meta: { title: '文件列表 - 柠枺镜像状态' }
    },
    {
      path: '/files/:launcherName',
      name: 'files-launcher',
      component: () => import('@/views/FilesView.vue'),
      props: true,
      meta: { title: '文件列表 - 柠枺镜像状态' }
    },
    {
      path: '/files/:launcherName/:versionName',
      name: 'files-version',
      component: () => import('@/views/FilesView.vue'),
      props: true,
      meta: { title: '文件列表 - 柠枺镜像状态' }
    },
    {
      path: '/stats',
      name: 'stats',
      component: () => import('@/views/StatsView.vue'),
      meta: { title: '统计信息 - 柠枺镜像状态' }
    },
    {
      path: '/api',
      name: 'api',
      component: () => import('@/views/ApiDocsView.vue'),
      meta: { title: 'API 文档 - 柠枺镜像状态' }
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('@/views/AboutView.vue'),
      meta: { title: '关于 - 柠枺镜像状态' }
    },
    {
       path: '/:pathMatch(.*)*',
       name: 'not-found',
       component: () => import('@/views/HomeView.vue'),
       meta: { title: '页面未找到 - 柠枺镜像状态' }
    }
  ]
})

router.beforeEach((to, from, next) => {
  document.title = to.meta.title || '柠枺镜像状态'
  next()
})

export default router
