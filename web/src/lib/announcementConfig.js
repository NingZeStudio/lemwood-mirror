export const announcementConfig = {
  enabled: true,
  id: '20260519_lemwood_announcement',
  title: '站点公告',
  content:
    '欢迎使用 Lemwood Mirror。我们会在这里同步站点维护、下载说明和重要更新。\n\n当公告内容更新时，只需要修改本配置中的 id，所有用户都会重新看到最新公告。',
  importantText: '请留意公告中的重要变更，避免错过下载与维护信息。',
  links: [
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

export const localStorageKeys = {
  announcementShown: 'lemwood_announcement_shown',
  lastAnnouncementId: 'lemwood_last_announcement_id'
}

function getStorage() {
  if (typeof window === 'undefined') return null

  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function hasSeenAnnouncement() {
  const storage = getStorage()
  if (!storage) return false

  const lastSeen = storage.getItem(localStorageKeys.lastAnnouncementId)
  const hasClosed = storage.getItem(localStorageKeys.announcementShown) === 'true'

  return lastSeen === announcementConfig.id && hasClosed
}

export function markAnnouncementAsSeen() {
  const storage = getStorage()
  if (!storage) return

  storage.setItem(localStorageKeys.announcementShown, 'true')
  storage.setItem(localStorageKeys.lastAnnouncementId, announcementConfig.id)
}

export function resetAnnouncement() {
  const storage = getStorage()
  if (!storage) return

  storage.removeItem(localStorageKeys.announcementShown)
  storage.removeItem(localStorageKeys.lastAnnouncementId)
}
