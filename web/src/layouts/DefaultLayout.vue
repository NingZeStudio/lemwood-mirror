<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useDark, useToggle } from '@vueuse/core'
import { Menu, Moon, Palette, Sun, Upload, X } from 'lucide-vue-next'
import Sidebar from '@/components/layout/Sidebar.vue'
import Footer from '@/components/layout/Footer.vue'
import Button from '@/components/ui/Button.vue'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { isNavigationActive, navigationLinks } from '@/lib/navigation'

const route = useRoute()
const isDark = useDark()
const toggleDark = useToggle(isDark)
const isMobileMenuOpen = ref(false)
const isThemePanelOpen = ref(false)

const colorOptions = [
  { name: '水墨', value: 'monochrome', color: 'bg-zinc-500' },
  { name: '海洋', value: 'blue', color: 'bg-blue-500' },
  { name: '薰衣草', value: 'purple', color: 'bg-purple-500' },
  { name: '森林', value: 'green', color: 'bg-green-500' },
  { name: '日落', value: 'orange', color: 'bg-orange-500' },
  { name: '樱花', value: 'pink', color: 'bg-pink-500' }
]

const selectedColor = ref(localStorage.getItem('theme-color') || 'monochrome')
const backdropBlur = ref(parseFloat(localStorage.getItem('backdrop-blur') || '0'))
const backdropOpacity = ref(parseFloat(localStorage.getItem('backdrop-opacity') || '1'))
const backgroundImage = ref(localStorage.getItem('background-image') || '')
const showBackgroundImage = ref(localStorage.getItem('show-background-image') === 'true')

const applyBackgroundSettings = () => {
  localStorage.setItem('backdrop-blur', String(backdropBlur.value))
  localStorage.setItem('backdrop-opacity', String(backdropOpacity.value))
  localStorage.setItem('background-image', backgroundImage.value)
  localStorage.setItem('show-background-image', String(showBackgroundImage.value))
  document.documentElement.style.setProperty('--backdrop-blur', `${backdropBlur.value}px`)
  document.documentElement.style.setProperty('--backdrop-opacity', String(backdropOpacity.value))

  if (showBackgroundImage.value && backgroundImage.value) {
    document.documentElement.style.setProperty('--background-image', `url(${backgroundImage.value})`)
  } else {
    document.documentElement.style.removeProperty('--background-image')
  }
}

const handleImageUpload = (event) => {
  const file = event.target.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = (e) => {
    backgroundImage.value = e.target?.result || ''
    showBackgroundImage.value = Boolean(backgroundImage.value)
    applyBackgroundSettings()
  }
  reader.readAsDataURL(file)
}

const applyThemeColor = (color) => {
  selectedColor.value = color
  localStorage.setItem('theme-color', color)
  document.documentElement.setAttribute('data-theme-color', color)
}

const resetThemeSettings = () => {
  localStorage.removeItem('theme-color')
  localStorage.removeItem('backdrop-blur')
  localStorage.removeItem('backdrop-opacity')
  localStorage.removeItem('background-image')
  localStorage.removeItem('show-background-image')
  document.documentElement.removeAttribute('data-theme-color')
  document.documentElement.style.removeProperty('--backdrop-blur')
  document.documentElement.style.removeProperty('--backdrop-opacity')
  document.documentElement.style.removeProperty('--background-image')
  selectedColor.value = 'monochrome'
  backdropBlur.value = 0
  backdropOpacity.value = 1
  backgroundImage.value = ''
  showBackgroundImage.value = false
  applyBackgroundSettings()
}

const hideBackgroundImage = () => {
  showBackgroundImage.value = false
  applyBackgroundSettings()
}

const clearBackgroundImage = () => {
  backgroundImage.value = ''
  showBackgroundImage.value = false
  applyBackgroundSettings()
}

applyThemeColor(selectedColor.value)
applyBackgroundSettings()
</script>

