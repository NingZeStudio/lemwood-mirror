<script setup>
import { computed, onMounted } from 'vue'
import {
  Code,
  DollarSign,
  ExternalLink,
  Github,
  Heart,
  Layers,
  Mail,
  MessageCircle,
  Pin,
  QrCode,
  Server,
  Zap
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
import FriendLinks from '@/components/FriendLinks.vue'
import {
  sponsors,
  sponsorConfig,
  getTotalAmount,
  getSponsorCount,
  getPlatformIcon,
  getPlatformColor
} from '@/lib/sponsorConfig'

const totalAmount = computed(() => getTotalAmount())
const sponsorCount = computed(() => getSponsorCount())

const sortedSponsors = computed(() => {
  return [...sponsors].sort((a, b) => {
    if (a.pinned && !b.pinned) return -1
    if (!a.pinned && b.pinned) return 1
    return new Date(b.date).getTime() - new Date(a.date).getTime()
  })
})

onMounted(() => {
  document.title = '关于 - 柠枺镜像状态'
  updateMetaDescription('了解柠枺镜像站背后的团队、技术栈和项目故事')
})

const updateMetaDescription = (desc) => {
  const metaDescription = document.querySelector('meta[name="description"]')
  const metaOgDescription = document.querySelector('meta[property="og:description"]')
  const metaTwitterDescription = document.querySelector('meta[property="twitter:description"]')

  if (metaDescription) metaDescription.setAttribute('content', desc)
  if (metaOgDescription) metaOgDescription.setAttribute('content', '关于 - ' + desc)
  if (metaTwitterDescription) metaTwitterDescription.setAttribute('content', '关于 - ' + desc)
}
</script>

<template>
  <div class="container max-w-4xl px-4 py-6 md:py-10 space-y-6 md:space-y-8">
    <div class="space-y-2 text-center md:text-left">
      <h1 class="text-3xl md:text-4xl font-extrabold tracking-tight">关于本项目</h1>
      <p class="text-muted-foreground text-base md:text-lg">
        探索 Lemwood Mirror 的幕后故事、技术底座与那些支持我们的朋友。
      </p>
    </div>

    <section class="space-y-5">
      <Card class="overflow-hidden border-amber-500/20 bg-gradient-to-br from-amber-500/10 via-background to-primary/5 ">
        <CardContent class="relative p-5 md:p-6">
          <div class="absolute -right-10 -top-10 h-32 w-32 rounded-full bg-amber-500/10 blur-2xl"></div>
          <div class="relative flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div class="space-y-2">
              <div class="inline-flex items-center gap-2 rounded-full border border-amber-500/20 bg-amber-500/10 px-3 py-1 text-sm font-medium text-amber-700 dark:text-amber-300">
                <Heart class="h-4 w-4" />
                Sponsor
              </div>
              <div>
                <h2 class="text-2xl font-bold tracking-tight md:text-3xl">
                  {{ sponsorConfig.title }}
                </h2>
                <p class="mt-2 max-w-2xl text-sm leading-relaxed text-muted-foreground md:text-base">
                  {{ sponsorConfig.description }}
                </p>
              </div>
            </div>
            <div class="rounded-2xl border border-amber-500/20 bg-background/70 p-4 text-center shadow-sm backdrop-blur">
              <p class="text-xs text-muted-foreground">累计赞助</p>
              <p class="text-3xl font-bold text-amber-600 dark:text-amber-400">¥{{ totalAmount }}</p>
              <p class="text-xs text-muted-foreground">{{ sponsorCount }} 位朋友支持</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Card class="bg-card">
          <CardContent class="flex items-center gap-3 p-4">
            <div class="rounded-md bg-red-500/10 p-2">
              <Heart class="h-4 w-4 text-red-500" />
            </div>
            <div>
              <p class="text-xs text-muted-foreground">赞助者</p>
              <p class="text-xl font-bold">{{ sponsorCount }}</p>
            </div>
          </CardContent>
        </Card>
        <Card class="bg-card">
          <CardContent class="flex items-center gap-3 p-4">
            <div class="rounded-md bg-green-500/10 p-2">
              <DollarSign class="h-4 w-4 text-green-500" />
            </div>
            <div>
              <p class="text-xs text-muted-foreground">总金额</p>
              <p class="text-xl font-bold">¥{{ totalAmount }}</p>
            </div>
          </CardContent>
        </Card>
        <Card class="bg-card">
          <CardContent class="flex items-center gap-3 p-4">
            <div class="rounded-md bg-blue-500/10 p-2">
              <QrCode class="h-4 w-4 text-blue-500" />
            </div>
            <div>
              <p class="text-xs text-muted-foreground">赞助方式</p>
              <p class="text-xl font-bold">2</p>
            </div>
          </CardContent>
        </Card>
        <Card class="bg-card">
          <CardContent class="flex items-center gap-3 p-4">
            <div class="rounded-md bg-purple-500/10 p-2">
              <Heart class="h-4 w-4 text-purple-500" />
            </div>
            <div>
              <p class="text-xs text-muted-foreground">感谢</p>
              <p class="text-xl font-bold">❤️</p>
            </div>
          </CardContent>
        </Card>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <Card class="group bg-card transition-all hover:border-primary/40 ">
          <CardHeader class="p-4 md:p-5">
            <CardTitle class="flex items-center gap-2 text-base">
              <span class="rounded-md bg-blue-500/10 p-2">
                <QrCode class="h-4 w-4 text-blue-500" />
              </span>
              支付宝
            </CardTitle>
          </CardHeader>
          <CardContent class="px-4 pb-4 md:px-5 md:pb-5">
            <div class="flex min-h-[260px] items-center justify-center overflow-hidden rounded-xl border bg-muted/40 p-4">
              <img
                :src="sponsorConfig.alipayQrCode"
                alt="支付宝赞助二维码"
                class="max-h-64 w-auto rounded-lg object-contain shadow-sm"
              />
            </div>
            <p class="mt-3 text-center text-sm text-muted-foreground">扫码赞助</p>
          </CardContent>
        </Card>

        <Card class="group bg-card transition-all hover:border-primary/40 ">
          <CardHeader class="p-4 md:p-5">
            <CardTitle class="flex items-center gap-2 text-base">
              <span class="rounded-md bg-green-500/10 p-2">
                <QrCode class="h-4 w-4 text-green-500" />
              </span>
              微信
            </CardTitle>
          </CardHeader>
          <CardContent class="px-4 pb-4 md:px-5 md:pb-5">
            <div class="flex min-h-[260px] items-center justify-center overflow-hidden rounded-xl border bg-muted/40 p-4">
              <img
                :src="sponsorConfig.wechatQrCode"
                alt="微信赞助二维码"
                class="max-h-64 w-auto rounded-lg object-contain shadow-sm"
              />
            </div>
            <p class="mt-3 text-center text-sm text-muted-foreground">扫码赞助</p>
          </CardContent>
        </Card>

        <Card
          v-if="sponsorConfig.afdianLink"
          class="bg-card md:col-span-2"
        >
          <CardContent class="p-4 md:p-5">
            <a
              :href="sponsorConfig.afdianLink"
              target="_blank"
              rel="noopener noreferrer"
              class="afdian-rainbow-ring relative flex w-full overflow-hidden rounded-xl p-[2px] font-semibold text-white transition-transform hover:scale-[1.01]"
            >
              <span class="flex w-full items-center justify-center gap-3 rounded-[calc(var(--radius)+6px)] bg-black py-3 transition-colors hover:bg-gray-900">
                <Zap class="h-5 w-5" />
                <span>爱发电</span>
              </span>
            </a>
            <p class="mt-3 text-center text-sm text-muted-foreground">支持月付和一次性赞助</p>
          </CardContent>
        </Card>
      </div>

      <Card class="overflow-hidden bg-card">
        <CardHeader class="border-b p-4 md:p-5">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle class="flex items-center gap-2 text-base">
              <span class="rounded-md bg-amber-500/10 p-2">
                <Heart class="h-4 w-4 text-amber-500" />
              </span>
              赞助者列表
            </CardTitle>
            <div class="text-sm text-muted-foreground">
              <span class="font-medium text-foreground">{{ sponsorCount }}</span> 位赞助者
              <span class="mx-2">·</span>
              <span class="font-medium text-foreground">¥{{ totalAmount }}</span>
            </div>
          </div>
        </CardHeader>

        <CardContent class="p-0">
          <div v-if="sortedSponsors.length" class="divide-y">
            <div
              v-for="sponsor in sortedSponsors"
              :key="sponsor.id"
              class="flex items-center gap-3 p-4 transition-colors hover:bg-muted/50 md:gap-4"
            >
              <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border bg-gradient-to-br from-primary/25 to-primary/10">
                <span class="text-sm font-bold text-primary">
                  {{ sponsor.name.charAt(0).toUpperCase() }}
                </span>
              </div>

              <div class="min-w-0 flex-1">
                <div class="mb-1 flex flex-wrap items-center gap-2">
                  <span class="font-medium text-foreground">{{ sponsor.name }}</span>
                  <span
                    :class="[
                      'rounded-full px-2 py-0.5 text-xs font-medium',
                      getPlatformColor(sponsor.platform)
                    ]"
                  >
                    {{ getPlatformIcon(sponsor.platform) }}
                  </span>
                  <span
                    v-if="sponsor.pinned"
                    class="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-500"
                  >
                    <Pin class="h-3 w-3" />
                    置顶
                  </span>
                </div>
                <p class="truncate text-sm text-muted-foreground">
                  {{ sponsor.message || sponsor.date }}
                </p>
              </div>

              <div class="shrink-0 text-right">
                <span class="text-lg font-bold text-red-500">¥{{ sponsor.amount }}</span>
                <p class="mt-0.5 text-xs text-muted-foreground">{{ sponsor.date }}</p>
              </div>
            </div>
          </div>

          <div v-else class="py-8 text-center">
            <Heart class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-50" />
            <p class="text-sm text-muted-foreground">暂无赞助者，成为第一位！</p>
          </div>
        </CardContent>
      </Card>

      <div class="rounded-xl border border-amber-500/20 bg-amber-500/5 p-4 text-center text-sm text-foreground">
        <p class="font-medium">项目捐助全部用于服务器运营，我们绝无私吞捐助的情况出现。</p>
      </div>

      <div class="rounded-xl border bg-muted/30 p-4 text-center text-sm text-muted-foreground">
        <div class="mb-1 flex items-center justify-center gap-2">
          <Heart class="h-4 w-4 text-red-500" />
          <span class="font-medium">感谢您的支持！</span>
        </div>
        <p>所有赞助将用于项目运营和发展</p>
      </div>
    </section>

    <div class="grid gap-4 md:gap-6 md:grid-cols-2">
        <Card class="bg-card border-border md:col-span-2">
          <CardHeader class="p-4 md:p-6">
            <CardTitle class="flex items-center gap-2">
                <Layers class="h-5 w-5 text-primary" />
                项目简介
            </CardTitle>
          </CardHeader>
          <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4 text-muted-foreground leading-relaxed">
            <p>
              Lemwood Mirror 是一个自托管的开源镜像服务，专为 Minecraft Java版 社区打造。它致力于提供稳定、高速的启动器及相关工具下载体验。
              通过全自动化的 GitHub Release 追踪系统，我们确保用户能第一时间获取到最新的软件版本，摆脱网络环境的束缚。
            </p>
          </CardContent>
        </Card>

        <Card class="bg-card border-border">
            <CardHeader class="p-4 md:p-6">
                <CardTitle class="flex items-center gap-2">
                    <Server class="h-5 w-5 text-blue-500" />
                    基础设施 & 后端
                </CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4">
                 <div class="flex items-start gap-4">
                     <div class="space-y-1">
                         <h4 class="font-semibold text-foreground">服务器 & 域名备案</h4>
                         <p class="text-sm text-muted-foreground">由 <span class="text-foreground font-medium">柠枺</span> 提供全方位支持。</p>
                         <div class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 text-xs text-muted-foreground mt-1">
                             <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> 3436464181@qq.com</span>
                             <span class="hidden sm:inline border-l h-3"></span>
                             <span>QQ: 3436464181</span>
                         </div>
                         <div class="flex items-center gap-1 text-xs text-muted-foreground mt-1">
                             <a href="https://qm.qq.com/q/FOGt99aayY" target="_blank" class="flex items-center gap-1 hover:text-foreground transition-colors">
                                 <MessageCircle class="h-3 w-3" /> QQ群：1077373741
                             </a>
                         </div>
                     </div>
                 </div>
                 <div class="space-y-1 pt-2 border-t border-dashed border-border">
                     <h4 class="font-semibold text-foreground">技术栈</h4>
                     <div class="flex flex-wrap gap-2 text-xs">
                         <span class="px-2 py-1 rounded-md bg-blue-500/10 text-blue-500 font-medium">Golang</span>
                         <span class="px-2 py-1 rounded-md bg-cyan-500/10 text-cyan-500 font-medium">Docker</span>
                     </div>
                 </div>
            </CardContent>
        </Card>

        <Card class="bg-card border-border">
            <CardHeader class="p-4 md:p-6">
                <CardTitle class="flex items-center gap-2">
                    <Code class="h-5 w-5 text-green-500" />
                    前端开发 & 设计
                </CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4">
                 <div class="flex items-start gap-4">
                     <div class="space-y-1">
                          <h4 class="font-semibold text-foreground">核心开发</h4>
                          <p class="text-sm text-muted-foreground">由 <span class="text-foreground font-medium">琪初QiTry</span> 设计与编码。</p>
                          <div class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 text-xs text-muted-foreground mt-1">
                              <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> qitryt@sina.cn</span>
                              <span class="hidden sm:inline border-l h-3"></span>
                              <span>Github：qitry</span>
                          </div>
                     </div>
                 </div>
                 <div class="space-y-1 pt-2 border-t border-dashed border-border">
                     <h4 class="font-semibold text-foreground">技术栈</h4>
                     <div class="flex flex-wrap gap-2 text-xs">
                         <span class="px-2 py-1 rounded-md bg-green-500/10 text-green-500 font-medium">Vue 3</span>
                         <span class="px-2 py-1 rounded-md bg-purple-500/10 text-purple-500 font-medium">Vite</span>
                         <span class="px-2 py-1 rounded-md bg-slate-500/10 text-slate-500 font-medium">Shadcn/Vue</span>
                     </div>
                 </div>
            </CardContent>
        </Card>
    </div>

    <FriendLinks />

    <div class="grid gap-4 md:gap-6 md:grid-cols-2">
         <Card class="bg-gradient-to-br from-background/50 to-primary/5  border-primary/20 hover:border-primary/40 transition-colors">
            <CardHeader class="p-4 md:p-6">
                <CardTitle>同根项目：LogShare.CN</CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-3">
                <p class="text-sm text-muted-foreground">
                    一个更好用的 Minecraft 日志分析平台。类似于 mclo.gs，但提供更丰富的功能和更友好的用户体验。
                </p>
                <div class="pt-2 text-center md:text-left">
                    <Button variant="outline" size="sm" as="a" href="https://logshare.cn/" target="_blank">
                        立即体验 <ExternalLink class="ml-2 h-3 w-3" />
                    </Button>
                </div>
            </CardContent>
        </Card>

        <Card class="bg-card border-border flex flex-col justify-center items-center text-center p-4 md:p-6">
             <Github class="h-8 md:h-10 w-8 md:w-10 mb-2 md:mb-4 text-foreground/80" />
             <h3 class="font-semibold mb-1 md:mb-2 text-sm md:text-base">开源贡献</h3>
             <p class="text-xs md:text-sm text-muted-foreground mb-3 md:mb-4">
                 本项目代码完全开源。欢迎 Star、Fork 或提交 Pull Request。
             </p>
             <Button size="sm" as="a" href="https://github.com/NingZeStudio/lemwood-mirror" target="_blank">
                 前往 GitHub 仓库
             </Button>
        </Card>
    </div>
  </div>
</template>

<style scoped>
.afdian-rainbow-ring {
  background: linear-gradient(90deg, #ec4899, #a855f7, #06b6d4, #22c55e, #f59e0b, #ec4899);
  background-size: 300% 100%;
  animation: afdian-rainbow-flow 4s linear infinite;
}

@keyframes afdian-rainbow-flow {
  from {
    background-position: 0% 50%;
  }

  to {
    background-position: 300% 50%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .afdian-rainbow-ring {
    animation: none;
  }
}
</style>
