<script setup>
import { onMounted, ref } from 'vue'
import { Cookie } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { globalConfig } from '@/lib/globalConfig'

const isOpen = ref(false)

const { cookiesConsented } = globalConfig.storage.keys

onMounted(() => {
  const consented = localStorage.getItem(cookiesConsented)
  if (!consented) {
    isOpen.value = true
  }
})

const accept = () => {
  localStorage.setItem(cookiesConsented, 'true')
  isOpen.value = false
}
</script>

<template>
  <div v-if="isOpen" class="fixed inset-x-0 bottom-0 z-50 p-4 animate-in slide-in-from-bottom-full duration-300">
    <div class="mx-auto flex max-w-4xl flex-col gap-4 rounded-lg border bg-background p-4 shadow-lg sm:flex-row sm:items-center sm:justify-between">
      <div class="flex items-start gap-3">
        <div class="rounded-md bg-primary/10 p-2 text-primary">
          <Cookie class="h-5 w-5" />
        </div>
        <div class="space-y-1 text-sm">
          <p class="font-medium text-foreground">Cookie 使用提示</p>
          <p class="text-muted-foreground">{{ globalConfig.legal.cookieConsent }}</p>
        </div>
      </div>
      <Button size="sm" class="shrink-0" @click="accept">我知道了</Button>
    </div>
  </div>
</template>
