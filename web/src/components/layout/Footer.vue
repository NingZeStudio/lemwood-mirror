<script setup>
import { Github, Heart, MessageCircle } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
</script>

<template>
  <footer class="border-t bg-background">
    <div class="container flex flex-col items-center justify-between gap-4 py-5 md:h-16 md:flex-row md:py-0">
      <p class="text-center text-sm text-muted-foreground md:text-left">
        Built with
        <Heart class="inline-block h-4 w-4 text-destructive" />
        by
        <router-link to="/about" class="font-medium text-foreground underline-offset-4 hover:underline">
          Lemwood & MengZe2
        </router-link>
      </p>

      <div class="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          as="a"
          href="https://qm.qq.com/q/FOGt99aayY"
          target="_blank"
          rel="noopener noreferrer"
        >
          <MessageCircle class="h-4 w-4" />
          <span class="sr-only">QQ 群</span>
        </Button>
        <Button
          variant="ghost"
          size="icon"
          as="a"
          href="https://github.com/JanePHPDev/lemwood_mirror_NewWeb"
          target="_blank"
          rel="noopener noreferrer"
        >
          <Github class="h-4 w-4" />
          <span class="sr-only">GitHub</span>
        </Button>
        <a
          href="https://beian.miit.gov.cn/"
          target="_blank"
          rel="noopener noreferrer"
          class="rounded-md px-2 py-2 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          新ICP备2024015133号-6
        </a>
      </div>
    </div>
  </footer>
</template>