<template>
  <div
    class="min-h-screen bg-background"
    :style="{
      backgroundImage: showBackgroundImage && backgroundImage ? `url(${backgroundImage})` : 'none',
      backgroundSize: 'cover',
      backgroundPosition: 'center',
      backgroundAttachment: 'fixed'
    }"
  >
    <div
      class="grid min-h-screen w-full md:grid-cols-[220px_1fr] lg:grid-cols-[280px_1fr]"
      :style="{ backgroundColor: `hsl(var(--background) / ${backdropOpacity})` }"
    >
      <Sidebar />

      <div class="flex min-h-screen flex-col">
        <header class="sticky top-0 z-30 flex h-14 items-center gap-3 border-b bg-background px-4 lg:h-[60px] lg:px-6">
          <Sheet v-model:open="isMobileMenuOpen">
            <SheetTrigger as-child>
              <Button variant="outline" size="icon" class="shrink-0 md:hidden">
                <Menu class="h-5 w-5" />
                <span class="sr-only">打开导航菜单</span>
              </Button>
            </SheetTrigger>
            <SheetContent side="left" class="flex flex-col">
              <SheetHeader class="text-left">
                <SheetTitle class="flex items-center gap-2">
                  <img src="/favicon.jpg" alt="Logo" class="h-8 w-8 rounded-md border object-cover" />
                  柠枺镜像
                </SheetTitle>
                <SheetDescription>Lemwood Mirror 站点导航</SheetDescription>
              </SheetHeader>

              <nav class="mt-6 grid gap-1 text-sm font-medium">
                <router-link
                  v-for="link in navigationLinks"
                  :key="link.path"
                  :to="link.path"
                  :class="cn(
                    'flex items-center gap-3 rounded-md px-3 py-2 transition-colors hover:bg-accent hover:text-accent-foreground',
                    isNavigationActive(route.path, link)
                      ? 'bg-accent text-accent-foreground'
                      : 'text-muted-foreground'
                  )"
                  @click="isMobileMenuOpen = false"
                >
                  <component :is="link.icon" class="h-4 w-4" />
                  {{ link.name }}
                </router-link>
              </nav>

              <div class="mt-auto border-t pt-4 text-center text-xs text-muted-foreground">v3.15.0</div>
            </SheetContent>
          </Sheet>

          <div class="min-w-0 flex-1">
            <span class="font-semibold md:hidden">柠枺镜像</span>
          </div>

          <Button variant="ghost" size="icon" @click="toggleDark()">
            <Sun v-if="!isDark" class="h-5 w-5" />
            <Moon v-else class="h-5 w-5" />
            <span class="sr-only">切换深浅色</span>
          </Button>
          <Button variant="ghost" size="icon" @click="isThemePanelOpen = true">
            <Palette class="h-5 w-5" />
            <span class="sr-only">主题设置</span>
          </Button>
        </header>

        <main class="flex flex-1 flex-col gap-6 p-4 lg:p-6">
          <slot />
        </main>
        <Footer />
      </div>
    </div>
  </div>

  <Sheet v-model:open="isThemePanelOpen">
    <SheetContent side="right" class="w-[320px] overflow-y-auto sm:w-[420px]">
      <SheetHeader class="text-left">
        <SheetTitle class="flex items-center gap-2">
          <Palette class="h-5 w-5" />
          主题设置
        </SheetTitle>
        <SheetDescription>调整主题色、深浅色模式和可选背景图片。</SheetDescription>
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
            <Button variant="outline" :class="!isDark ? 'bg-accent' : ''" @click="!isDark || toggleDark()">
              <Sun class="mr-2 h-4 w-4" />
              浅色
            </Button>
            <Button variant="outline" :class="isDark ? 'bg-accent' : ''" @click="isDark || toggleDark()">
              <Moon class="mr-2 h-4 w-4" />
              深色
            </Button>
          </div>
        </section>

        <section class="space-y-4">
          <h4 class="text-sm font-medium">背景图片</h4>
          <div class="grid gap-2">
            <label class="inline-flex h-10 cursor-pointer items-center justify-center rounded-md border bg-background px-4 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground">
              <Upload class="mr-2 h-4 w-4" />
              上传图片
              <input type="file" accept="image/*" class="hidden" @change="handleImageUpload" />
            </label>
            <div class="grid grid-cols-2 gap-2" v-if="backgroundImage">
              <Button variant="outline" @click="hideBackgroundImage">
                <X class="mr-2 h-4 w-4" />
                隐藏
              </Button>
              <Button variant="outline" @click="clearBackgroundImage">
                <X class="mr-2 h-4 w-4" />
                清除
              </Button>
            </div>
          </div>
          <div v-if="backgroundImage" class="overflow-hidden rounded-md border">
            <img :src="backgroundImage" alt="背景预览" class="h-28 w-full object-cover" />
          </div>
        </section>

        <section class="space-y-4">
          <h4 class="text-sm font-medium">背景效果</h4>
          <div class="space-y-2">
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>模糊强度</span>
              <span>{{ backdropBlur }}px</span>
            </div>
            <input
              v-model.number="backdropBlur"
              type="range"
              min="0"
              max="20"
              step="1"
              class="w-full accent-primary"
              @input="applyBackgroundSettings"
            />
          </div>
          <div class="space-y-2">
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>背景遮罩</span>
              <span>{{ Math.round(backdropOpacity * 100) }}%</span>
            </div>
            <input
              v-model.number="backdropOpacity"
              type="range"
              min="0.1"
              max="1"
              step="0.1"
              class="w-full accent-primary"
              @input="applyBackgroundSettings"
            />
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
