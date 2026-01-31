<script setup>
import { Github, Heart } from 'lucide-vue-next'
</script>

<template>
  <footer class="border-t bg-background/50 backdrop-blur-sm py-6 md:py-0">
    <div class="container flex flex-col items-center justify-between gap-4 md:h-16 md:flex-row">
      <div class="flex flex-col items-center gap-4 md:flex-row md:gap-2">
        <p class="text-center text-sm leading-loose text-muted-foreground md:text-left">
          Built with <Heart class="inline-block h-4 w-4 text-red-500 animate-pulse" /> by 
          <a href="/#/about" class="font-medium underline underline-offset-4 hover:text-foreground">
            Lemwood & MengZe2
          </a>
          .
        </p>
      </div>
      <div class="flex gap-4">
          <a href="https://github.com/JanePHPDev/lemwood_mirror_NewWeb" target="_blank" class="text-muted-foreground hover:text-foreground">
              <Github class="h-5 w-5" />
          </a>
          <a href="https://beian.miit.gov.cn/" target="_blank" class="text-xs text-muted-foreground hover:text-foreground flex items-center">
              新ICP备2024015133号-5
          </a>
      </div>
    </div>
  </footer>
</template>
