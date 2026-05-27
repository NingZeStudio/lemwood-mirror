<script setup>
import { onMounted } from 'vue'
import VersionList from '@/components/VersionList.vue'
import { globalConfig } from '@/lib/globalConfig'

const mkTitle = (name) => `${name} - ${globalConfig.site.nameFull}`

onMounted(() => {
  const title = mkTitle('首页')
  document.title = title
  updateMetaInfo(globalConfig.site.description)
})

const updateMetaInfo = (desc) => {
  const title = mkTitle('首页')
  const titleEl = document.querySelector('title')
  if (titleEl) titleEl.textContent = title

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
  ogTitle.setAttribute('content', title)

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
  twitterTitle.setAttribute('content', title)

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
