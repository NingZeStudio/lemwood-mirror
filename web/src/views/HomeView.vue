<script setup>
import { onMounted } from 'vue'
import VersionList from '@/components/VersionList.vue'

onMounted(() => {
  const title = '首页 - 柠枺镜像状态'
  document.title = title
  updateMetaInfo('提供多种 Minecraft 启动器版本的镜像下载服务，包括 Flower Launcher、HMCL 等')
})

const updateMetaInfo = (desc) => {
  // 更新 title
  const titleEl = document.querySelector('title')
  if (titleEl) titleEl.textContent = '首页 - 柠枺镜像状态'
  
  // 更新 meta description
  let metaDescription = document.querySelector('meta[name="description"]')
  if (!metaDescription) {
    metaDescription = document.createElement('meta')
    metaDescription.setAttribute('name', 'description')
    document.head.appendChild(metaDescription)
  }
  metaDescription.setAttribute('content', desc)
  
  // 更新 og:title
  let ogTitle = document.querySelector('meta[property="og:title"]')
  if (!ogTitle) {
    ogTitle = document.createElement('meta')
    ogTitle.setAttribute('property', 'og:title')
    document.head.appendChild(ogTitle)
  }
  ogTitle.setAttribute('content', '首页 - 柠枺镜像状态 - Minecraft 启动器版本镜像站')
  
  // 更新 og:description
  let ogDescription = document.querySelector('meta[property="og:description"]')
  if (!ogDescription) {
    ogDescription = document.createElement('meta')
    ogDescription.setAttribute('property', 'og:description')
    document.head.appendChild(ogDescription)
  }
  ogDescription.setAttribute('content', desc)
  
  // 更新 twitter:title
  let twitterTitle = document.querySelector('meta[property="twitter:title"]')
  if (!twitterTitle) {
    twitterTitle = document.createElement('meta')
    twitterTitle.setAttribute('property', 'twitter:title')
    document.head.appendChild(twitterTitle)
  }
  twitterTitle.setAttribute('content', '首页 - 柠枺镜像状态')
  
  // 更新 twitter:description
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
    <VersionList />
  </div>
</template>
