<script setup>
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { getStatus, getLatest } from '@/services/api';
import { Search, File, Folder, Download, Copy, Loader2, FileArchive, HardDrive, ChevronRight, Home, ArrowLeft } from 'lucide-vue-next';
import Input from '@/components/ui/Input.vue';
import Button from '@/components/ui/Button.vue';
import Badge from '@/components/ui/Badge.vue';
import Skeleton from '@/components/ui/Skeleton.vue';
import { cn } from '@/lib/utils';
import { useClipboard } from '@vueuse/core';
import { getLauncherDisplayName } from '@/lib/launcher-info';

/**
 * 文件浏览页面的路由参数
 * @typedef {{ launcherName?: string, versionName?: string }} RouteProps
 */

/** @type {RouteProps} */
const props = defineProps({
  launcherName: String,
  versionName: String
});

const loading = ref(true);
const searchQuery = ref('');
const launchers = ref({});
const latestData = ref({});
const { copy, copied } = useClipboard();

const route = useRoute();
// 导航路径栈
const currentPath = ref([]);

/**
 * 加载启动器和版本数据
 */
const loadData = async () => {
  loading.value = true;
  try {
    const [statusRes, latestRes] = await Promise.all([getStatus(), getLatest()]);
    const sortedLaunchers = {};
    Object.keys(statusRes.data).sort().forEach(key => {
        sortedLaunchers[key] = statusRes.data[key].sort((a, b) =>
            String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name))
        );
    });
    launchers.value = sortedLaunchers;
    latestData.value = latestRes.data;
  } catch (error) {
    console.error(error);
  } finally {
    loading.value = false;
  }
};

/**
 * 根据文件扩展名获取图标
 * @param {string} filename - 文件名
 * @returns {import('lucide-vue-next').Component} 图标组件
 */
const getFileIcon = (filename) => {
  const ext = filename.split('.').pop()?.toLowerCase();
  if (['zip', 'tar', 'gz', '7z', 'rar'].includes(ext)) return FileArchive;
  if (['exe', 'msi', 'apk', 'dmg'].includes(ext)) return HardDrive;
  return File;
};

/**
 * 格式化日期
 * @param {string} dateString - 日期字符串
 * @returns {string} 格式化后的日期
 */
const formatDate = (dateString) => {
  if (!dateString) return '未知';
  try {
    return new Date(dateString).toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: 'short',
        day: 'numeric'
    });
  } catch {
    return dateString;
  }
};

/**
 * 复制URL到剪贴板
 * @param {string} url - 要复制的URL
 */
const copyUrl = (url) => {
  copy(url);
};

/**
 * 导航到指定项目
 * @param {Object} item - 要导航到的项目
 * @param {'launcher'|'version'} type - 项目类型
 */
const navigateTo = (item, type) => {
    if (type === 'launcher') {
        currentPath.value = [{ name: getLauncherDisplayName(item.id), id: item.id, type: 'launcher', displayName: item.id }];
    } else if (type === 'version') {
        currentPath.value.push({ name: item.name, id: item.id, type: 'version', data: item.data });
    }
    updateUrl();
};

/**
 * 返回上一级
 */
const navigateUp = () => {
    currentPath.value.pop();
    updateUrl();
};

/**
 * 导航到指定面包屑
 * @param {number} index - 面包屑索引
 */
const navigateToBreadcrumb = (index) => {
    if (index === -1) {
        currentPath.value = [];
        router.push({ name: 'files' });
    } else {
        // 只保留到指定索引的路径
        currentPath.value = currentPath.value.slice(0, index + 1);
        updateUrl();
    }
};

/**
 * 更新URL以反映当前路径
 */
const updateUrl = () => {
    if (currentPath.value.length === 0) {
        router.replace({ name: 'files' });
    } else if (currentPath.value.length === 1) {
        router.replace({ name: 'files-launcher', params: { launcherName: currentPath.value[0].id } });
    } else if (currentPath.value.length >= 2) {
        router.replace({
            name: 'files-version',
            params: {
                launcherName: currentPath.value[0].id,
                versionName: currentPath.value[1].id
            }
        });
    }
};

