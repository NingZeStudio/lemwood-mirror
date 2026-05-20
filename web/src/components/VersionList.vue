<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Download, History, Loader2, Package } from 'lucide-vue-next'
import { getStatus, getLatest, getCaptchaConfig, prepareDownload } from '@/services/api'
import { LAUNCHER_INFO_MAP } from '@/lib/launcher-info'
import { cn } from '@/lib/utils'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardFooter from '@/components/ui/CardFooter.vue'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'

import zlLogo from '@/assets/images/34c1ec9e07f826df.webp'
import zl2Logo from '@/assets/images/ee0028bd82493eb3.webp'
import hmclLogo from '@/assets/images/3835841e4b9b7abf.jpeg'
import mgLogo from '@/assets/images/3625548d2639a024.png'
import fclLogo from '@/assets/images/dc5e0ee14d8f54f0.png'
import fclTurnipLogo from '@/assets/images/Image_1770256620866_693.webp'
import shizukuLogo from '@/assets/images/f7067665f073b4cc.png'
import luminolLogo from '@/assets/images/c25a955166388e1257c23d01c78a62e6.webp'
import leafLogo from '@/assets/images/leaf.png'
import leavesLogo from '@/assets/images/Leaves.png'
import authlibinjectorLogo from '@/assets/images/authlib-injector.png'
import amethystLogo from '@/assets/images/amethyst.png'

const router = useRouter()

const EXTENDED_LAUNCHER_INFO_MAP = {
  ...LAUNCHER_INFO_MAP,
  zl: { ...LAUNCHER_INFO_MAP.zl, logoUrl: zlLogo },
  zl2: { ...LAUNCHER_INFO_MAP.zl2, logoUrl: zl2Logo },
  hmcl: { ...LAUNCHER_INFO_MAP.hmcl, logoUrl: hmclLogo },
  MG: { ...LAUNCHER_INFO_MAP.MG, logoUrl: mgLogo },
  fcl: { ...LAUNCHER_INFO_MAP.fcl, logoUrl: fclLogo },
  FCL_Turnip: { ...LAUNCHER_INFO_MAP.FCL_Turnip, logoUrl: fclTurnipLogo },
  shizuku: { ...LAUNCHER_INFO_MAP.shizuku, logoUrl: shizukuLogo },
  leaves: { ...LAUNCHER_INFO_MAP.leaves, logoUrl: leavesLogo },
  leaf: { ...LAUNCHER_INFO_MAP.leaf, logoUrl: leafLogo },
  luminol: { ...LAUNCHER_INFO_MAP.luminol, logoUrl: luminolLogo },
  fcl_dl: { ...LAUNCHER_INFO_MAP.fcl_dl, logoUrl: fclLogo },
  fcl_di: { ...LAUNCHER_INFO_MAP.fcl_di, logoUrl: fclLogo },
  'authlib-injector': { ...LAUNCHER_INFO_MAP['authlib-injector'], logoUrl: authlibinjectorLogo },
  aamc: { ...LAUNCHER_INFO_MAP.aamc, logoUrl: amethystLogo }
}

const rawLaunchers = ref({})
const latestMap = ref({})
const loading = ref(true)
const captchaConfig = ref({ enabled: false, app_id: '' })

const launcherList = computed(() => {
  return Object.keys(rawLaunchers.value).map((name) => {
    const versions = rawLaunchers.value[name]
    const latestVersion = latestMap.value[name]
    const latestObj = versions.find((v) => (v.tag_name || v.name) === latestVersion) || versions[0]
    const info = EXTENDED_LAUNCHER_INFO_MAP[name] || { displayName: name, logoUrl: fclTurnipLogo }

    const latestDownloadUrl = latestObj?.assets?.length
      ? getAssetUrl(name, latestObj, latestObj.assets[0])
      : '#'

    return {
      name,
      displayName: info.displayName,
      logoUrl: info.logoUrl,
      versions,
      latest: latestVersion,
      lastUpdated: versions.length ? versions[0].published_at : null,
      hasAssets: Boolean(latestObj?.assets?.length),
      latestObj,
      latestDownloadUrl
    }
  })
})

const loadData = async () => {
  loading.value = true
  try {
    const [statusRes, latestRes, captchaRes] = await Promise.all([
      getStatus(),
      getLatest(),
      getCaptchaConfig().catch(() => ({ data: { enabled: false, app_id: '' } }))
    ])

    const data = statusRes.data
    for (const key in data) {
      data[key].sort((a, b) => String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name)))
    }

    rawLaunchers.value = data
    latestMap.value = latestRes.data
    captchaConfig.value = captchaRes.data
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const formatDate = (dateStr) => {
  if (!dateStr) return '未知时间'
  return new Date(dateStr).toLocaleDateString()
}

