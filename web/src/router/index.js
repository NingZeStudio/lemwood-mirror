import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import { globalConfig } from '@/lib/globalConfig'

const T = (name) => `${name} - ${globalConfig.site.nameFull}`

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { title: T('首页') }
    },
    {
      path: '/verify',
      name: 'verify',
      component: () => import('@/views/VerifyView.vue'),
      meta: { title: T('安全验证') }
    },
    {
      path: '/files',
      name: 'files',
      component: () => import('@/views/FilesView.vue'),
      meta: { title: T('文件列表') }
    },
    {
      path: '/files/:launcherName',
      name: 'files-launcher',
      component: () => import('@/views/FilesView.vue'),
      props: true,
      meta: { title: T('文件列表') }
    },
    {
      path: '/files/:launcherName/:versionName',
      name: 'files-version',
      component: () => import('@/views/FilesView.vue'),
      props: true,
      meta: { title: T('文件列表') }
    },
    {
      path: '/stats',
      name: 'stats',
      component: () => import('@/views/StatsView.vue'),
      meta: { title: T('统计信息') }
    },
    {
      path: '/api',
      name: 'api',
      component: () => import('@/views/ApiDocsView.vue'),
      meta: { title: T('API 文档') }
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('@/views/AboutView.vue'),
      meta: { title: T('关于') }
    },
    {
      path: '/download-started',
      name: 'download-started',
      component: () => import('@/views/DownloadStartedView.vue'),
      meta: { title: T('下载已开始') }
    },
    {
       path: '/:pathMatch(.*)*',
       name: 'not-found',
       component: () => import('@/views/HomeView.vue'),
       meta: { title: T('页面未找到') }
    }
  ]
})

router.beforeEach((to, from, next) => {
  document.title = to.meta.title || globalConfig.site.nameFull
  next()
})

export default router
