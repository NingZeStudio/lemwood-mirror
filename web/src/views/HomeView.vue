<script setup>
import { onMounted } from 'vue'
import { ArrowRight, Download, Sparkles } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import VersionList from '@/components/VersionList.vue'

onMounted(() => {
  const title = '首页 - 柠枺镜像状态'
  document.title = title
  updateMetaInfo('提供多种 Minecraft 启动器版本的镜像下载服务，包括 Flower Launcher、HMCL 等')
})

const updateMetaInfo = (desc) => {
  const titleEl = document.querySelector('title')
  if (titleEl) titleEl.textContent = '首页 - 柠枺镜像状态'

  let metaDescription = document.querySelector('meta[name="description"]')
  if (!metaDescription) {
    metaDescription = document.createElement('meta')
    metaDescription.setAttribute('name', 'description')
    document.head.appendChild(metaDescription)
  }
  metaDescription.setAttribute('content', desc)

  let ogTitle = document.querySelector('meta[property="og:title"]')
  if (!ogTitle) {
    ogTitle = document.createElement('meta')
    ogTitle.setAttribute('property', 'og:title')
    document.head.appendChild(ogTitle)
  }
  ogTitle.setAttribute('content', '首页 - 柠枺镜像状态')

  let ogDescription = document.querySelector('meta[property="og:description"]')
  if (!ogDescription) {
    ogDescription = document.createElement('meta')
    ogDescription.setAttribute('property', 'og:description')
    document.head.appendChild(ogDescription)
  }
  ogDescription.setAttribute('content', desc)

  let twitterTitle = document.querySelector('meta[property="twitter:title"]')
  if (!twitterTitle) {
    twitterTitle = document.createElement('meta')
    twitterTitle.setAttribute('property', 'twitter:title')
    document.head.appendChild(twitterTitle)
  }
  twitterTitle.setAttribute('content', '首页 - 柠枺镜像状态')

  let twitterDescription = document.querySelector('meta[property="twitter:description"]')
  if (!twitterDescription) {
    twitterDescription = document.createElement('meta')
    twitterDescription.setAttribute('property', 'twitter:description')
    document.head.appendChild(twitterDescription)
  }
  twitterDescription.setAttribute('content', desc)
}
</script>

<template>
  <div class="space-y-6">
    <Card class="overflow-hidden">
      <CardHeader class="space-y-4 pb-4">
        <div class="inline-flex w-fit items-center gap-2 rounded-full border bg-accent px-3 py-1 text-sm text-accent-foreground">
          <Sparkles class="h-4 w-4" />
          简洁、快速、稳定的启动器镜像站
        </div>
        <div class="space-y-2">
          <CardTitle class="text-3xl md:text-4xl">Lemwood Mirror</CardTitle>
          <CardDescription class="max-w-2xl text-base leading-relaxed">
            浏览启动器最新版本、查看历史文件、获取统计信息。整体界面已统一为更接近 shadcn-vue 默认风格的简洁卡片体系。
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent class="flex flex-col gap-3 sm:flex-row">
        <Button as="a" href="/files">
          <Download class="mr-2 h-4 w-4" />
          立即浏览文件
        </Button>
        <Button as="a" href="/stats" variant="outline">
          查看统计信息
          <ArrowRight class="ml-2 h-4 w-4" />
        </Button>
      </CardContent>
    </Card>

    <VersionList />
  </div>
</template>
