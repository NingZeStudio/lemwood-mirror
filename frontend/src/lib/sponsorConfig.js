export const sponsorConfig = {
  title: '赞助支持',
  description: '您的支持会用于 Lemwood Mirror 的服务器、带宽、镜像存储和其他基础设施支出。',
  afdianLink: '',
  alipayQrCode: new URL('@/assets/images/alipay-sponsor.jpg', import.meta.url).href,
  wechatQrCode: new URL('@/assets/images/wechat-sponsor.png', import.meta.url).href
}

export const sponsors = [
  {
    id: 4,
    name: 'Swung 0x48',
    amount: 1000,
    currency: 'CNY',
    date: '2026-04-26',
    platform: 'wechat',
    pinned: true
  },
  {
    id: 1,
    name: 'ksgf452',
    amount: 40,
    currency: 'CNY',
    date: '2026-03-08',
    platform: 'alipay'
  },
  {
    id: 2,
    name: '@习习中',
    amount: 34,
    currency: 'CNY',
    date: '2026-03-08',
    platform: 'wechat'
  },
  {
    id: 3,
    name: '@WZW_王宗王',
    amount: 50,
    currency: 'CNY',
    date: '2026-03-22',
    platform: 'wechat',
    message: 'Bilibili UP 主 UID: 504274118'
  },
  {
    id: 5,
    name: '菜花',
    amount: 10,
    currency: 'CNY',
    date: '2026-04-26',
    platform: 'wechat'
  },
  {
    id: 6,
    name: '懵逼的兔子',
    amount: 5,
    currency: 'CNY',
    date: '2026-04-26',
    platform: 'wechat'
  },
  {
    id: 7,
    name: '我的世界良心君',
    amount: 5,
    currency: 'CNY',
    date: '2026-04-26',
    platform: 'wechat',
    message: 'Baidu 博主'
  },
  {
    id: 9,
    name: '苦瓜',
    amount: 10,
    currency: 'CNY',
    date: '2026-05-02',
    platform: 'wechat'
  },
  {
    id: 10,
    name: 'Janson',
    amount: 10,
    currency: 'CNY',
    date: '2026-05-02',
    platform: 'wechat'
  },
  {
    id: 8,
    name: '马铃薯_potato',
    amount: 1,
    currency: 'CNY',
    date: '2026-05-02',
    platform: 'wechat'
  }
]

export function getTotalAmount() {
  return sponsors.reduce((sum, sponsor) => sum + sponsor.amount, 0)
}

export function getSponsorCount() {
  return sponsors.length
}

export function getPlatformIcon(platform) {
  switch (platform) {
    case 'alipay':
      return 'Alipay'
    case 'wechat':
      return 'WeChat'
    case 'afdian':
      return '爱发电'
    default:
      return '匿名'
  }
}

export function getPlatformColor(platform) {
  switch (platform) {
    case 'alipay':
      return 'text-blue-500 bg-blue-500/10'
    case 'wechat':
      return 'text-green-500 bg-green-500/10'
    case 'afdian':
      return 'text-purple-500 bg-purple-500/10'
    default:
      return 'text-gray-500 bg-gray-500/10'
  }
}