/**
 * 当前层级的项目列表
 */
const currentItems = computed(() => {
    const query = searchQuery.value.toLowerCase().trim();
    const depth = currentPath.value.length;

    if (depth === 0) {
        // 根目录：显示启动器
        return Object.keys(launchers.value).map(name => ({
            id: name,
            name: getLauncherDisplayName(name),
            displayName: name, // 保留原始名称用于路由等用途
            type: 'launcher',
            count: launchers.value[name].length,
            latest: latestData.value[name]
        })).filter(l => !query || l.name.toLowerCase().includes(query));
    } else if (depth === 1) {
        // 启动器目录：显示版本
        const launcherName = currentPath.value[0].id;
        const versions = launchers.value[launcherName] || [];

        return versions.map(v => ({
            id: v.tag_name || v.name,
            name: v.tag_name || v.name,
            type: 'version',
            date: v.published_at,
            isLatest: latestData.value[launcherName] === (v.tag_name || v.name),
            data: v,
            fileCount: v.assets?.length || 0
        })).filter(v => !query || v.name.toLowerCase().includes(query));
    } else if (depth === 2) {
         // 版本目录：显示文件
         const versionData = currentPath.value[1].data;
         const launcherName = currentPath.value[0].id;
         const versionName = currentPath.value[1].id;

         return (versionData.assets || []).map(asset => ({
             id: asset.name,
             name: asset.name,
             type: 'file',
             size: asset.size,
             downloadUrl: asset.url && asset.url.startsWith('http')
              ? asset.url
              : `${window.location.origin}/download/${launcherName}/${versionName}/${asset.name}`
         })).filter(f => !query || f.name.toLowerCase().includes(query));
    }
    return [];
});

/**
 * 初始化页面数据和路径
 */
onMounted(async () => {
  await loadData();
  
  // 使用路由参数初始化路径
  if (props.launcherName) {
    if (launchers.value[props.launcherName]) {
      currentPath.value = [{ name: getLauncherDisplayName(props.launcherName), id: props.launcherName, type: 'launcher', displayName: props.launcherName }];
      
      if (props.versionName) {
        const versions = launchers.value[props.launcherName] || [];
        const versionData = versions.find(v => (v.tag_name || v.name) === props.versionName);
        
        if (versionData) {
          currentPath.value.push({ 
            name: props.versionName, 
            id: props.versionName, 
            type: 'version', 
            data: versionData 
          });
        }
      }
    }
  }
});
</script>

