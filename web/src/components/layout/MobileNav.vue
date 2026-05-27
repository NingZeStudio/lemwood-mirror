<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { Github, Menu, X } from 'lucide-vue-next'
import { cn } from '@/lib/utils'
import { isNavigationActive, navigationLinks } from '@/lib/navigation'
import { globalConfig } from '@/lib/globalConfig'

const route = useRoute()
const isOpen = ref(false)

const toggleNav = () => { isOpen.value = !isOpen.value }
const closeNav = () => { isOpen.value = false }
</script>

<template>
  <div class="relative md:hidden">
    <button
      class="rounded-md p-2 transition-colors hover:bg-accent"
      aria-label="菜单"
      @click="toggleNav"
    >
      <Menu v-if="!isOpen" class="h-5 w-5" />
      <X v-else class="h-5 w-5" />
    </button>

    <Transition
      enter-active-class="transition-all duration-200"
      enter-from-class="translate-y-2 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition-all duration-150"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-2 opacity-0"
    >
      <div
        v-if="isOpen"
        class="absolute right-0 top-full mt-2 w-56 overflow-hidden rounded-xl border bg-card shadow-lg"
      >
        <div class="space-y-0.5 p-1.5">
          <router-link
            v-for="link in navigationLinks"
            :key="link.path"
            :to="link.path"
            :class="cn(
              'flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors',
              isNavigationActive(route.path, link)
                ? 'bg-accent text-accent-foreground'
                : 'text-muted-foreground hover:bg-accent/50 hover:text-accent-foreground'
            )"
            @click="closeNav"
          >
            <component :is="link.icon" class="h-4 w-4" />
            {{ link.name }}
          </router-link>
          <div class="border-t my-1"></div>
          <a
            :href="globalConfig.links.githubRepo"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent/50 hover:text-accent-foreground"
            @click="closeNav"
          >
            <Github class="h-4 w-4" />
            GitHub
          </a>
        </div>
      </div>
    </Transition>
  </div>
</template>
