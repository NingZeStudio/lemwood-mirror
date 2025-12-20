<template>
  <v-alert
    color="info"
    icon="mdi-bullhorn"
    variant="tonal"
    class="mb-6"
    title="公告"
  >
    <div class="d-flex flex-column gap-2 mt-2">
      <div v-for="(link, index) in links" :key="index">
        {{ link.label }}：
        <a :href="link.url" target="_blank" class="text-decoration-none font-weight-bold text-primary">
          {{ link.text || link.url }}
        </a>
      </div>
    </div>
  </v-alert>
</template>

<script setup>
const links = [
  { label: '赞助网站', url: 'https://zs.lemwood.cn/' },
  { label: 'mt管理器下载', url: 'https://mt2.cn', text: 'mt2.cn' },
  { label: 'shizuku官网地址', url: 'https://shizuku.rikka.app/zh-hans/' },
];
</script>