<template>
  <div class="flex flex-col h-full space-y-4 max-w-full">
    <!-- Header & Breadcrumbs -->
    <div class="flex flex-col space-y-4 shrink-0">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 sm:gap-4 w-full">
             <div class="flex items-center gap-1 sm:gap-2 overflow-hidden text-sm font-medium text-muted-foreground min-w-0">
                 <Button variant="ghost" size="icon" class="h-8 w-8 shrink-0" @click="navigateToBreadcrumb(-1)" :disabled="!currentPath.length">
                     <Home class="h-4 w-4" />
                 </Button>
                 <div class="flex overflow-x-auto py-1 hide-scrollbar min-w-0">
                     <template v-for="(crumb, index) in currentPath" :key="crumb.id">
                         <ChevronRight class="h-4 w-4 shrink-0 opacity-50 my-auto" />
                         <Button 
                           variant="ghost" 
                           size="sm" 
                           class="h-8 px-2 truncate max-w-[100px] sm:max-w-[120px] whitespace-nowrap" 
                           @click="navigateToBreadcrumb(index)">
                             {{ crumb.name }}
                         </Button>
                     </template>
                 </div>
             </div>
             <div class="relative w-full sm:w-40 md:w-64 shrink-0">
                 <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                 <Input
                   v-model="searchQuery"
                   type="search"
                   placeholder="筛选..."
                   class="pl-8 h-9 w-full bg-background/50 backdrop-blur border-white/10"
                 />
             </div>
        </div>
    </div>

    <!-- Content Area -->
    <div v-if="loading" class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4">
        <Skeleton class="aspect-square rounded-xl" v-for="i in 10" :key="i" />
    </div>

    <div v-else-if="!currentItems.length" class="flex flex-col items-center justify-center py-20 text-muted-foreground">
        <Folder class="h-16 w-16 mb-4 opacity-20" />
        <p>空文件夹</p>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-2 pb-20">
        <!-- Back Button (if not root) -->
        <div 
            v-if="currentPath.length > 0" 
            @click="navigateUp"
            class="group relative flex flex-row items-center p-3 rounded-xl border border-dashed border-muted-foreground/20 bg-muted/5 hover:bg-muted/10 cursor-pointer transition-all hover:scale-[1.02] active:scale-[0.98] w-full max-w-[400px] mx-auto sm:max-w-none"
        >
             <ArrowLeft class="h-6 w-6 text-muted-foreground group-hover:text-foreground transition-colors mr-3" />
             <span class="text-xs font-medium text-muted-foreground flex-1">返回上一级</span>
        </div>

        <!-- Items -->
        <div 
            v-for="item in currentItems" 
            :key="item.id"
            @click="item.type !== 'file' ? navigateTo(item, item.type) : null"
            :class="cn(
                'group relative flex flex-row items-center p-3 rounded-xl border border-white/5 bg-background/40 backdrop-blur-md shadow-sm transition-all duration-200',
                'w-full max-w-[400px] mx-auto sm:max-w-none', // 在移动端居中并限制宽度
                item.type !== 'file' ? 'cursor-pointer hover:bg-background/60 hover:border-white/20 hover:shadow-md hover:scale-[1.02] active:scale-[0.98]' : ''
            )"
        >
            <!-- Icon Area -->
            <div class="flex-shrink-0 p-2 rounded-lg bg-gradient-to-br from-primary/10 to-primary/5 text-primary group-hover:from-primary/20 group-hover:to-primary/10 transition-colors mr-3">
                 <Folder v-if="item.type === 'launcher'" class="h-5 w-5" />
                 <Folder v-else-if="item.type === 'version'" class="h-5 w-5" />
                 <component v-else :is="getFileIcon(item.name)" class="h-5 w-5" />
            </div>
            
            <!-- Text Area -->
            <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between">
                    <h3 class="font-medium truncate text-sm leading-none mr-2" :title="item.name">{{ item.name }}</h3>
                    <div v-if="item.isLatest" class="px-1.5 py-0.5 rounded-full bg-green-500/10 text-green-600 text-[9px] font-bold uppercase tracking-wider border border-green-500/20 flex-shrink-0">
                        Latest
                    </div>
                </div>
                <div class="flex items-center justify-between text-[10px] text-muted-foreground mt-1">
                    <span v-if="item.type === 'launcher'">{{ item.count }} 版本</span>
                    <span v-else-if="item.type === 'version'">{{ formatDate(item.date) }}</span>
                    <span v-else>文件</span>
                </div>
            </div>
            
            <div v-if="item.type === 'file'" class="flex-shrink-0 flex gap-0.5 ml-2">
                 <Button size="icon" variant="ghost" class="h-6 w-6" @click.stop="copyUrl(item.downloadUrl)">
                     <Copy class="h-3 w-3" />
                 </Button>
                 <Button size="icon" variant="ghost" class="h-6 w-6" as="a" :href="item.downloadUrl">
                     <Download class="h-3 w-3" />
                 </Button>
            </div>
        </div>
    </div>
  </div>
</template>


<style scoped>
.hide-scrollbar::-webkit-scrollbar {
  display: none;
}
.hide-scrollbar {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
</style>
