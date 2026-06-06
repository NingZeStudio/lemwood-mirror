<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Download, GitFork, History, Package } from 'lucide-vue-next'
import { getStatus, getLatest, getCaptchaConfig, prepareDownload } from '@/services/api'
import { globalConfig } from '@/lib/globalConfig'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import Badge from '@/components/ui/Badge.vue'
import Skeleton from '@/components/ui/Skeleton.vue'

const router = useRouter()
const launcherDefaultLogo = globalConfig.launchers.fcl?.logoUrl

const rawLaunchers = ref({})
const latestMap = ref({})
const loading = ref(true)
const captchaConfig = ref({ enabled: false, app_id: '' })

const launcherList = computed(() => {
  return Object.keys(rawLaunchers.value).map((name) => {
    const versions = rawLaunchers.value[name]
    const latestVersion = latestMap.value[name]
    const latestObj = versions.find((v) => (v.tag_name || v.name) === latestVersion) || versions[0]
    const info = globalConfig.launchers[name] || { displayName: name, logoUrl: launcherDefaultLogo }

    const latestDownloadUrl = latestObj?.assets?.length
      ? getAssetUrl(name, latestObj, latestObj.assets[0])
      : '#'

    const cloneUrl = versions[0]?.clone_url || ''

    return {
      name,
      displayName: info.displayName,
      logoUrl: info.logoUrl,
      versions,
      latest: latestVersion,
      lastUpdated: versions.length ? versions[0].published_at : null,
      hasAssets: Boolean(latestObj?.assets?.length),
      latestObj,
      latestDownloadUrl,
      cloneUrl
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
  const source = globalConfig.download.sourceLabels.home

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

const copyCloneUrl = (url) => {
  navigator.clipboard.writeText(`git clone ${url}`)
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

    <div v-if="loading && !launcherList.length" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card v-for="i in 4" :key="i">
        <div class="flex items-start gap-4 p-4">
          <Skeleton class="h-12 w-12 shrink-0 rounded-lg" />
          <div class="min-w-0 flex-1 space-y-2">
            <Skeleton class="h-5 w-3/4" />
            <Skeleton class="h-4 w-1/2" />
          </div>
        </div>
        <div class="flex items-center justify-end gap-2 border-t px-4 py-3">
          <Skeleton class="h-8 w-28" />
          <Skeleton class="h-8 w-24" />
        </div>
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
        class="transition-shadow hover:shadow-md"
      >
        <div class="flex items-start gap-4 p-4">
          <img
            :src="item.logoUrl"
            class="h-12 w-12 shrink-0 rounded-lg border bg-background object-contain p-1.5 shadow-sm"
            :alt="item.displayName"
          />
          <div class="min-w-0 flex-1 space-y-1.5">
            <div class="flex items-center gap-2">
              <span class="truncate text-base font-semibold text-foreground">{{ item.displayName }}</span>
              <span v-if="item.latest" class="shrink-0 text-xs text-muted-foreground">
                {{ item.latest }}
              </span>
            </div>
            <p class="text-sm text-muted-foreground">最近更新：{{ formatDate(item.lastUpdated) }}</p>
          </div>
        </div>
        <div class="flex items-center justify-end gap-2 border-t px-4 py-3">
          <Button
            v-if="item.hasAssets"
            size="sm"
            @click="handleDownload(item)"
          >
            <Download class="mr-1.5 h-3.5 w-3.5" />
            下载最新版
          </Button>
          <Button
            v-else
            size="sm"
            disabled
          >
            <Download class="mr-1.5 h-3.5 w-3.5" />
            暂无资源
          </Button>
          <Button
            variant="outline"
            size="sm"
            @click="$router.push(`/files/${item.name}`)"
          >
            <History class="mr-1.5 h-3.5 w-3.5" />
            历史版本
          </Button>
          <Button
            v-if="item.cloneUrl"
            variant="outline"
            size="sm"
            @click="copyCloneUrl(item.cloneUrl)"
          >
            <GitFork class="mr-1.5 h-3.5 w-3.5" />
            克隆仓库
          </Button>
        </div>
      </Card>
    </div>
  </div>
</template>
