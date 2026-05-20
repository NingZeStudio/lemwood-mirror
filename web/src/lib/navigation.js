import { BarChart2, FileText, Folder, Home, Info } from 'lucide-vue-next'

export const navigationLinks = [
  { name: '首页', path: '/', icon: Home, match: '/' },
  { name: '文件浏览', path: '/files', icon: Folder, match: '/files' },
  { name: '数据统计', path: '/stats', icon: BarChart2, match: '/stats' },
  { name: 'API 文档', path: '/api', icon: FileText, match: '/api' },
  { name: '关于', path: '/about', icon: Info, match: '/about' }
]

export function isNavigationActive(routePath, link) {
  if (link.match === '/') return routePath === '/'
  return routePath === link.path || routePath.startsWith(`${link.match}/`)
}
