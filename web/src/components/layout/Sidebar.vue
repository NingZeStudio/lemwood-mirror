<script setup>
import { useRoute } from 'vue-router'
import { cn } from '@/lib/utils'
import { Home, Folder, BarChart2, FileText, Github, Info } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'

const route = useRoute()

const links = [
  { name: '首页', path: '/', icon: Home },
  { name: '文件浏览', path: '/files', icon: Folder },
  { name: '数据统计', path: '/stats', icon: BarChart2 },
  { name: 'API 文档', path: '/api', icon: FileText },
  { name: '关于', path: '/about', icon: Info },
]
</script>

<template>
  <div class="hidden border-r bg-muted/40 md:block md:w-64 lg:w-72 h-full flex flex-col">
    <div class="flex h-14 items-center border-b px-4 lg:h-[60px] lg:px-6">
      <a href="/" class="flex items-center gap-2 font-semibold">
        <img src="/favicon.jpg" alt="Logo" class="h-6 w-6 rounded-full object-cover">
        <span>柠枺镜像</span>
      </a>
      <Button variant="outline" size="icon" class="ml-auto h-8 w-8">
        <Github class="h-4 w-4" />
        <a href="https://github.com/leemwood/lemwood_mirror/" class="sr-only">GitHub</a>
      </Button>
    </div>
    <div class="flex-1">
      <nav class="grid items-start px-2 text-sm font-medium lg:px-4 mt-4">
        <router-link
          v-for="link in links"
          :key="link.path"
          :to="link.path"
          :class="cn(
            'flex items-center gap-3 rounded-lg px-3 py-2 transition-all hover:text-primary',
            route.path === link.path ? 'bg-muted text-primary' : 'text-muted-foreground'
          )"
        >
          <component :is="link.icon" class="h-4 w-4" />
          {{ link.name }}
        </router-link>
      </nav>
    </div>
    <div class="mt-auto p-4">
        <div class="text-xs text-muted-foreground text-center">
            v3.15.0
        </div>
    </div>
  </div>
</template>
