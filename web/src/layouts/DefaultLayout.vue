<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { useDark, useToggle } from '@vueuse/core'
import { Menu, Sun, Moon, Home, Folder, BarChart2, FileText } from 'lucide-vue-next'
import Sidebar from '@/components/layout/Sidebar.vue'
import Button from '@/components/ui/Button.vue'
import { Sheet, SheetTrigger, SheetContent } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'

const isDark = useDark()
const toggleDark = useToggle(isDark)
const isMobileMenuOpen = ref(false)
const route = useRoute()

const links = [
  { name: '首页', path: '/', icon: Home },
  { name: '文件浏览', path: '/files', icon: Folder },
  { name: '数据统计', path: '/stats', icon: BarChart2 },
  { name: 'API 文档', path: '/api', icon: FileText },
]
</script>

<template>
  <div class="grid min-h-screen w-full md:grid-cols-[220px_1fr] lg:grid-cols-[280px_1fr]">
    <Sidebar />
    <div class="flex flex-col">
      <header class="flex h-14 items-center gap-4 border-b bg-muted/40 px-4 lg:h-[60px] lg:px-6">
        <Sheet v-model:open="isMobileMenuOpen">
          <SheetTrigger as-child>
            <Button variant="outline" size="icon" class="shrink-0 md:hidden">
              <Menu class="h-5 w-5" />
              <span class="sr-only">Toggle navigation menu</span>
            </Button>
          </SheetTrigger>
          <SheetContent side="left" class="flex flex-col">
            <div class="flex items-center gap-2 font-semibold mb-6">
               <img src="https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/b4ee27d31312bdb9.svg" alt="Logo" class="h-6 w-6">
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
                   v1.0.0
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
      </header>
      <main class="flex flex-1 flex-col gap-4 p-4 lg:gap-6 lg:p-6 overflow-auto">
        <slot />
      </main>
    </div>
  </div>
</template>