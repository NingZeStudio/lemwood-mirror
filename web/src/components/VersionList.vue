<script setup>
import { ref, computed, onMounted } from 'vue';
import { getStatus, getLatest } from '@/services/api';
import { Download, History, X, Loader2, Package } from 'lucide-vue-next';
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
import zlLogo from '@/assets/images/34c1ec9e07f826df.webp'
import zl2Logo from '@/assets/images/ee0028bd82493eb3.webp'
import hmclLogo from '@/assets/images/3835841e4b9b7abf.jpeg'
import mgLogo from '@/assets/images/3625548d2639a024.png'
import fclLogo from '@/assets/images/dc5e0ee14d8f54f0.png'
import fclTurnipLogo from '@/assets/images/Image_1770256620866_693.webp'
import shizukuLogo from '@/assets/images/f7067665f073b4cc.png'
import luminolLogo from '@/assets/images/c25a955166388e1257c23d01c78a62e6.webp'
import leafLogo from '@/assets/images/leaf.png'
import leavesLogo from '@/assets/images/Leaves.png'
import { LAUNCHER_INFO_MAP } from '@/lib/launcher-info';

const EXTENDED_LAUNCHER_INFO_MAP = {
  ...LAUNCHER_INFO_MAP,
  'zl': { ...LAUNCHER_INFO_MAP['zl'], logoUrl: zlLogo },
  'zl2': { ...LAUNCHER_INFO_MAP['zl2'], logoUrl: zl2Logo },
  'hmcl': { ...LAUNCHER_INFO_MAP['hmcl'], logoUrl: hmclLogo },
  'MG': { ...LAUNCHER_INFO_MAP['MG'], logoUrl: mgLogo },
  'fcl': { ...LAUNCHER_INFO_MAP['fcl'], logoUrl: fclLogo },
  'FCL_Turnip': { ...LAUNCHER_INFO_MAP['FCL_Turnip'], logoUrl: fclTurnipLogo },
  'shizuku': { ...LAUNCHER_INFO_MAP['shizuku'], logoUrl: shizukuLogo },
  'leaves': { ...LAUNCHER_INFO_MAP['leaves'], logoUrl: leavesLogo },
  'leaf': { ...LAUNCHER_INFO_MAP['leaf'], logoUrl: leafLogo },
  'luminol': { ...LAUNCHER_INFO_MAP['luminol'], logoUrl: luminolLogo }
};


const rawLaunchers = ref({});
const latestMap = ref({});
const loading = ref(true);

const launcherList = computed(() => {
  return Object.keys(rawLaunchers.value).map(name => {
    const versions = rawLaunchers.value[name];
    const latestVersion = latestMap.value[name];
    const latestObj = versions.find(v => (v.tag_name || v.name) === latestVersion) || versions[0];
    const info = EXTENDED_LAUNCHER_INFO_MAP[name] || { displayName: name, logoUrl: fclTurnipLogo };

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
                  as="a"
                  :href="`/#/files/${item.name}`"
                >
                  <History class="mr-2 h-4 w-4" />
                  历史版本
                </Button>
              </div>
        </CardFooter>
      </Card>
    </div>

  </div>
</template>

<style scoped>
/* 无特殊样式 */
</style>
