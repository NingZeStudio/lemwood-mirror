<script setup>
import { onMounted, ref } from 'vue'
import { ExternalLink, Megaphone, X } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardFooter from '@/components/ui/CardFooter.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Badge from '@/components/ui/Badge.vue'
import {
  announcementConfig,
  hasSeenAnnouncement,
  markAnnouncementAsSeen,
  resetAnnouncement
} from '@/lib/announcementConfig'

const isOpen = ref(false)

onMounted(() => {
  if (!announcementConfig.enabled || hasSeenAnnouncement()) return

  window.setTimeout(() => {
    isOpen.value = true
  }, 500)
})

const closeDialog = () => {
  isOpen.value = false
  markAnnouncementAsSeen()
}

const forceShowAnnouncement = () => {
  resetAnnouncement()
  isOpen.value = true
}

const isExternalLink = (url) => /^https?:\/\//.test(url)

defineExpose({
  forceShowAnnouncement
})
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition ease-out duration-200"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition ease-in duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="isOpen"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        aria-labelledby="announcement-title"
        @click.self="closeDialog"
        @keydown.esc.window="closeDialog"
      >
        <Transition
          appear
          enter-active-class="transition ease-out duration-200"
          enter-from-class="scale-95 translate-y-3 opacity-0"
          enter-to-class="scale-100 translate-y-0 opacity-100"
          leave-active-class="transition ease-in duration-150"
          leave-from-class="scale-100 translate-y-0 opacity-100"
          leave-to-class="scale-95 translate-y-3 opacity-0"
        >
          <Card class="relative w-full max-w-2xl overflow-hidden shadow-xl">
            <div class="absolute inset-x-0 top-0 h-1 bg-primary"></div>
            <button
              class="absolute right-4 top-4 rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
              aria-label="关闭公告"
              @click="closeDialog"
            >
              <X class="h-4 w-4" />
            </button>

            <CardHeader class="pb-4 pr-12">
              <Badge variant="secondary" class="w-fit">最新公告</Badge>
              <CardTitle id="announcement-title" class="flex items-center gap-3 text-2xl">
                <span class="rounded-md bg-primary/10 p-2 text-primary">
                  <Megaphone class="h-5 w-5" />
                </span>
                {{ announcementConfig.title }}
              </CardTitle>
              <CardDescription>当公告 ID 更新时，已读状态会自动失效并再次显示。</CardDescription>
            </CardHeader>

            <CardContent class="space-y-4">
              <div class="rounded-lg border bg-muted/40 p-4 text-sm leading-7 text-muted-foreground">
                <p class="whitespace-pre-line">{{ announcementConfig.content }}</p>
              </div>

              <div
                v-if="announcementConfig.importantText"
                class="rounded-lg border border-warning/20 bg-warning/10 p-4 text-sm text-foreground"
              >
                {{ announcementConfig.importantText }}
              </div>

              <div v-if="announcementConfig.links?.length" class="grid gap-2 sm:grid-cols-2">
                <a
                  v-for="link in announcementConfig.links"
                  :key="link.url"
                  :href="link.url"
                  :target="isExternalLink(link.url) ? '_blank' : null"
                  :rel="isExternalLink(link.url) ? 'noopener noreferrer' : null"
                  class="flex items-center justify-between gap-3 rounded-lg border bg-background px-4 py-3 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground"
                >
                  <span>{{ link.label }}</span>
                  <ExternalLink class="h-4 w-4 text-muted-foreground" />
                </a>
              </div>
            </CardContent>

            <CardFooter class="flex flex-col gap-3 border-t pt-5 sm:flex-row sm:items-center sm:justify-between">
              <p class="text-xs text-muted-foreground">关闭后不会重复显示，除非公告 ID 更新。</p>
              <Button class="w-full sm:w-auto" @click="closeDialog">我知道了</Button>
            </CardFooter>
          </Card>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
