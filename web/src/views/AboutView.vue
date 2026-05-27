<script setup>
import { computed, onMounted } from 'vue'
import { Code, ExternalLink, Github, Heart, Layers, Mail, MessageCircle, Server, Zap } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import FriendLinks from '@/components/FriendLinks.vue'
import { globalConfig } from '@/lib/globalConfig'
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
  document.title = `关于 - ${globalConfig.site.nameFull}`
  const desc = `了解${globalConfig.site.name}背后的团队、技术栈和项目故事`
  const metaDescription = document.querySelector('meta[name="description"]')
  const metaOgDescription = document.querySelector('meta[property="og:description"]')
  const metaTwitterDescription = document.querySelector('meta[property="twitter:description"]')
  if (metaDescription) metaDescription.setAttribute('content', desc)
  if (metaOgDescription) metaOgDescription.setAttribute('content', '关于 - ' + desc)
  if (metaTwitterDescription) metaTwitterDescription.setAttribute('content', '关于 - ' + desc)
})
</script>

<template>
  <div class="space-y-6">
    <div class="space-y-1">
      <h1 class="text-3xl font-bold tracking-tight">关于本项目</h1>
      <p class="text-sm text-muted-foreground">探索 Lemwood Mirror 的幕后故事、技术底座与那些支持我们的朋友。</p>
    </div>

    <div class="space-y-4">
      <div class="rounded-lg border bg-card p-5 shadow-sm">
        <div class="flex items-center gap-2 text-base font-semibold">
          <Layers class="h-5 w-5 text-primary" />
          项目简介
        </div>
        <p class="mt-3 text-sm leading-relaxed text-muted-foreground">
          Lemwood Mirror 是一个自托管的开源镜像服务，专为 Minecraft Java版 社区打造。它致力于提供稳定、高速的启动器及相关工具下载体验。
          通过全自动化的 GitHub Release 追踪系统，我们确保用户能第一时间获取到最新的软件版本，摆脱网络环境的束缚。
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div class="rounded-lg border bg-card p-5 shadow-sm">
          <div class="flex items-center gap-2 text-base font-semibold">
            <Server class="h-5 w-5 text-blue-500" />
            基础设施 & 后端
          </div>
          <div class="mt-3 space-y-3">
            <div>
              <h4 class="text-sm font-medium text-foreground">服务器 & 域名备案</h4>
              <p class="mt-0.5 text-xs text-muted-foreground">由 <span class="font-medium text-foreground">柠枺</span> 提供全方位支持。</p>
              <div class="mt-1 flex flex-col gap-1 text-xs text-muted-foreground">
                <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> {{ globalConfig.contact.email }}</span>
                <span>{{ globalConfig.contact.qq }}</span>
                <a :href="globalConfig.links.qqGroup" target="_blank" class="flex items-center gap-1 transition-colors hover:text-foreground">
                  <MessageCircle class="h-3 w-3" /> QQ群：{{ globalConfig.contact.qqGroup }}
                </a>
              </div>
            </div>
            <div class="border-t border-dashed pt-3">
              <h4 class="text-sm font-medium text-foreground">技术栈</h4>
              <div class="mt-1 flex flex-wrap gap-1.5">
                <span class="rounded-md bg-blue-500/10 px-2 py-0.5 text-xs font-medium text-blue-500">Golang</span>
                <span class="rounded-md bg-cyan-500/10 px-2 py-0.5 text-xs font-medium text-cyan-500">Docker</span>
              </div>
            </div>
          </div>
        </div>

        <div class="rounded-lg border bg-card p-5 shadow-sm">
          <div class="flex items-center gap-2 text-base font-semibold">
            <Code class="h-5 w-5 text-green-500" />
            前端开发 & 设计
          </div>
          <div class="mt-3 space-y-3">
            <div>
              <h4 class="text-sm font-medium text-foreground">核心开发</h4>
              <p class="mt-0.5 text-xs text-muted-foreground">由 <span class="font-medium text-foreground">琪初QiTry</span> 设计与编码。</p>
              <div class="mt-1 flex flex-col gap-1 text-xs text-muted-foreground">
                <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> qitryt@sina.cn</span>
                <span>Github：qitry</span>
              </div>
            </div>
            <div class="border-t border-dashed pt-3">
              <h4 class="text-sm font-medium text-foreground">技术栈</h4>
              <div class="mt-1 flex flex-wrap gap-1.5">
                <span class="rounded-md bg-green-500/10 px-2 py-0.5 text-xs font-medium text-green-500">Vue 3</span>
                <span class="rounded-md bg-purple-500/10 px-2 py-0.5 text-xs font-medium text-purple-500">Vite</span>
                <span class="rounded-md bg-slate-500/10 px-2 py-0.5 text-xs font-medium text-slate-500">Shadcn/Vue</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="space-y-4">
      <div class="rounded-lg border bg-card p-5 shadow-sm">
        <div class="flex items-center justify-between gap-4">
          <div class="space-y-2">
            <div class="inline-flex items-center gap-1.5 rounded-full border border-amber-500/20 bg-amber-500/10 px-3 py-1 text-xs font-medium text-amber-700 dark:text-amber-300">
              <Heart class="h-3.5 w-3.5" />
              赞助支持
            </div>
            <h2 class="text-xl font-bold tracking-tight">{{ sponsorConfig.title }}</h2>
            <p class="max-w-lg text-sm leading-relaxed text-muted-foreground">{{ sponsorConfig.description }}</p>
          </div>
          <div class="shrink-0 rounded-xl border bg-background px-5 py-3 text-center shadow-sm">
            <p class="text-xs text-muted-foreground">累计赞助</p>
            <p class="text-2xl font-bold text-amber-600 dark:text-amber-400">¥{{ totalAmount }}</p>
            <p class="text-xs text-muted-foreground">{{ sponsorCount }} 位朋友</p>
          </div>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div class="rounded-lg border bg-card p-5 shadow-sm">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <span class="rounded-md bg-blue-500/10 p-1.5">
              <Zap class="h-4 w-4 text-blue-500" />
            </span>
            支付宝
          </div>
          <div class="mt-3 flex min-h-[200px] items-center justify-center overflow-hidden rounded-lg border bg-muted/40 p-4">
            <img :src="sponsorConfig.alipayQrCode" alt="支付宝赞助二维码" class="max-h-48 w-auto rounded object-contain shadow-sm" />
          </div>
          <p class="mt-2 text-center text-xs text-muted-foreground">扫码赞助</p>
        </div>

        <div class="rounded-lg border bg-card p-5 shadow-sm">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <span class="rounded-md bg-green-500/10 p-1.5">
              <Zap class="h-4 w-4 text-green-500" />
            </span>
            微信
          </div>
          <div class="mt-3 flex min-h-[200px] items-center justify-center overflow-hidden rounded-lg border bg-muted/40 p-4">
            <img :src="sponsorConfig.wechatQrCode" alt="微信赞助二维码" class="max-h-48 w-auto rounded object-contain shadow-sm" />
          </div>
          <p class="mt-2 text-center text-xs text-muted-foreground">扫码赞助</p>
        </div>

        <div v-if="sponsorConfig.afdianLink" class="sm:col-span-2">
          <a :href="sponsorConfig.afdianLink" target="_blank" rel="noopener noreferrer"
            class="afdian-rainbow-ring relative flex overflow-hidden rounded-lg p-[2px] font-semibold text-white transition-transform hover:scale-[1.005]">
            <span class="flex w-full items-center justify-center gap-2 rounded-[7px] bg-black py-3 text-sm transition-colors hover:bg-zinc-900">
              <Zap class="h-4 w-4" />
              爱发电赞助
            </span>
          </a>
          <p class="mt-1.5 text-center text-xs text-muted-foreground">支持月付和一次性赞助</p>
        </div>
      </div>

      <div class="rounded-lg border bg-card shadow-sm">
        <div class="flex items-center justify-between border-b px-5 py-3">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <span class="rounded-md bg-amber-500/10 p-1.5">
              <Heart class="h-4 w-4 text-amber-500" />
            </span>
            赞助者列表
          </div>
          <div class="text-xs text-muted-foreground">
            <span class="font-medium text-foreground">{{ sponsorCount }}</span> 位 ·
            <span class="font-medium text-foreground">¥{{ totalAmount }}</span>
          </div>
        </div>

        <div v-if="sortedSponsors.length" class="divide-y">
          <div v-for="sponsor in sortedSponsors" :key="sponsor.id"
            class="flex items-center gap-3 px-5 py-3 transition-colors hover:bg-muted/30">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border bg-gradient-to-br from-primary/25 to-primary/10">
              <span class="text-sm font-bold text-primary">{{ sponsor.name.charAt(0).toUpperCase() }}</span>
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="text-sm font-medium text-foreground">{{ sponsor.name }}</span>
                <span :class="['rounded-full px-2 py-0.5 text-[10px] font-medium', getPlatformColor(sponsor.platform)]">
                  {{ getPlatformIcon(sponsor.platform) }}
                </span>
                <span v-if="sponsor.pinned" class="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] font-medium text-amber-500">
                  置顶
                </span>
              </div>
              <p class="truncate text-xs text-muted-foreground">{{ sponsor.message || sponsor.date }}</p>
            </div>
            <div class="shrink-0 text-right">
              <span class="text-base font-bold text-red-500">¥{{ sponsor.amount }}</span>
              <p class="text-[10px] text-muted-foreground">{{ sponsor.date }}</p>
            </div>
          </div>
        </div>

        <div v-else class="py-10 text-center">
          <Heart class="mx-auto mb-3 h-8 w-8 text-muted-foreground opacity-40" />
          <p class="text-sm text-muted-foreground">暂无赞助者，成为第一位！</p>
        </div>
      </div>

      <div class="rounded-lg border border-amber-500/20 bg-amber-500/5 px-5 py-3 text-center text-xs text-foreground">
        <p class="font-medium">项目捐助全部用于服务器运营，我们绝无私吞捐助的情况出现。</p>
      </div>
    </div>

    <FriendLinks />

    <div class="grid gap-4 sm:grid-cols-2">
      <div class="rounded-lg border bg-card p-5 shadow-sm transition-colors hover:border-primary/30">
        <h3 class="flex items-center gap-2 text-sm font-semibold">同根项目：LogShare.CN</h3>
          <p class="mt-2 text-sm leading-relaxed text-muted-foreground">
           一个更好用的 Minecraft 日志分析平台。类似于 mclo.gs，但提供更丰富的功能和更友好的用户体验。
         </p>
         <Button variant="outline" size="sm" class="mt-3" as="a" :href="globalConfig.links.logshare" target="_blank">
          立即体验 <ExternalLink class="ml-1.5 h-3 w-3" />
        </Button>
      </div>

      <div class="flex flex-col items-center justify-center rounded-lg border bg-card p-5 text-center shadow-sm">
        <Github class="mb-3 h-8 w-8 text-foreground/70" />
        <h3 class="text-sm font-semibold">开源贡献</h3>
        <p class="mt-1 text-xs text-muted-foreground">本项目代码完全开源。欢迎 Star、Fork 或提交 Pull Request。</p>
        <Button size="sm" class="mt-3" as="a" :href="globalConfig.links.githubOrg" target="_blank">
          前往 GitHub 仓库
        </Button>
      </div>
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
  from { background-position: 0% 50%; }
  to { background-position: 300% 50%; }
}

@media (prefers-reduced-motion: reduce) {
  .afdian-rainbow-ring { animation: none; }
}
</style>
