<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  ChevronRight,
  Copy,
  Download,
  File,
  FileArchive,
  Folder,
  HardDrive,
  Home,
  Search
} from 'lucide-vue-next'
import { useClipboard } from '@vueuse/core'
import { getStatus, getLatest, getCaptchaConfig } from '@/services/api'
import { getLauncherDisplayName } from '@/lib/launcher-info'
import { cn } from '@/lib/utils'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Input from '@/components/ui/Input.vue'
import Skeleton from '@/components/ui/Skeleton.vue'

const props = defineProps({
  launcherName: String,
  versionName: String
})

const loading = ref(true)
const searchQuery = ref('')
const launchers = ref({})
const latestData = ref({})
const captchaConfig = ref({ enabled: false, app_id: '' })
const { copy, copied } = useClipboard()

const route = useRoute()
const router = useRouter()
const currentPath = ref([])

const loadData = async () => {
  loading.value = true
  try {
    const [statusRes, latestRes, captchaRes] = await Promise.all([
      getStatus(),
      getLatest(),
      getCaptchaConfig().catch(() => ({ data: { enabled: false, app_id: '' } }))
    ])

    const sortedLaunchers = {}
    Object.keys(statusRes.data)
      .sort()
      .forEach((key) => {
        sortedLaunchers[key] = statusRes.data[key].sort((a, b) =>
          String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name))
        )
      })

    launchers.value = sortedLaunchers
    latestData.value = latestRes.data
    captchaConfig.value = captchaRes.data
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const getFileIcon = (filename) => {
  const ext = filename.split('.').pop()?.toLowerCase()
  if (['zip', 'tar', 'gz', '7z', 'rar'].includes(ext)) return FileArchive
  if (['exe', 'msi', 'apk', 'dmg'].includes(ext)) return HardDrive
  return File
}

const formatDate = (dateString) => {
  if (!dateString) return '未知'
  try {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })
  } catch {
    return dateString
  }
}

const copyUrl = (url) => {
  copy(url)
}

const handleDownload = (item) => {
  if (!captchaConfig.value.enabled) {
    window.open(item.downloadUrl, '_blank')
    return
  }

  const launcherName = currentPath.value[0]?.id
  const versionName = currentPath.value[1]?.id
  if (!launcherName || !versionName) {
    window.open(item.downloadUrl, '_blank')
    return
  }

  const filePath = `${launcherName}/${versionName}/${item.name}`
  router.push(`/verify?file=${encodeURIComponent(filePath)}`)
}

const navigateTo = (item, type) => {
  if (type === 'launcher') {
    currentPath.value = [
      { name: getLauncherDisplayName(item.id), id: item.id, type: 'launcher', displayName: item.id }
    ]
  } else if (type === 'version') {
    currentPath.value.push({ name: item.name, id: item.id, type: 'version', data: item.data })
  }
  updateUrl()
}

const navigateUp = () => {
  currentPath.value.pop()
  updateUrl()
}

const navigateToBreadcrumb = (index) => {
  if (index === -1) {
    currentPath.value = []
    router.push({ name: 'files' })
  } else {
    currentPath.value = currentPath.value.slice(0, index + 1)
    updateUrl()
  }
}

const updateUrl = () => {
  if (currentPath.value.length === 0) {
    router.push({ name: 'files' })
  } else if (currentPath.value.length === 1) {
    router.push({ name: 'files-launcher', params: { launcherName: currentPath.value[0].id } })
  } else if (currentPath.value.length >= 2) {
    router.push({
      name: 'files-version',
      params: {
        launcherName: currentPath.value[0].id,
        versionName: currentPath.value[1].id
      }
    })
  }
}

const currentItems = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  const depth = currentPath.value.length

  if (depth === 0) {
    return Object.keys(launchers.value)
      .map((name) => ({
        id: name,
        name: getLauncherDisplayName(name),
        displayName: name,
        type: 'launcher',
        count: launchers.value[name].length,
        latest: latestData.value[name]
      }))
      .filter((l) => !query || l.name.toLowerCase().includes(query))
  }

  if (depth === 1) {
    const launcherName = currentPath.value[0].id
    const versions = launchers.value[launcherName] || []

    return versions
      .map((v) => ({
        id: v.tag_name || v.name,
        name: v.tag_name || v.name,
        type: 'version',
        date: v.published_at,
        isLatest: latestData.value[launcherName] === (v.tag_name || v.name),
        data: v,
        fileCount: v.assets?.length || 0
      }))
      .filter((v) => !query || v.name.toLowerCase().includes(query))
  }

  if (depth === 2) {
    const versionData = currentPath.value[1].data
    const launcherName = currentPath.value[0].id
    const versionName = currentPath.value[1].id

    return (versionData.assets || [])
      .map((asset) => ({
        id: asset.name,
        name: asset.name,
        type: 'file',
        size: asset.size,
        downloadUrl:
          asset.url && asset.url.startsWith('http')
            ? asset.url
            : `https://miawa.cn/download/${launcherName}/${versionName}/${asset.name}`
      }))
      .filter((f) => !query || f.name.toLowerCase().includes(query))
  }

  return []
})

onMounted(async () => {
  await loadData()

  if (props.launcherName && launchers.value[props.launcherName]) {
    currentPath.value = [
      {
        name: getLauncherDisplayName(props.launcherName),
        id: props.launcherName,
        type: 'launcher',
        displayName: props.launcherName
      }
    ]

    if (props.versionName) {
      const versions = launchers.value[props.launcherName] || []
      const versionData = versions.find((v) => (v.tag_name || v.name) === props.versionName)

      if (versionData) {
        currentPath.value.push({
          name: props.versionName,
          id: props.versionName,
          type: 'version',
          data: versionData
        })
      }
    }
  }
})

