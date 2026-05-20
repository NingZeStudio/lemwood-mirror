<script setup>
import { useRoute } from 'vue-router'
import { Github } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { cn } from '@/lib/utils'
import { isNavigationActive, navigationLinks } from '@/lib/navigation'

const route = useRoute()
</script>

<template>
  <aside class="hidden min-h-screen border-r bg-sidebar text-sidebar-foreground md:flex md:w-64 lg:w-72">
    <div class="flex w-full flex-col">
      <div class="flex h-14 items-center gap-3 border-b border-sidebar-border px-4 lg:h-[60px] lg:px-6">
        <router-link to="/" class="flex min-w-0 items-center gap-2 font-semibold">
          <img src="/favicon.jpg" alt="Logo" class="h-8 w-8 rounded-md border object-cover" />
          <div class="min-w-0">
            <div class="truncate leading-none">柠枺镜像</div>
            <div class="mt-1 text-xs font-normal text-muted-foreground">Lemwood Mirror</div>
          </div>
        </router-link>
        <Button
          variant="ghost"
          size="icon"
          class="ml-auto h-8 w-8"
          as="a"
          href="https://github.com/leemwood/lemwood_mirror/"
          target="_blank"
          rel="noopener noreferrer"
        >
          <Github class="h-4 w-4" />
          <span class="sr-only">GitHub</span>
        </Button>
      </div>

      <nav class="grid gap-1 p-3 text-sm font-medium lg:p-4">
        <router-link
          v-for="link in navigationLinks"
          :key="link.path"
          :to="link.path"
          :class="cn(
            'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
            isNavigationActive(route.path, link)
              ? 'bg-sidebar-accent text-sidebar-accent-foreground'
              : 'text-muted-foreground'
          )"
        >
          <component :is="link.icon" class="h-4 w-4" />
          {{ link.name }}
        </router-link>
      </nav>

      <div class="mt-auto border-t border-sidebar-border p-4 text-center text-xs text-muted-foreground">
        v3.15.0
      </div>
    </div>
  </aside>
</template>
