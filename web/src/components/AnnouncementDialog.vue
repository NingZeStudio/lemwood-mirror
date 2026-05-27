<script setup>
import { onMounted, ref } from 'vue'
import { X } from 'lucide-vue-next'
import {
  announcementConfig,
  hasSeenAnnouncement,
  markAnnouncementAsSeen,
  resetAnnouncement
} from '@/lib/announcementConfig'

const isOpen = ref(false)

onMounted(() => {
  if (!hasSeenAnnouncement()) {
    setTimeout(() => {
      isOpen.value = true
    }, 500)
  }
})

const closeDialog = () => {
  isOpen.value = false
  markAnnouncementAsSeen()
}

const forceShowAnnouncement = () => {
  resetAnnouncement()
  isOpen.value = true
}

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
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-3 backdrop-blur-sm sm:p-4"
        @click.self="closeDialog"
      >
        <Transition
          enter-active-class="transition ease-out duration-200"
          enter-from-class="opacity-0 scale-95"
          enter-to-class="opacity-100 scale-100"
          leave-active-class="transition ease-in duration-150"
          leave-from-class="opacity-100 scale-100"
          leave-to-class="opacity-0 scale-95"
        >
          <div class="relative w-full max-w-md overflow-hidden rounded-lg bg-card text-card-foreground shadow-2xl">
            <div class="p-5 sm:p-6">
              <div class="mb-4 flex items-start justify-between">
                <div class="flex items-center gap-2">
                  <div class="h-6 w-1 rounded-full bg-primary"></div>
                  <h2 class="text-lg font-semibold text-foreground">
                    {{ announcementConfig.title }}
                  </h2>
                </div>
                <button
                  class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label="关闭"
                  @click="closeDialog"
                >
                  <X class="h-4 w-4" />
                </button>
              </div>

              <div class="space-y-3 text-sm leading-relaxed text-muted-foreground">
                <div class="rounded-lg bg-muted/50 p-4">
                  <p class="whitespace-pre-line leading-relaxed">
                    {{ announcementConfig.content }}
                  </p>
                  <p
                    v-if="announcementConfig.importantText"
                    class="mt-3 font-bold text-red-500"
                  >
                    {{ announcementConfig.importantText }}
                  </p>
                </div>
              </div>

              <div
                v-if="announcementConfig.links?.length"
                class="mt-4 flex flex-wrap gap-2"
              >
                <a
                  v-for="(link, index) in announcementConfig.links"
                  :key="index"
                  :href="link.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 rounded-md bg-primary/10 px-3 py-1.5 text-sm font-medium text-primary transition-colors hover:bg-primary/20"
                >
                  {{ link.label }}
                </a>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
