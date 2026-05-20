<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CheckCircle, Copy, Download, Home, Loader2, ShieldCheck, XCircle } from 'lucide-vue-next'
import { getCaptchaConfig, verifyDownload } from '@/services/api'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardFooter from '@/components/ui/CardFooter.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'

const route = useRoute()
const router = useRouter()

const filePath = ref('')
const captchaId = ref('')
const isLoading = ref(true)
const isVerifying = ref(false)
const verifyStatus = ref('pending')
const errorMessage = ref('')
const downloadUrl = ref('')
const showCopiedTip = ref(false)
const downloadStarted = ref(false)

const fullDownloadUrl = computed(() => {
  if (!downloadUrl.value) return ''
  return 'https://miawa.cn' + downloadUrl.value
})

let captchaObj = null

const showCaptcha = () => {
  if (captchaObj) {
    isLoading.value = false
    verifyStatus.value = 'pending'
    captchaObj.showCaptcha()
  }
}

const verifyCaptcha = async (lotNumber, captchaOutput, passToken, genTime) => {
  isVerifying.value = true
  verifyStatus.value = 'pending'

  try {
    const returnUrl = route.query.return_url || window.location.href
    const source = route.query.source || 'verify-download'
    const response = await verifyDownload(lotNumber, captchaOutput, passToken, genTime, filePath.value, returnUrl, source)
    
    const landingUrl = response.data.landing_url
    if (landingUrl) {
      if (landingUrl.startsWith('http')) {
        window.location.href = landingUrl
      } else {
        router.push(landingUrl)
      }
      return
    }

    downloadUrl.value = response.data.download_url
    verifyStatus.value = 'success'
    isLoading.value = false
  } catch (error) {
    console.error('Verify download error:', error)
    errorMessage.value = error.response?.data?.message || '验证失败，请重试'
    verifyStatus.value = 'error'
    isLoading.value = false
  } finally {
    isVerifying.value = false
  }
}

const startDownload = () => {
  if (!downloadUrl.value) return
  window.location.href = downloadUrl.value
  downloadStarted.value = true
}

const goToHome = () => {
  router.push('/')
}

const copyUrl = async () => {
  if (!downloadUrl.value) return

  try {
    await navigator.clipboard.writeText(fullDownloadUrl.value)
    showCopiedTip.value = true
    setTimeout(() => {
      showCopiedTip.value = false
    }, 2000)
  } catch (err) {
    console.error('Copy failed:', err)
  }
}

const loadCaptchaScript = () => {
  return new Promise((resolve, reject) => {
    if (window.initGeetest4) {
      resolve()
      return
    }

    const existingScript = document.querySelector('script[src*="gt4.js"]')
    if (existingScript) {
      const checkLoaded = setInterval(() => {
        if (window.initGeetest4) {
          clearInterval(checkLoaded)
          resolve()
        }
      }, 100)
      return
    }

    const script = document.createElement('script')
    script.src = 'https://static.geetest.com/v4/gt4.js'
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('Failed to load Geetest script'))
    document.head.appendChild(script)
  })
}

const initCaptcha = () => {
  isLoading.value = true
  verifyStatus.value = 'pending'

  window.initGeetest4(
    {
      captchaId: captchaId.value,
      product: 'bind'
    },
    (captcha) => {
      captchaObj = captcha

      captcha.onReady(() => {
        isLoading.value = false
        captcha.showCaptcha()
      })

      captcha.onSuccess(() => {
        const result = captcha.getValidate()
        if (result && result.lot_number) {
          verifyCaptcha(result.lot_number, result.captcha_output, result.pass_token, result.gen_time)
        } else {
          isLoading.value = false
          verifyStatus.value = 'error'
          errorMessage.value = '验证结果获取失败，请重试'
        }
      })

      captcha.onError((e) => {
        isLoading.value = false
        verifyStatus.value = 'error'
        errorMessage.value = '验证加载失败: ' + (e.msg || '未知错误')
      })

      captcha.onClose(() => {
        isLoading.value = false
        verifyStatus.value = 'error'
        errorMessage.value = '用户取消验证'
      })
    }
  )
}

