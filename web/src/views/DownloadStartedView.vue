<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getDownloadLanding } from '@/services/api'
import { globalConfig } from '@/lib/globalConfig'
import { Download, Home, ArrowLeft, Loader2, XCircle } from 'lucide-vue-next'
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

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else if (fileInfo.value?.return_url) {
    window.location.href = fileInfo.value.return_url
  } else {
    router.push('/')
  }
}

const goToWebsite = () => {
  if (fileInfo.value?.return_url) {
    window.location.href = fileInfo.value.return_url
  } else {
    router.push('/')
  }
}

onMounted(() => {
  document.title = `下载已开始 - ${globalConfig.site.nameFull}`
  loadLandingInfo()
})
</script>

<template>
  <div class="flex min-h-[calc(100vh-10rem)] items-center justify-center py-8">
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
          <p class="text-center text-sm text-muted-foreground">
            如果没有自动开始下载，您可以<a :href="fileInfo.download_url" class="text-primary hover:underline" target="_blank">点击这里手动下载</a>。
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
  </div>
</template>