const updateMetaInfo = () => {
  const baseTitle = '文件列表 - 柠枺镜像状态'
  let title = baseTitle
  let description = '浏览和下载 Minecraft 启动器版本文件'

  if (currentPath.value.length === 1) {
    const launcher = currentPath.value[0]
    title = `${launcher.name} - 柠枺镜像状态`
    description = `浏览 ${launcher.name} 的所有版本`
  } else if (currentPath.value.length >= 2) {
    const launcher = currentPath.value[0]
    const version = currentPath.value[1]
    title = `${version.name} - ${launcher.name} - 柠枺镜像状态`
    description = `下载 ${launcher.name} ${version.name} 版本的资源文件`
  }

  document.title = title

  const metaDescription = document.querySelector('meta[name="description"]')
  const metaOgTitle = document.querySelector('meta[property="og:title"]')
  const metaOgDescription = document.querySelector('meta[property="og:description"]')
  const metaTwitterTitle = document.querySelector('meta[property="twitter:title"]')
  const metaTwitterDescription = document.querySelector('meta[property="twitter:description"]')

  if (metaDescription) metaDescription.setAttribute('content', description)
  if (metaOgTitle) metaOgTitle.setAttribute('content', title)
  if (metaOgDescription) metaOgDescription.setAttribute('content', description)
  if (metaTwitterTitle) metaTwitterTitle.setAttribute('content', title)
  if (metaTwitterDescription) metaTwitterDescription.setAttribute('content', description)
}

watch([() => props.launcherName, () => props.versionName, currentPath], () => {
  updateMetaInfo()
}, { deep: true })
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-2">
        <h1 class="text-3xl font-bold tracking-tight">文件浏览</h1>
        <p class="text-muted-foreground">按启动器、版本和文件层级浏览镜像资源。</p>
      </div>
      <div class="relative w-full sm:w-72">
        <Search class="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input v-model="searchQuery" type="search" placeholder="筛选当前目录..." class="pl-9" />
      </div>
    </div>

    <Card>
      <CardContent class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-center gap-1 overflow-x-auto text-sm text-muted-foreground">
          <Button variant="ghost" size="sm" class="shrink-0" @click="navigateToBreadcrumb(-1)">
            <Home class="mr-2 h-4 w-4" />
            根目录
          </Button>
          <template v-for="(crumb, index) in currentPath" :key="crumb.id">
            <ChevronRight class="h-4 w-4 shrink-0" />
            <Button variant="ghost" size="sm" class="shrink-0" @click="navigateToBreadcrumb(index)">
              {{ crumb.name }}
            </Button>
          </template>
        </div>
        <span v-if="copied" class="text-xs text-muted-foreground">链接已复制</span>
      </CardContent>
    </Card>

    <div v-if="loading" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <Skeleton v-for="i in 12" :key="i" class="h-20 rounded-lg" />
    </div>

    <div v-else-if="!currentItems.length" class="rounded-lg border border-dashed p-12 text-center text-muted-foreground">
      <Folder class="mx-auto mb-4 h-12 w-12 opacity-40" />
      <p class="font-medium text-foreground">空文件夹</p>
      <p class="mt-1 text-sm">没有找到匹配的项目。</p>
    </div>

    <div v-else class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <button
        v-if="currentPath.length > 0"
        type="button"
        class="flex items-center gap-3 rounded-lg border border-dashed bg-background p-4 text-left transition-colors hover:bg-accent hover:text-accent-foreground"
        @click="navigateUp"
      >
        <ArrowLeft class="h-5 w-5 text-muted-foreground" />
        <span class="text-sm font-medium">返回上一级</span>
      </button>

      <Card
        v-for="item in currentItems"
        :key="item.id"
        :class="cn('transition-colors hover:bg-accent/50', item.type !== 'file' ? 'cursor-pointer' : '')"
        @click="item.type !== 'file' ? navigateTo(item, item.type) : null"
      >
        <CardContent class="flex items-center gap-3 p-4">
          <div class="rounded-md bg-primary/10 p-2 text-primary">
            <Folder v-if="item.type === 'launcher' || item.type === 'version'" class="h-5 w-5" />
            <component v-else :is="getFileIcon(item.name)" class="h-5 w-5" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h3 class="truncate text-sm font-medium" :title="item.name">{{ item.name }}</h3>
              <Badge v-if="item.isLatest" variant="success">Latest</Badge>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">
              <span v-if="item.type === 'launcher'">{{ item.count }} 个版本</span>
              <span v-else-if="item.type === 'version'">{{ formatDate(item.date) }} · {{ item.fileCount }} 个文件</span>
              <span v-else>文件资源</span>
            </p>
          </div>

          <div v-if="item.type === 'file'" class="flex shrink-0 gap-1">
            <Button size="icon" variant="ghost" class="h-8 w-8" @click.stop="copyUrl(item.downloadUrl)">
              <Copy class="h-4 w-4" />
              <span class="sr-only">复制链接</span>
            </Button>
            <Button size="icon" variant="ghost" class="h-8 w-8" @click.stop="handleDownload(item)">
              <Download class="h-4 w-4" />
              <span class="sr-only">下载</span>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
