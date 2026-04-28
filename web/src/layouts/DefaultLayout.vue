<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useDark, useToggle } from '@vueuse/core'
import { Menu, Sun, Moon, Home, Folder, BarChart2, FileText, Info, Settings, Palette, Image, Upload, X } from 'lucide-vue-next'
import Sidebar from '@/components/layout/Sidebar.vue'
import Footer from '@/components/layout/Footer.vue'
import Button from '@/components/ui/Button.vue'
import { Sheet, SheetTrigger, SheetContent } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'

const isDark = useDark()
const toggleDark = useToggle(isDark)
const isMobileMenuOpen = ref(false)
const isThemePanelOpen = ref(false)
const route = useRoute()

// 主题色选项
const colorOptions = [
  { name: '水墨', value: 'monochrome', color: 'bg-zinc-500' },
  { name: '海洋', value: 'blue', color: 'bg-blue-500' },
  { name: '薰衣草', value: 'purple', color: 'bg-purple-500' },
  { name: '森林', value: 'green', color: 'bg-green-500' },
  { name: '日落', value: 'orange', color: 'bg-orange-500' },
  { name: '樱花', value: 'pink', color: 'bg-pink-500' },
]

// 当前选中的主题色
const selectedColor = ref(localStorage.getItem('theme-color') || 'monochrome')

// 背景设置
const backdropBlur = ref(parseFloat(localStorage.getItem('backdrop-blur') || '12'))
const backdropOpacity = ref(parseFloat(localStorage.getItem('backdrop-opacity') || '0.6'))
const backgroundImage = ref(localStorage.getItem('background-image') || '')
const showBackgroundImage = ref(localStorage.getItem('show-background-image') === 'true')

// 应用背景设置
const applyBackgroundSettings = () => {
  localStorage.setItem('backdrop-blur', backdropBlur.value.toString())
  localStorage.setItem('backdrop-opacity', backdropOpacity.value.toString())
  localStorage.setItem('background-image', backgroundImage.value)
  localStorage.setItem('show-background-image', showBackgroundImage.value.toString())
  document.documentElement.style.setProperty('--backdrop-blur', `${backdropBlur.value}px`)
  document.documentElement.style.setProperty('--backdrop-opacity', backdropOpacity.value.toString())
  if (showBackgroundImage.value && backgroundImage.value) {
    document.documentElement.style.setProperty('--background-image', `url(${backgroundImage.value})`)
  } else {
    document.documentElement.style.removeProperty('--background-image')
  }
}

// 处理图片上传
const handleImageUpload = (event) => {
  const file = event.target.files?.[0]
  if (file) {
    const reader = new FileReader()
    reader.onload = (e) => {
      backgroundImage.value = e.target?.result
      showBackgroundImage.value = true
      applyBackgroundSettings()
    }
    reader.readAsDataURL(file)
  }
}

// 应用主题色
const applyThemeColor = (color) => {
  selectedColor.value = color
  localStorage.setItem('theme-color', color)
  document.documentElement.setAttribute('data-theme-color', color)
}

// 初始化主题设置
if (localStorage.getItem('theme-color')) {
  applyThemeColor(localStorage.getItem('theme-color'))
}
if (localStorage.getItem('backdrop-blur')) {
  backdropBlur.value = parseFloat(localStorage.getItem('backdrop-blur'))
}
if (localStorage.getItem('backdrop-opacity')) {
  backdropOpacity.value = parseFloat(localStorage.getItem('backdrop-opacity'))
}
if (localStorage.getItem('background-image')) {
  backgroundImage.value = localStorage.getItem('background-image')
}
if (localStorage.getItem('show-background-image') === 'true') {
  showBackgroundImage.value = true
}
applyBackgroundSettings()

const links = [
  { name: '首页', path: '/', icon: Home },
  { name: '文件浏览', path: '/files', icon: Folder },
  { name: '数据统计', path: '/stats', icon: BarChart2 },
  { name: 'API 文档', path: '/api', icon: FileText },
  { name: '关于', path: '/about', icon: Info },
]
</script>

