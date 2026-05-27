<script setup>
import { onMounted } from 'vue'
import { FileText } from 'lucide-vue-next'
import { globalConfig } from '@/lib/globalConfig'

onMounted(() => {
  document.title = `API 文档 - ${globalConfig.site.nameFull}`
})
</script>

<template>
  <div class="flex flex-1 flex-col items-center justify-center gap-4 py-24 text-center">
    <div class="rounded-full bg-primary/10 p-4 text-primary">
      <FileText class="h-8 w-8" />
    </div>
    <h1 class="text-2xl font-bold tracking-tight">新版 API 文档正在编写中</h1>
    <p class="max-w-md text-sm text-muted-foreground">
      我们正在重新整理和完善 API 文档，请耐心等待。感谢您的理解与支持。
    </p>
  </div>
</template>
