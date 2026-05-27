<script setup>
import { Github, Heart, MessageCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { globalConfig } from '@/lib/globalConfig'
</script>

<template>
  <footer class="mt-8 border-t bg-background">
    <div class="mx-auto flex w-[calc(100%-2rem)] max-w-6xl flex-col items-center justify-between gap-4 py-6 md:flex-row md:py-4">
      <p class="text-center text-xs text-muted-foreground md:text-left">
        Built with
        <Heart class="inline-block h-3.5 w-3.5 text-destructive" />
        by
        <router-link to="/about" class="font-medium text-foreground underline-offset-4 hover:underline">
          {{ globalConfig.site.author }}
        </router-link>
      </p>

      <div class="flex items-center gap-2">
        <a
          :href="globalConfig.links.beian"
          target="_blank"
          rel="noopener noreferrer"
          class="text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          {{ globalConfig.legal.icp }}
        </a>
        <span class="text-muted-foreground/40">|</span>
        <div class="flex items-center gap-0.5">
          <Button variant="ghost" size="icon" class="h-7 w-7" as="a" :href="globalConfig.links.qqGroup" target="_blank" rel="noopener noreferrer">
            <MessageCircle class="h-3.5 w-3.5" />
            <span class="sr-only">QQ 群</span>
          </Button>
          <Button variant="ghost" size="icon" class="h-7 w-7" as="a" :href="globalConfig.links.githubRepoNewWeb" target="_blank" rel="noopener noreferrer">
            <Github class="h-3.5 w-3.5" />
            <span class="sr-only">GitHub</span>
          </Button>
        </div>
      </div>
    </div>
  </footer>
</template>
