<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  ArrowUpToLine,
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
import { getStatus, getLatest, getCaptchaConfig, prepareDownload } from '@/services/api'
import { getLauncherDisplayName } from '@/lib/launcher-info'
import { cn } from '@/lib/utils'
import { globalConfig } from '@/lib/globalConfig'
import Badge from '@/components/ui/Badge.vue'
import Button from '@/components/ui/Button.vue'
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
  if (!dateString) return ''
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

const formatSize = (bytes) => {
  if (bytes == null) return ''
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

const copyUrl = (url) => copy(url)

const handleDownload = async (item) => {
  const launcherName = currentPath.value[0]?.id
  const versionName = currentPath.value[1]?.id
  if (!launcherName || !versionName) {
    window.open(item.downloadUrl, '_blank')
    return
  }

  const filePath = `${launcherName}/${versionName}/${item.name}`
  const returnUrl = window.location.href
  const source = globalConfig.download.sourceLabels.files

  if (!captchaConfig.value.enabled) {
    try {
      const response = await prepareDownload(filePath, returnUrl, source)
      const token = response.data.download_token
      if (token) {
        router.push(`/download-started?token=${token}`)
      } else {
        window.open(response.data.download_url || item.downloadUrl, '_blank')
      }
    } catch (error) {
      console.error('Prepare download error:', error)
      window.open(item.downloadUrl, '_blank')
    }
    return
  }

  router.push(`/verify?file=${encodeURIComponent(filePath)}&return_url=${encodeURIComponent(returnUrl)}&source=${encodeURIComponent(source)}`)
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
            : `${globalConfig.download.baseUrl}/download/${launcherName}/${versionName}/${asset.name}`
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

watch([() => props.launcherName, () => props.versionName, currentPath], () => {
  updateMetaInfo()
}, { deep: true })

const updateMetaInfo = () => {
  const nameFull = globalConfig.site.nameFull
  const baseTitle = `文件列表 - ${nameFull}`
  let title = baseTitle
  let description = '浏览和下载 Minecraft 启动器版本文件'

  if (currentPath.value.length === 1) {
    const launcher = currentPath.value[0]
    title = `${launcher.name} - ${nameFull}`
    description = `浏览 ${launcher.name} 的所有版本`
  } else if (currentPath.value.length >= 2) {
    const launcher = currentPath.value[0]
    const version = currentPath.value[1]
    title = `${version.name} - ${launcher.name} - ${nameFull}`
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
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div class="space-y-1">
        <h1 class="text-3xl font-bold tracking-tight">文件浏览</h1>
        <p class="text-sm text-muted-foreground">按启动器、版本和文件层级浏览镜像资源。</p>
      </div>
      <div class="relative w-full sm:w-64">
        <Search class="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input v-model="searchQuery" type="search" placeholder="筛选..." class="pl-9" />
      </div>
    </div>

    <div class="overflow-hidden rounded-lg border bg-card text-card-foreground shadow-sm">
      <div class="flex items-center gap-1 border-b px-3 py-2 text-sm text-muted-foreground">
        <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="navigateToBreadcrumb(-1)">
          <Home class="mr-1 h-3.5 w-3.5" />
          根目录
        </Button>
        <template v-for="(crumb, index) in currentPath" :key="crumb.id">
          <ChevronRight class="h-3.5 w-3.5 shrink-0" />
          <Button variant="ghost" size="sm" class="h-7 px-2 text-xs" @click="navigateToBreadcrumb(index)">
            {{ crumb.name }}
          </Button>
        </template>
        <span v-if="copied" class="ml-auto text-xs text-muted-foreground">链接已复制</span>
      </div>

      <div v-if="loading" class="divide-y">
        <div v-for="i in 8" :key="i" class="flex items-center gap-3 px-4 py-3">
          <Skeleton class="h-8 w-8 shrink-0 rounded" />
          <div class="min-w-0 flex-1 space-y-1.5">
            <Skeleton class="h-4 w-3/5" />
            <Skeleton class="h-3 w-2/5" />
          </div>
          <Skeleton class="h-3 w-16 shrink-0" />
        </div>
      </div>

      <div v-else-if="!currentItems.length" class="flex flex-col items-center gap-3 px-4 py-16 text-muted-foreground">
        <Folder class="h-10 w-10 opacity-40" />
        <p class="text-sm font-medium text-foreground">空文件夹</p>
        <p class="text-xs">没有找到匹配的项目。</p>
      </div>

      <div v-else class="divide-y">
        <button
          v-if="currentPath.length > 0"
          type="button"
          class="flex w-full items-center gap-3 px-4 py-3 text-left text-sm text-muted-foreground transition-colors hover:bg-accent"
          @click="navigateUp"
        >
          <ArrowUpToLine class="h-4 w-4" />
          <span>返回上一级</span>
        </button>

        <div
          v-for="item in currentItems"
          :key="item.id"
          :class="cn(
            'flex items-center gap-3 px-4 py-2.5 transition-colors',
            item.type !== 'file' ? 'cursor-pointer hover:bg-accent/50' : 'hover:bg-muted/30'
          )"
          @click="item.type !== 'file' ? navigateTo(item, item.type) : null"
        >
          <div class="flex shrink-0 items-center justify-center rounded-md bg-primary/10 p-1.5 text-primary">
            <Folder v-if="item.type === 'launcher' || item.type === 'version'" class="h-4 w-4" />
            <component v-else :is="getFileIcon(item.name)" class="h-4 w-4" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="truncate text-sm font-medium" :title="item.name">{{ item.name }}</span>
              <Badge v-if="item.isLatest" variant="success" class="text-[10px] leading-none">Latest</Badge>
            </div>
            <p v-if="item.type === 'launcher'" class="mt-0.5 text-xs text-muted-foreground">{{ item.count }} 个版本</p>
            <p v-else-if="item.type === 'version'" class="mt-0.5 text-xs text-muted-foreground">{{ formatDate(item.date) }} · {{ item.fileCount }} 个文件</p>
          </div>

          <span v-if="item.type === 'file' && item.size != null" class="shrink-0 text-xs text-muted-foreground">
            {{ formatSize(item.size) }}
          </span>

          <div v-if="item.type === 'file'" class="flex shrink-0 gap-0.5">
            <Button size="icon" variant="ghost" class="h-7 w-7" @click.stop="copyUrl(item.downloadUrl)">
              <Copy class="h-3.5 w-3.5" />
              <span class="sr-only">复制链接</span>
            </Button>
            <Button size="icon" variant="ghost" class="h-7 w-7" @click.stop="handleDownload(item)">
              <Download class="h-3.5 w-3.5" />
              <span class="sr-only">下载</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
