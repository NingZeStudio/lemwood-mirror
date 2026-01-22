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
       path: '/:pathMatch(.*)*',
       redirect: '/'
    }
  ]
})

export default router
