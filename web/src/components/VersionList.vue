<script setup>
import { ref, computed, onMounted } from 'vue';
import { getStatus, getLatest } from '@/services/api';
import { Download, History, Copy, X, Loader2, Package, Check } from 'lucide-vue-next';
import {
  DialogRoot,
  DialogTrigger,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogClose,
} from 'radix-vue';
import Card from '@/components/ui/Card.vue';
import CardHeader from '@/components/ui/CardHeader.vue';
import CardTitle from '@/components/ui/CardTitle.vue';
import CardDescription from '@/components/ui/CardDescription.vue';
import CardContent from '@/components/ui/CardContent.vue';
import CardFooter from '@/components/ui/CardFooter.vue';
import Button from '@/components/ui/Button.vue';
import Badge from '@/components/ui/Badge.vue';
import Skeleton from '@/components/ui/Skeleton.vue';
import { cn } from '@/lib/utils';
import { useClipboard } from '@vueuse/core';

const LAUNCHER_INFO_MAP = {
  'zl': { displayName: 'ZalithLauncher', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/34c1ec9e07f826df.webp' },
  'zl2': { displayName: 'ZalithLauncher2', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/ee0028bd82493eb3.webp' },
  'hmcl': { displayName: 'HMCL', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/3835841e4b9b7abf.jpeg' },
  'MG': { displayName: 'MobileGlues', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/3625548d2639a024.png' },
  'fcl': { displayName: 'FoldCraftLauncher', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/dc5e0ee14d8f54f0.png' },
  'shizuku': { displayName: 'Shizuku', logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/f7067665f073b4cc.png' }
};

const rawLaunchers = ref({});
const latestMap = ref({});
const loading = ref(true);
const selectedLauncher = ref(null);
const isDialogOpen = ref(false);
const { copy } = useClipboard();
const copiedStates = ref({});

const launcherList = computed(() => {
  return Object.keys(rawLaunchers.value).map(name => {
    const versions = rawLaunchers.value[name];
    const latestVersion = latestMap.value[name];
    const latestObj = versions.find(v => (v.tag_name || v.name) === latestVersion) || versions[0];
    const info = LAUNCHER_INFO_MAP[name] || { displayName: name, logoUrl: 'https://cdn.mengze.vip/gh/JanePHPDev/Blog-Static-Resource@main/images/b4ee27d31312bdb9.svg' };
    
    const latestDownloadUrl = latestObj && latestObj.assets && latestObj.assets.length > 0
      ? getAssetUrl(name, latestObj, latestObj.assets[0])
      : '#';
    
    return {
      name,
      displayName: info.displayName,
      logoUrl: info.logoUrl,
      versions,
      latest: latestVersion,
      lastUpdated: versions.length ? versions[0].published_at : null,
      hasAssets: latestObj && latestObj.assets && latestObj.assets.length > 0,
      latestObj,
      latestDownloadUrl
    };
  });
});

const loadData = async () => {
  loading.value = true;
  try {
    const [statusRes, latestRes] = await Promise.all([getStatus(), getLatest()]);
    
    const data = statusRes.data;
    for (const key in data) {
        data[key].sort((a, b) => String(b.tag_name || b.name).localeCompare(String(a.tag_name || b.name)));
    }
    rawLaunchers.value = data;
    latestMap.value = latestRes.data;
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
};

const openHistory = (item) => {
  selectedLauncher.value = item;
  isDialogOpen.value = true;
};

const formatDate = (dateStr) => {
    if (!dateStr) return '未知时间';
    return new Date(dateStr).toLocaleDateString();
};

const getAssetUrl = (launcherName, version, asset) => {
     if (asset.url && (asset.url.startsWith('http://') || asset.url.startsWith('https://'))) {
        return asset.url;
    }
    return `/download/${launcherName}/${version.tag_name || version.name}/${asset.name}`;
};

const copyLink = (url, id) => {
    const fullUrl = url.startsWith('http') ? url : window.location.origin + url;
    copy(fullUrl);
    copiedStates.value[id] = true;
    setTimeout(() => {
        copiedStates.value[id] = false;
    }, 2000);
};

onMounted(() => {
    loadData();
});

defineExpose({ refresh: loadData });
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
       <div>
         <h2 class="text-3xl font-bold tracking-tight">版本探索</h2>
         <p class="text-muted-foreground mt-1">发现并下载最新的启动器组件</p>
       </div>
       <Button variant="ghost" size="icon" @click="loadData" :disabled="loading">
         <Loader2 v-if="loading" class="h-5 w-5 animate-spin" />
         <template v-else>
            <span class="sr-only">Refresh</span>
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-5 w-5"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
         </template>
       </Button>
    </div>
    
    <div v-if="loading && !launcherList.length" class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <Card v-for="i in 4" :key="i" class="flex flex-col overflow-hidden">
        <div class="h-40 bg-muted animate-pulse" />
        <CardHeader class="pb-2 text-center">
          <Skeleton class="h-6 w-3/4 mx-auto mb-2" />
          <Skeleton class="h-4 w-1/2 mx-auto" />
        </CardHeader>
        <CardContent class="flex-1" />
        <CardFooter class="flex flex-col gap-2 pt-0">
          <Skeleton class="h-10 w-full" />
          <div class="flex gap-2 w-full">
            <Skeleton class="h-10 flex-1" />
            <Skeleton class="h-10 flex-1" />
          </div>
        </CardFooter>
      </Card>
    </div>

    <div v-else-if="!launcherList.length" class="text-center text-muted-foreground p-12">
      <Package class="mx-auto h-12 w-12 mb-4 opacity-50" />
      <div>暂无数据</div>
    </div>

    <div v-else class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <Card 
        v-for="item in launcherList" 
        :key="item.name" 
        class="flex flex-col overflow-hidden hover:shadow-md transition-shadow group"
      >
        <div class="relative h-40 overflow-hidden bg-muted/20">
            <img 
            :src="item.logoUrl" 
            class="absolute inset-0 h-full w-full object-cover blur-xl opacity-50 scale-110 group-hover:scale-105 transition-transform duration-500"
            alt=""
          />
          <div class="absolute inset-0 flex items-center justify-center">
             <img 
                :src="item.logoUrl" 
                class="h-20 w-20 object-contain drop-shadow-lg rounded-xl"
                :alt="item.displayName"
              />
          </div>
          <Badge 
            v-if="item.latest" 
            class="absolute top-4 right-4 font-bold bg-green-500 hover:bg-green-600 border-none text-white shadow-sm"
          >
            {{ item.latest }}
          </Badge>
        </div>

        <CardHeader class="pb-2 text-center">
          <CardTitle>{{ item.displayName }}</CardTitle>
          <CardDescription>最近更新: {{ formatDate(item.lastUpdated) }}</CardDescription>
        </CardHeader>

        <CardContent class="flex-1"></CardContent>
        
        <CardFooter class="flex flex-col gap-2 pt-0">
             <Button 
                v-if="item.hasAssets"
                class="w-full" 
                as="a"
                :href="item.latestDownloadUrl"
              >
                <Download class="mr-2 h-4 w-4" />
                下载最新版
              </Button>
              <div class="flex gap-2 w-full">
                <Button 
                  variant="outline" 
                  class="flex-1"
                  @click="openHistory(item)"
                >
                  <History class="mr-2 h-4 w-4" />
                  历史版本
                </Button>
                <Button 
                    v-if="item.hasAssets"
                    variant="outline"
                    class="flex-1"
                    @click="copyLink(item.latestDownloadUrl, item.name)"
                >
                    <Check v-if="copiedStates[item.name]" class="mr-2 h-4 w-4 text-green-500" />
                    <Copy v-else class="mr-2 h-4 w-4" />
                    {{ copiedStates[item.name] ? '已复制' : '复制' }}
                </Button>
              </div>
        </CardFooter>
      </Card>
    </div>

    <!-- History Dialog -->
    <DialogRoot v-model:open="isDialogOpen">
      <DialogPortal>
        <DialogOverlay class="fixed inset-0 z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <DialogContent class="fixed left-[50%] top-[50%] z-50 grid w-[95vw] sm:w-full max-w-3xl translate-x-[-50%] translate-y-[-50%] gap-4 border bg-background p-6 shadow-lg duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%] sm:rounded-lg max-h-[85vh] h-[85vh] sm:h-auto overflow-hidden flex flex-col">
          <div class="flex flex-col space-y-1.5 text-center sm:text-left shrink-0">
            <DialogTitle class="text-lg font-semibold leading-none tracking-tight">
              {{ selectedLauncher?.displayName }} 版本历史
            </DialogTitle>
          </div>
          <div class="overflow-y-auto pr-2 -mr-2 flex-1 min-h-0">
             <div class="grid gap-4 md:grid-cols-2">
                 <Card v-for="v in selectedLauncher?.versions" :key="v.tag_name || v.name">
                     <CardHeader class="p-4 pb-2">
                         <div class="flex justify-between items-center">
                             <div class="font-semibold">{{ v.tag_name || v.name }}</div>
                             <Badge v-if="selectedLauncher.latest === (v.tag_name || v.name)" variant="secondary" class="bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 border-transparent">LATEST</Badge>
                         </div>
                         <div class="text-xs text-muted-foreground">{{ formatDate(v.published_at) }}</div>
                     </CardHeader>
                     <CardContent class="p-4 pt-2">
                         <div class="flex flex-col gap-2">
                             <div 
                                v-for="asset in v.assets" 
                                :key="asset.name"
                                class="flex items-center justify-between text-sm rounded-md border p-2 bg-muted/30"
                            >
                                <div class="flex-1 overflow-x-auto whitespace-nowrap scrollbar-hide mr-2">
                                    <span class="font-medium" :title="asset.name">{{ asset.name }}</span>
                                </div>
                                <div class="flex shrink-0 gap-1">
                                    <Button variant="ghost" size="icon" class="h-8 w-8" as="a" :href="getAssetUrl(selectedLauncher.name, v, asset)" :download="asset.name">
                                        <Download class="h-4 w-4" />
                                    </Button>
                                     <Button variant="ghost" size="icon" class="h-8 w-8" @click="copyLink(getAssetUrl(selectedLauncher.name, v, asset), `${v.tag_name}-${asset.name}`)">
                                        <Check v-if="copiedStates[`${v.tag_name}-${asset.name}`]" class="h-4 w-4 text-green-500" />
                                        <Copy v-else class="h-4 w-4" />
                                    </Button>
                                </div>
                             </div>
                         </div>
                     </CardContent>
                 </Card>
             </div>
          </div>
          <DialogClose class="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground">
            <X class="h-4 w-4" />
            <span class="sr-only">Close</span>
          </DialogClose>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </div>
</template>

<style scoped>
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>