<template>
  <div class="grid min-h-screen w-full md:grid-cols-[220px_1fr] lg:grid-cols-[280px_1fr]"
    :style="{
      backgroundImage: showBackgroundImage && backgroundImage ? `url(${backgroundImage})` : 'none',
      backgroundSize: 'cover',
      backgroundPosition: 'center',
      backgroundAttachment: 'fixed',
      backdropFilter: `blur(${backdropBlur}px)`
    }">
    <div class="fixed inset-0 -z-10"
      :style="{
        backgroundColor: `hsl(var(--background) / ${backdropOpacity})`
      }">
    </div>
    <Sidebar />
    <div class="flex flex-col min-h-screen">
      <header class="flex h-14 items-center gap-4 border-b px-4 lg:h-[60px] lg:px-6 sticky top-0 z-30 bg-background/60 backdrop-blur-md">
        <Sheet v-model:open="isMobileMenuOpen">
          <SheetTrigger as-child>
            <Button variant="outline" size="icon" class="shrink-0 md:hidden">
              <Menu class="h-5 w-5" />
              <span class="sr-only">Toggle navigation menu</span>
            </Button>
          </SheetTrigger>
          <SheetContent side="left" class="flex flex-col">
            <div class="flex items-center gap-2 font-semibold mb-6">
               <img src="/favicon.jpg" alt="Logo" class="h-6 w-6 rounded-full object-cover">
               <span>柠枺镜像</span>
            </div>
            <nav class="grid gap-2 text-lg font-medium">
              <router-link
                v-for="link in links"
                :key="link.path"
                :to="link.path"
                :class="cn(
                  'flex items-center gap-4 rounded-xl px-3 py-2 hover:text-foreground',
                  route.path === link.path ? 'bg-muted text-foreground' : 'text-muted-foreground'
                )"
                @click="isMobileMenuOpen = false"
              >
                <component :is="link.icon" class="h-5 w-5" />
                {{ link.name }}
              </router-link>
            </nav>
            <div class="mt-auto">
               <div class="text-sm text-muted-foreground text-center">
                   v3.15.0
               </div>
            </div>
          </SheetContent>
        </Sheet>
        <div class="w-full flex-1">
          <!-- Breadcrumb or Title could go here -->
          <span class="font-semibold md:hidden">柠枺镜像</span>
        </div>
        <Button variant="ghost" size="icon" @click="toggleDark()">
          <Sun v-if="!isDark" class="h-5 w-5" />
          <Moon v-else class="h-5 w-5" />
          <span class="sr-only">Toggle theme</span>
        </Button>
        <Button variant="ghost" size="icon" @click="isThemePanelOpen = true">
          <Palette class="h-5 w-5" />
          <span class="sr-only">Theme settings</span>
        </Button>
      </header>
      <main class="flex flex-1 flex-col gap-4 p-4 lg:gap-6 lg:p-6 overflow-auto">
        <slot />
      </main>
      <Footer />
    </div>
  </div>
  
  <!-- 主题设置面板 -->
  <Sheet v-model:open="isThemePanelOpen" side="right">
    <SheetContent side="right" class="w-[300px] sm:w-[400px]">
      <div class="flex items-center gap-2 font-semibold mb-6">
        <Palette class="h-5 w-5" />
        <span>主题设置</span>
      </div>
      
      <div class="space-y-6">
        <!-- 主题色选择 -->
        <div class="space-y-3">
          <h4 class="text-sm font-medium text-muted-foreground">主题色</h4>
          <div class="grid grid-cols-3 gap-3">
            <button
              v-for="option in colorOptions"
              :key="option.value"
              @click="applyThemeColor(option.value)"
              class="relative flex flex-col items-center gap-2 rounded-lg border p-3 hover:bg-muted/50 transition-colors"
              :class="selectedColor === option.value ? 'border-primary bg-muted' : 'border-border'"
            >
              <div 
                class="h-8 w-8 rounded-full shadow-sm" 
                :class="[option.color, selectedColor === option.value ? 'ring-2 ring-primary ring-offset-2' : '']"
              />
              <span class="text-xs font-medium">{{ option.name }}</span>
            </button>
          </div>
        </div>
        
        <!-- 深色模式切换 -->
        <div class="space-y-3">
          <h4 class="text-sm font-medium text-muted-foreground">显示模式</h4>
          <div class="flex gap-2">
            <Button
              variant="outline"
              class="flex-1"
              :class="!isDark ? 'bg-primary text-primary-foreground' : ''"
              @click="!isDark || toggleDark()"
            >
              <Sun class="h-4 w-4 mr-2" />
              浅色
            </Button>
            <Button
              variant="outline"
              class="flex-1"
              :class="isDark ? 'bg-primary text-primary-foreground' : ''"
              @click="isDark || toggleDark()"
            >
              <Moon class="h-4 w-4 mr-2" />
              深色
            </Button>
          </div>
        </div>
        
        <!-- 背景设置 -->
        <div class="space-y-4">
          <h4 class="text-sm font-medium text-muted-foreground">背景效果</h4>
          
          <!-- 背景图片 -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs text-muted-foreground">背景图片</span>
              <Button
                v-if="showBackgroundImage && backgroundImage"
                variant="ghost"
                size="sm"
                @click="() => {
                  showBackgroundImage = false
                  applyBackgroundSettings()
                }"
                class="h-6 text-xs"
              >
                <X class="h-3 w-3 mr-1" />
                隐藏
              </Button>
            </div>
            
            <div class="flex gap-2">
              <label class="flex-1">
                <input
                  type="file"
                  accept="image/*"
                  @change="handleImageUpload"
                  class="hidden"
                />
                <Button variant="outline" class="w-full" as="div">
                  <Upload class="h-4 w-4 mr-2" />
                  上传图片
                </Button>
              </label>
              <Button
                v-if="backgroundImage"
                variant="outline"
                size="icon"
                @click="() => {
                  backgroundImage = ''
                  showBackgroundImage = false
                  applyBackgroundSettings()
                }"
              >
                <X class="h-4 w-4" />
              </Button>
            </div>
            
            <!-- 图片预览 -->
            <div v-if="backgroundImage" class="relative rounded-lg overflow-hidden border">
              <img
                :src="backgroundImage"
                alt="Background preview"
                class="w-full h-24 object-cover"
              />
              <div
                v-if="showBackgroundImage"
                class="absolute top-2 right-2 bg-green-500 text-white text-xs px-2 py-1 rounded-full"
              >
                已启用
              </div>
            </div>
          </div>
          
          <div class="space-y-2">
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>模糊强度</span>
              <span>{{ backdropBlur }}px</span>
            </div>
            <input
              type="range"
              min="0"
              max="20"
              step="1"
              v-model.number="backdropBlur"
              @input="applyBackgroundSettings"
              class="w-full h-2 bg-muted rounded-lg appearance-none cursor-pointer"
            />
          </div>
          
          <div class="space-y-2">
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>背景透明度</span>
              <span>{{ Math.round(backdropOpacity * 100) }}%</span>
            </div>
            <input
              type="range"
              min="0.1"
              max="1"
              step="0.1"
              v-model.number="backdropOpacity"
              @input="applyBackgroundSettings"
              class="w-full h-2 bg-muted rounded-lg appearance-none cursor-pointer"
            />
          </div>
        </div>
        
        <!-- 重置主题 -->
        <div class="pt-4 border-t">
          <Button
            variant="outline"
            class="w-full"
            @click="() => {
              localStorage.removeItem('theme-color')
              localStorage.removeItem('backdrop-blur')
              localStorage.removeItem('backdrop-opacity')
              localStorage.removeItem('background-image')
              localStorage.removeItem('show-background-image')
              document.documentElement.removeAttribute('data-theme-color')
              document.documentElement.style.removeProperty('--backdrop-blur')
              document.documentElement.style.removeProperty('--backdrop-opacity')
              document.documentElement.style.removeProperty('--background-image')
              selectedColor = 'monochrome'
              backdropBlur = 12
              backdropOpacity = 0.6
              backgroundImage = ''
              showBackgroundImage = false
              applyBackgroundSettings()
            }"
          >
            重置主题设置
          </Button>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
