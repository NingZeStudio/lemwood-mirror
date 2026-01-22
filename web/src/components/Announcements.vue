<script setup>
import { Megaphone, ExternalLink } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'

const links = [
  { label: '赞助网站', url: 'https://zs.lemwood.cn/', color: 'text-pink-500' },
  { label: 'MT管理器下载', url: 'https://mt2.cn', text: 'mt2.cn', color: 'text-green-500' },
  { label: 'Shizuku 官网', url: 'https://shizuku.rikka.app/zh-hans/', color: 'text-blue-500' },
];
</script>

<template>
  <Card class="mb-6">
    <CardHeader class="pb-3">
      <CardTitle class="flex items-center gap-2 text-lg">
        <Megaphone class="h-5 w-5 text-primary" />
        站点公告
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div class="grid gap-4 sm:grid-cols-3">
        <a
          v-for="(link, index) in links"
          :key="index"
          :href="link.url"
          target="_blank"
          class="flex items-center gap-2 rounded-lg border p-3 hover:bg-muted/50 transition-colors"
        >
          <div class="flex flex-col overflow-hidden">
            <span class="font-medium text-sm">{{ link.label }}</span>
            <span class="text-xs text-muted-foreground truncate">{{ link.text || link.url }}</span>
          </div>
        </a>
      </div>
    </CardContent>
  </Card>
</template>