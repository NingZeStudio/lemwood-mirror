import { createRouter, createWebHashHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView
    },
    {
      path: '/files',
      name: 'files',
      component: () => import('@/views/FilesView.vue')
    },
    {
      path: '/files/:launcherName',
      name: 'files-launcher',
      component: () => import('@/views/FilesView.vue'),
      props: true
    },
    {
      path: '/files/:launcherName/:versionName',
      name: 'files-version',
      component: () => import('@/views/FilesView.vue'),
      props: true
    },
    {
      path: '/stats',
      name: 'stats',
      component: () => import('@/views/StatsView.vue')
    },
    {
      path: '/api',
      name: 'api',
      component: () => import('@/views/ApiDocsView.vue')
    },
    {
      path: '/about',
      name: 'about',
      component: () => import('@/views/AboutView.vue')
    },
    {
       path: '/:pathMatch(.*)*',
       redirect: '/'
    }
  ]
})

export default router