const getAssetUrl = (launcherName, version, asset) => {
  if (asset.url && (asset.url.startsWith('http://') || asset.url.startsWith('https://'))) {
    return asset.url
  }
  return `/download/${launcherName}/${version.tag_name || version.name}/${asset.name}`
}

const getAssetPath = (launcherName, version, asset) => {
  return `${launcherName}/${version.tag_name || version.name}/${asset.name}`
}

const handleDownload = async (item) => {
  if (!item.hasAssets || !item.latestObj) return

  const asset = item.latestObj.assets[0]
  const filePath = getAssetPath(item.name, item.latestObj, asset)
  const returnUrl = window.location.href
  const source = 'home-latest-download'

  if (!captchaConfig.value.enabled) {
    try {
      const response = await prepareDownload(filePath, returnUrl, source)
      const token = response.data.download_token
      if (token) {
        router.push(`/download-started?token=${token}`)
      } else {
        window.open(response.data.download_url || item.latestDownloadUrl, '_blank')
      }
    } catch (error) {
      console.error('Prepare download error:', error)
      window.open(item.latestDownloadUrl, '_blank')
    }
    return
  }

  router.push(`/verify?file=${encodeURIComponent(filePath)}&return_url=${encodeURIComponent(returnUrl)}&source=${encodeURIComponent(source)}`)
}

onMounted(() => {
  loadData()
})

defineExpose({ refresh: loadData })
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-2">
      <h2 class="text-3xl font-bold tracking-tight">版本探索</h2>
      <p class="text-muted-foreground">浏览最新启动器版本，快速下载或查看历史构建。</p>
    </div>

    <div v-if="loading && !launcherList.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <Card v-for="i in 4" :key="i" class="overflow-hidden">
        <Skeleton class="aspect-[16/10] w-full rounded-none" />
        <CardHeader>
          <Skeleton class="h-6 w-3/4" />
          <Skeleton class="h-4 w-1/2" />
        </CardHeader>
        <CardContent>
          <Skeleton class="h-4 w-full" />
        </CardContent>
        <CardFooter class="grid gap-2 sm:grid-cols-2">
          <Skeleton class="h-10 w-full" />
          <Skeleton class="h-10 w-full" />
        </CardFooter>
      </Card>
    </div>

    <div v-else-if="!launcherList.length" class="rounded-lg border border-dashed p-12 text-center text-muted-foreground">
      <Package class="mx-auto mb-4 h-12 w-12 opacity-40" />
      <p class="font-medium text-foreground">暂无数据</p>
      <p class="mt-1 text-sm">稍后再试，或检查接口连接。</p>
    </div>

    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card
        v-for="item in launcherList"
        :key="item.name"
        class="group overflow-hidden transition-shadow hover:shadow-md"
      >
        <div class="relative aspect-[16/10] overflow-hidden border-b bg-muted/30">
          <div class="absolute inset-0 bg-gradient-to-br from-background via-transparent to-primary/5"></div>
          <img
            :src="item.logoUrl"
            class="absolute inset-0 h-full w-full object-cover opacity-20 blur-2xl transition-transform duration-500 group-hover:scale-105"
            alt=""
          />
          <div class="absolute inset-0 flex items-center justify-center p-6">
            <img
              :src="item.logoUrl"
              class="h-20 w-20 rounded-xl border bg-background object-contain p-2 shadow-sm"
              :alt="item.displayName"
            />
          </div>
          <Badge v-if="item.latest" variant="success" class="absolute right-4 top-4">Latest {{ item.latest }}</Badge>
        </div>

        <CardHeader>
          <CardTitle class="text-xl">{{ item.displayName }}</CardTitle>
          <CardDescription>最近更新：{{ formatDate(item.lastUpdated) }}</CardDescription>
        </CardHeader>

        <CardContent class="space-y-3">
          <div class="flex items-center justify-between rounded-lg border bg-muted/30 px-3 py-2 text-sm">
            <span class="text-muted-foreground">版本数量</span>
            <span class="font-medium text-foreground">{{ item.versions.length }}</span>
          </div>
          <div class="flex items-center justify-between rounded-lg border bg-muted/30 px-3 py-2 text-sm">
            <span class="text-muted-foreground">最新资源</span>
            <span class="font-medium text-foreground">{{ item.hasAssets ? '可下载' : '无资源' }}</span>
          </div>
        </CardContent>

        <CardFooter class="grid gap-2 sm:grid-cols-2">
          <Button v-if="item.hasAssets" class="w-full" @click="handleDownload(item)">
            <Download class="mr-2 h-4 w-4" />
            下载最新版
          </Button>
          <Button v-else class="w-full" disabled>
            <Download class="mr-2 h-4 w-4" />
            暂无资源
          </Button>
          <Button variant="outline" class="w-full" @click="$router.push(`/files/${item.name}`)">
            <History class="mr-2 h-4 w-4" />
            历史版本
          </Button>
        </CardFooter>
      </Card>
    </div>
  </div>
</template>