const init = async () => {
  isLoading.value = true
  filePath.value = route.query.file || ''

  if (!filePath.value) {
    errorMessage.value = '缺少文件参数'
    verifyStatus.value = 'error'
    isLoading.value = false
    return
  }

  try {
    const [response] = await Promise.all([getCaptchaConfig(), loadCaptchaScript()])
    captchaId.value = response.data.app_id

    if (!response.data.enabled) {
      window.location.href = `/download/${filePath.value}`
      return
    }

    initCaptcha()
  } catch (error) {
    console.error('Init error:', error)
    errorMessage.value = '加载配置失败: ' + error.message
    verifyStatus.value = 'error'
    isLoading.value = false
  }
}

onMounted(() => {
  init()
})

onUnmounted(() => {
  if (captchaObj) {
    captchaObj.destroy()
  }
})
</script>

<template>
  <div class="flex min-h-[calc(100vh-10rem)] items-center justify-center py-8">
    <div
      v-if="showCopiedTip"
      class="fixed left-1/2 top-20 z-50 -translate-x-1/2 rounded-md border bg-background px-4 py-2 text-sm shadow-md"
    >
      已复制到剪贴板
    </div>

    <Card class="w-full max-w-lg">
      <CardHeader class="items-center text-center">
        <div class="mb-2 rounded-full bg-primary/10 p-3 text-primary">
          <ShieldCheck class="h-8 w-8" />
        </div>
        <CardTitle class="text-2xl">安全验证</CardTitle>
        <CardDescription>请完成验证后开始下载</CardDescription>
      </CardHeader>

      <CardContent class="space-y-6">
        <div
          v-if="isLoading && verifyStatus !== 'error'"
          class="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed py-12 text-muted-foreground"
        >
          <Loader2 class="h-8 w-8 animate-spin" />
          <span>{{ isVerifying ? '正在验证...' : '正在加载验证...' }}</span>
        </div>

        <div v-else-if="verifyStatus === 'success'" class="space-y-5">
          <div class="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-4 text-center">
            <CheckCircle class="mx-auto mb-3 h-12 w-12 text-emerald-500" />
            <p class="font-medium text-foreground">验证成功</p>
            <p class="mt-1 text-sm text-muted-foreground">下载链接已生成，可以直接下载或复制。</p>
          </div>

          <div class="rounded-md border bg-muted/40 p-3 text-xs text-muted-foreground break-all">
            {{ fullDownloadUrl }}
          </div>

          <div class="grid gap-2 sm:grid-cols-2">
            <Button v-if="!downloadStarted" @click="startDownload">
              <Download class="mr-2 h-4 w-4" />
              直接下载
            </Button>
            <Button v-else @click="goToHome">
              <Home class="mr-2 h-4 w-4" />
              返回首页
            </Button>
            <Button variant="outline" @click="copyUrl">
              <Copy class="mr-2 h-4 w-4" />
              复制链接
            </Button>
          </div>
        </div>

        <div v-else-if="verifyStatus === 'error'" class="space-y-5">
          <div class="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-center">
            <XCircle class="mx-auto mb-3 h-12 w-12 text-destructive" />
            <p class="font-medium text-foreground">验证失败</p>
            <p class="mt-1 text-sm text-muted-foreground">{{ errorMessage }}</p>
          </div>

          <Button class="w-full" @click="showCaptcha">重新验证</Button>
        </div>
      </CardContent>

      <CardFooter v-if="filePath" class="border-t text-xs text-muted-foreground">
        <span class="break-all">文件：{{ filePath.split('/').pop() }}</span>
      </CardFooter>
    </Card>
  </div>
</template>
