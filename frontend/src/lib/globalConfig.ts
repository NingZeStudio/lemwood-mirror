export const globalConfig = {
  site: {
    name: '柠枺镜像',
    nameFull: '柠枺镜像状态',
    nameEn: 'Lemwood Mirror',
    version: '3.15.0',
    description: 'Minecraft 启动器版本镜像站',
    url: 'https://beta.miawa.cn/',
    language: 'zh-CN',
    author: 'Lemwood & QiTry'
  },

  contact: {
    email: '3436464181@qq.com',
    qq: '3436464181',
    qqGroup: '1077373741'
  },

  links: {
    qqGroup: 'https://qm.qq.com/q/WMXCSUhU4O',
    githubRepo: 'https://github.com/leemwood/lemwood_mirror/',
    githubRepoNewWeb: 'https://github.com/JanePHPDev/lemwood_mirror_NewWeb',
    githubOrg: 'https://github.com/NingZeStudio/lemwood-mirror',
    logshare: 'https://logshare.cn/',
    beian: 'https://beian.miit.gov.cn/'
  },

  legal: {
    icp: '新ICP备2024015133号-6',
    cookieConsent: '本网站使用 Cookies 以优化您的体验。继续使用即表示您同意我们的 Cookie 政策。'
  },

  api: {
    baseUrl: import.meta.env.VITE_API_BASE_URL || '/api/v2',
    endpoints: {
      status: '/launchers',
      latest: '/latest',
      stats: '/stats',
      files: '/files',
      scan: '/admin/scans',
      captchaConfig: '/captcha/config',
      downloadVerify: '/download/verify',
      downloadPrepare: '/download/prepare',
      downloadLanding: '/download/landing'
    }
  },

  launchers: {
    zl: {
      displayName: 'ZalithLauncher',
      logoUrl: new URL('../assets/images/34c1ec9e07f826df.webp', import.meta.url).href
    },
    zl2: {
      displayName: 'ZalithLauncher2',
      logoUrl: new URL('../assets/images/ee0028bd82493eb3.webp', import.meta.url).href
    },
    hmcl: {
      displayName: 'Hello Minecraft! Launcher',
      logoUrl: new URL('../assets/images/3835841e4b9b7abf.jpeg', import.meta.url).href
    },
    MG: {
      displayName: 'MobileGlues',
      logoUrl: new URL('../assets/images/3625548d2639a024.png', import.meta.url).href
    },
    fcl: {
      displayName: 'FoldCraftLauncher',
      logoUrl: new URL('../assets/images/dc5e0ee14d8f54f0.png', import.meta.url).href
    },
    FCL_Turnip: {
      displayName: 'FCL_Turnip Plugin',
      logoUrl: new URL('../assets/images/Image_1770256620866_693.webp', import.meta.url).href
    },
    shizuku: {
      displayName: 'Shizuku',
      logoUrl: new URL('../assets/images/f7067665f073b4cc.png', import.meta.url).href
    },
    leaves: {
      displayName: 'Leaves 服务端',
      logoUrl: new URL('../assets/images/Leaves.png', import.meta.url).href
    },
    leaf: {
      displayName: 'Leaf 服务端',
      logoUrl: new URL('../assets/images/leaf.png', import.meta.url).href
    },
    luminol: {
      displayName: 'Luminol 服务端',
      logoUrl: new URL('../assets/images/c25a955166388e1257c23d01c78a62e6.webp', import.meta.url).href
    },
    'authlib-injector': {
      displayName: '【开发者】authlib-injector 库',
      logoUrl: new URL('../assets/images/authlib-injector.png', import.meta.url).href
    },
    fcl_dl: {
      displayName: 'FoldCraftLauncher (DL)',
      logoUrl: new URL('../assets/images/dc5e0ee14d8f54f0.png', import.meta.url).href
    },
    fcl_di: {
      displayName: '【直装版】FoldCraftLauncher',
      logoUrl: new URL('../assets/images/dc5e0ee14d8f54f0.png', import.meta.url).href
    },
    aamc: {
      displayName: 'Angel Aura Amethyst',
      logoUrl: new URL('../assets/images/amethyst.png', import.meta.url).href
    }
  },

  download: {
    baseUrl: 'https://beta.miawa.cn',
    sourceLabels: {
      home: 'home-latest-download',
      files: 'files-download',
      verify: 'verify-download'
    }
  },

  theme: {
    defaultColor: 'monochrome',
    colors: [
      { name: '水墨', value: 'monochrome' },
      { name: '海洋', value: 'blue' },
      { name: '薰衣草', value: 'purple' },
      { name: '森林', value: 'green' },
      { name: '日落', value: 'orange' },
      { name: '樱花', value: 'pink' }
    ]
  },

  storage: {
    keys: {
      themeColor: 'theme-color',
      cookiesConsented: 'cookies-consented',
      darkMode: 'vueuse-color-scheme',
      announcementShown: 'lemwood_announcement_shown',
      lastAnnouncementId: 'lemwood_last_announcement_id'
    }
  }
}
