import { globalConfig } from '@/lib/globalConfig'

export const announcementConfig = {
  id: '20260527_mirror_strategy',
  title: '镜像策略调整通知',
  content:
    '由于服务器硬盘资源紧张，本站镜像策略由「全版本镜像」调整为「仅保留每个启动器最新三个版本」。更早的历史版本将逐步清理释放空间，如需特定旧版本请提前备份或联系我们。\n\n感谢您的理解与支持。',
  importantText: 'NingZeStudio 官方 QQ 群现已开放，群内急需可以为萌新分析日志、用户答疑解惑的好心人，欢迎加入交流。',
  links: [
    {
      label: '加入 QQ 群',
      url: 'https://qm.qq.com/q/WMXCSUhU4O'
    },
    {
      label: '查看文件',
      url: '/files'
    },
    {
      label: '数据统计',
      url: '/stats'
    }
  ]
}

const KEYS = {
  shown: globalConfig.storage.keys.announcementShown,
  lastId: globalConfig.storage.keys.lastAnnouncementId
}

export function hasSeenAnnouncement() {
  return (
    localStorage.getItem(KEYS.lastId) === announcementConfig.id &&
    localStorage.getItem(KEYS.shown) === 'true'
  )
}

export function markAnnouncementAsSeen() {
  localStorage.setItem(KEYS.shown, 'true')
  localStorage.setItem(KEYS.lastId, announcementConfig.id)
}

export function resetAnnouncement() {
  localStorage.removeItem(KEYS.shown)
  localStorage.removeItem(KEYS.lastId)
}
