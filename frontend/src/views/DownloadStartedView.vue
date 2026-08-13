<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getDownloadLanding } from '@/services/api'
import { globalConfig } from '@/lib/globalConfig'
import { Download, Home, ArrowLeft, Loader2, Heart, Users, XCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const fileInfo = ref(null)
const downloadTriggered = ref(false)

const loadLandingInfo = async () => {
  const token = route.query.token
  if (!token) {
    error.value = '缺少下载凭证'
    loading.value = false
    return
  }

  try {
    const response = await getDownloadLanding(token)
    fileInfo.value = response.data
    triggerDownload()
  } catch (err) {
    error.value = err.response?.data?.message || '获取下载信息失败，凭证可能已过期'
  } finally {
    loading.value = false
  }
}

const triggerDownload = () => {
  if (downloadTriggered.value || !fileInfo.value?.download_url) return
  window.location.href = fileInfo.value.download_url
  downloadTriggered.value = true
}

// 返回来源网站（集成站）：优先外部 referrer，其次外部 return_url，兜底首页。
// 不 router.back() 回验证页。
const goBack = () => {
  const externalReferrer = getExternalReferrer()
  if (externalReferrer) {
    window.location.href = externalReferrer
    return
  }
  const returnUrl = route.query.return_url || fileInfo.value?.return_url
  if (returnUrl && isExternal(returnUrl)) {
    window.location.href = returnUrl
    return
  }
  router.push('/')
}

const goToWebsite = () => {
  const externalReferrer = getExternalReferrer()
  if (externalReferrer) {
    window.location.href = externalReferrer
    return
  }
  const returnUrl = route.query.return_url || fileInfo.value?.return_url
  if (returnUrl && isExternal(returnUrl)) {
    window.location.href = returnUrl
    return
  }
  router.push('/')
}

// getExternalReferrer 返回外部来源站点 URL（不同源才返回，避免回到本站）。
const getExternalReferrer = () => {
  const ref = document.referrer
  if (ref && isExternal(ref)) {
    return ref
  }
  return ''
}

// isExternal 判断 URL 是否来自其他源（站外）。
const isExternal = (url) => {
  try {
    return new URL(url, window.location.origin).origin !== window.location.origin
  } catch (e) {
    return false
  }
}

onMounted(() => {
  document.title = `下载已开始 - ${globalConfig.site.nameFull}`
  loadLandingInfo()
})
</script>

<template>
  <div class="flex min-h-[calc(100vh-10rem)] flex-col items-center justify-center gap-4 py-8">
    <Card class="w-full max-w-lg">
      <CardHeader class="items-center text-center">
        <div class="mb-2 rounded-full bg-primary/10 p-3 text-primary">
          <Download class="h-8 w-8" />
        </div>
        <CardTitle class="text-2xl">下载已开始</CardTitle>
        <CardDescription v-if="fileInfo?.file_name">
          正在为您下载 {{ fileInfo.file_name }}
        </CardDescription>
        <CardDescription v-else>
          正在为您获取下载信息...
        </CardDescription>
      </CardHeader>

      <CardContent class="space-y-6">
        <div v-if="loading" class="flex flex-col items-center justify-center gap-3 py-8 text-muted-foreground">
          <Loader2 class="h-8 w-8 animate-spin" />
          <span>正在准备下载...</span>
        </div>

        <div v-else-if="error" class="space-y-5">
          <div class="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-center">
            <XCircle class="mx-auto mb-3 h-12 w-12 text-destructive" />
            <p class="font-medium text-foreground">下载失败</p>
            <p class="mt-1 text-sm text-muted-foreground">{{ error }}</p>
          </div>
          <Button class="w-full" @click="goBack">返回上一页</Button>
        </div>

        <div v-else class="space-y-5">
          <!-- 真实 <a> 用户手势触发下载：Chrome/Android 会拦截非手势的自动下载 -->
          <a
            :href="fileInfo.download_url"
            :download="fileInfo.file_name || undefined"
            class="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-3 text-sm font-medium text-primary-foreground shadow-sm transition-opacity hover:opacity-90"
          >
            <Download class="h-5 w-5" />
            开始下载
          </a>
          <p class="text-center text-sm text-muted-foreground">
            验证已完成。Android 浏览器会拦截自动下载，<span class="font-medium text-foreground">请点击上方按钮开始下载</span>。
          </p>

          <div class="grid gap-2 sm:grid-cols-2">
            <Button variant="outline" @click="goBack">
              <ArrowLeft class="mr-2 h-4 w-4" />
              返回上一页
            </Button>
            <Button v-if="fileInfo.return_url" @click="goToWebsite">
              <Home class="mr-2 h-4 w-4" />
              前往网站
            </Button>
            <Button v-else @click="router.push('/')">
              <Home class="mr-2 h-4 w-4" />
              返回首页
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- 赞助请求：收款码在关于页 -->
    <Card class="w-full max-w-lg">
      <CardContent class="flex items-center justify-between gap-3 p-5">
        <div class="min-w-0">
          <p class="text-sm font-semibold text-foreground">喜欢本站？请考虑赞助支持</p>
          <p class="mt-0.5 text-xs leading-relaxed text-muted-foreground">
            您的支持会用于服务器、带宽与镜像存储等基础设施支出，收款码在关于页
          </p>
        </div>
        <RouterLink
          to="/about"
          class="inline-flex shrink-0 items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
        >
          <Heart class="h-4 w-4" />
          查看收款码
        </RouterLink>
      </CardContent>
    </Card>

    <!-- 官方用户群 -->
    <a
      :href="globalConfig.links.qqGroup"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-2 rounded-lg border px-5 py-2.5 text-sm font-medium text-foreground hover:bg-muted/60"
    >
      <Users class="h-4 w-4 text-primary" />
      进入官方用户群
      <span class="text-xs font-normal text-muted-foreground">柠泽资源站用户群</span>
    </a>
  </div>
</template>
