<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Moon, Palette, Sun, X } from 'lucide-vue-next'
import Footer from '@/components/layout/Footer.vue'
import MobileNav from '@/components/layout/MobileNav.vue'
import Button from '@/components/ui/Button.vue'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { isNavigationActive, navigationLinks } from '@/lib/navigation'
import { globalConfig } from '@/lib/globalConfig'

const route = useRoute()

const { themeColor, darkMode } = globalConfig.storage.keys

const isDark = computed({
  get: () => document.documentElement.classList.contains('dark'),
  set: (v) => document.documentElement.classList.toggle('dark', v)
})

const toggleDark = () => { isDark.value = !isDark.value }
const isThemePanelOpen = ref(false)

const colorOptions = globalConfig.theme.colors.map(c => ({
  name: c.name,
  value: c.value,
  color: ({
    monochrome: 'bg-zinc-500',
    blue: 'bg-blue-500',
    purple: 'bg-purple-500',
    green: 'bg-green-500',
    orange: 'bg-orange-500',
    pink: 'bg-pink-500'
  })[c.value] || 'bg-zinc-500'
}))

const selectedColor = ref(localStorage.getItem(themeColor) || globalConfig.theme.defaultColor)

const applyThemeColor = (color) => {
  selectedColor.value = color
  localStorage.setItem(themeColor, color)
  document.documentElement.setAttribute('data-theme-color', color)
}

const resetThemeSettings = () => {
  localStorage.removeItem(themeColor)
  document.documentElement.removeAttribute('data-theme-color')
  selectedColor.value = globalConfig.theme.defaultColor
}

onMounted(() => {
  const stored = localStorage.getItem(darkMode)
  if (stored === 'dark' || (!stored && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
})

applyThemeColor(selectedColor.value)
</script>

<template>
  <div class="min-h-screen bg-background">
    <div class="flex min-h-screen flex-col">
      <header class="sticky top-3 z-30 mx-auto w-[calc(100%-2rem)] max-w-6xl rounded-xl border bg-background/95 shadow-sm backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div class="flex h-14 items-center gap-2 px-3 lg:h-[60px] lg:gap-3 lg:px-5">
          <router-link to="/" class="flex shrink-0 items-center gap-2 font-semibold">
            <img src="/favicon.jpg" alt="Logo" class="h-7 w-7 rounded border object-cover" />
            <span class="inline">{{ globalConfig.site.name }}</span>
          </router-link>

          <nav class="ml-4 hidden items-center gap-0.5 md:flex">
            <router-link
              v-for="link in navigationLinks"
              :key="link.path"
              :to="link.path"
              :class="cn(
                'rounded-md px-2.5 py-1.5 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground',
                isNavigationActive(route.path, link)
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground'
              )"
            >
              {{ link.name }}
            </router-link>
          </nav>

          <div class="flex-1" />

          <Button variant="ghost" size="icon" class="h-8 w-8" @click="toggleDark()">
            <Sun v-if="!isDark" class="h-4 w-4" />
            <Moon v-else class="h-4 w-4" />
            <span class="sr-only">切换深浅色</span>
          </Button>
          <Button variant="ghost" size="icon" class="h-8 w-8" @click="isThemePanelOpen = true">
            <Palette class="h-4 w-4" />
            <span class="sr-only">主题设置</span>
          </Button>
          <MobileNav />
        </div>
      </header>

      <main class="mx-auto flex w-[calc(100%-2rem)] max-w-6xl flex-1 flex-col gap-6 pt-6 lg:pt-8">
        <slot />
      </main>
      <Footer />
    </div>
  </div>

  <Sheet v-model:open="isThemePanelOpen">
    <SheetContent side="right" class="w-[320px] overflow-y-auto sm:w-[420px]">
      <SheetHeader class="text-left">
        <SheetTitle class="flex items-center gap-2">
          <Palette class="h-5 w-5" />
          主题设置
        </SheetTitle>
        <SheetDescription>调整主题色和深浅色模式。</SheetDescription>
      </SheetHeader>

      <div class="mt-6 space-y-6">
        <section class="space-y-3">
          <h4 class="text-sm font-medium">主题色</h4>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="option in colorOptions"
              :key="option.value"
              type="button"
              :class="cn(
                'flex items-center gap-3 rounded-md border px-3 py-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground',
                selectedColor === option.value ? 'border-primary bg-accent text-accent-foreground' : 'bg-background'
              )"
              @click="applyThemeColor(option.value)"
            >
              <span class="h-4 w-4 rounded-full" :class="option.color"></span>
              {{ option.name }}
            </button>
          </div>
        </section>

        <section class="space-y-3">
          <h4 class="text-sm font-medium">显示模式</h4>
          <div class="grid grid-cols-2 gap-2">
            <Button variant="outline" :class="!isDark ? 'bg-accent' : ''" @click="isDark.value = false">
              <Sun class="mr-2 h-4 w-4" />
              浅色
            </Button>
            <Button variant="outline" :class="isDark ? 'bg-accent' : ''" @click="isDark.value = true">
              <Moon class="mr-2 h-4 w-4" />
              深色
            </Button>
          </div>
        </section>

        <div class="border-t pt-4">
          <Button variant="outline" class="w-full" @click="resetThemeSettings">
            重置主题设置
          </Button>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
