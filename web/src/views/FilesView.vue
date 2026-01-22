<script setup>
import { ref, computed, onMounted } from 'vue';
import { getStatus, getLatest } from '@/services/api';
import { TabsRoot, TabsList, TabsTrigger, TabsContent } from 'radix-vue';
import { Search, File, Folder, Download, Copy, Loader2, FileArchive, HardDrive } from 'lucide-vue-next';
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import Input from '@/components/ui/Input.vue';
import Button from '@/components/ui/Button.vue';
import Badge from '@/components/ui/Badge.vue';
import Skeleton from '@/components/ui/Skeleton.vue';
import { cn } from '@/lib/utils';
import { useClipboard } from '@vueuse/core';

const loading = ref(true);
const searchQuery = ref('');
const launchers = ref([]);
const latestData = ref({});
const activeTab = ref('');
const { copy, copied } = useClipboard();

const loadData = async () => {
  loading.value = true;
  
  try {
    const [statusRes, latestRes] = await Promise.all([getStatus(), getLatest()]);
    
    launchers.value = Object.entries(statusRes.data).map(([name, versions]) => ({
      name,
      versions: versions.sort((a, b) => 
        String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name))
      )
    }));
    
    latestData.value = latestRes.data;
    
    if (launchers.value.length && !activeTab.value) {
      activeTab.value = launchers.value[0].name;
    }
  } catch (error) {
    console.error(error);
  } finally {
    loading.value = false;
  }
};

const getFileIcon = (filename) => {
  const ext = filename.split('.').pop()?.toLowerCase();
  if (['zip', 'tar', 'gz', '7z', 'rar'].includes(ext)) return FileArchive;
  if (['exe', 'msi', 'apk', 'dmg'].includes(ext)) return HardDrive; // Application-ish
  return File;
};

const formatDate = (dateString) => {
  if (!dateString) return '未知';
  try {
    return new Date(dateString).toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    });
  } catch {
    return dateString;
  }
};

const copyUrl = (url) => {
  copy(url);
};

const filteredLaunchers = computed(() => {
  const query = searchQuery.value.toLowerCase().trim();
  const result = [];

  launchers.value.forEach(launcher => {
    const files = [];
    
    launcher.versions.forEach(version => {
      const versionName = version.tag_name || version.name;
      const isLatest = latestData.value[launcher.name] === versionName;
      
      version.assets?.forEach(asset => {
        const matchesSearch = !query || 
          launcher.name.toLowerCase().includes(query) ||
          versionName.toLowerCase().includes(query) ||
          asset.name.toLowerCase().includes(query);
        
        if (matchesSearch) {
          files.push({
            id: `${launcher.name}-${versionName}-${asset.name}`,
            name: asset.name,
            version: versionName,
            published_at: version.published_at,
            isLatest,
            launcher: launcher.name,
            downloadUrl: asset.url && asset.url.startsWith('http') 
              ? asset.url 
              : `${window.location.origin}/download/${launcher.name}/${versionName}/${asset.name}`
          });
        }
      });
    });

    if (files.length) {
      result.push({
        name: launcher.name,
        files: files.sort((a, b) => b.isLatest - a.isLatest || b.published_at.localeCompare(a.published_at)),
        totalFiles: files.length
      });
    }
  });

  return result.sort((a, b) => a.name.localeCompare(b.name));
});

onMounted(() => {
  loadData();
});
</script>

<template>
  <div class="flex flex-col h-full space-y-6 max-w-full overflow-hidden">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 shrink-0">
       <div class="min-w-0">
           <h2 class="text-3xl font-bold tracking-tight truncate">文件浏览</h2>
           <p class="text-muted-foreground truncate">浏览所有已收录的镜像文件</p>
       </div>
       <div class="relative w-full md:w-96">
           <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
           <Input
             v-model="searchQuery"
             type="search"
             placeholder="搜索文件名、版本或启动器..."
             class="pl-8 bg-background"
           />
       </div>
    </div>

    <div v-if="loading" class="space-y-4">
       <div class="flex gap-2 overflow-x-auto pb-2">
         <Skeleton class="h-10 w-24 rounded-md shrink-0" v-for="i in 4" :key="i" />
       </div>
       <div class="rounded-md border">
          <div class="p-4 space-y-4">
             <Skeleton class="h-10 w-full" v-for="i in 5" :key="i" />
          </div>
       </div>
    </div>

    <div v-else-if="!filteredLaunchers.length" class="flex flex-col items-center justify-center p-12 border rounded-lg border-dashed bg-muted/10">
       <div class="flex h-20 w-20 items-center justify-center rounded-full bg-muted">
           <Folder class="h-10 w-10 text-muted-foreground opacity-50" />
       </div>
       <h3 class="mt-4 text-lg font-semibold">无匹配结果</h3>
       <p class="mb-4 mt-2 text-center text-sm text-muted-foreground max-w-sm">
         我们找不到与您的搜索相关的任何文件。请尝试使用不同的关键词。
       </p>
       <Button variant="outline" @click="loadData">刷新列表</Button>
    </div>

    <TabsRoot v-else v-model="activeTab" class="flex flex-col space-y-4 min-h-0">
      <div class="overflow-x-auto pb-2 -mx-1 px-1 shrink-0">
          <TabsList class="inline-flex h-10 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground">
            <TabsTrigger
              v-for="launcher in filteredLaunchers"
              :key="launcher.name"
              :value="launcher.name"
              class="inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm"
            >
              {{ launcher.name }}
              <span class="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{{ launcher.totalFiles }}</span>
            </TabsTrigger>
          </TabsList>
      </div>

      <TabsContent
        v-for="launcher in filteredLaunchers"
        :key="launcher.name"
        :value="launcher.name"
        class="outline-none min-h-0"
      >
        <div class="rounded-md border bg-card overflow-hidden">
            <div class="overflow-x-auto w-full">
                <Table class="min-w-full">
                    <TableHeader>
                        <TableRow>
                            <TableHead class="min-w-[150px] sm:min-w-[200px] max-w-[300px]">文件名</TableHead>
                            <TableHead class="min-w-[80px]">版本</TableHead>
                            <TableHead class="hidden md:table-cell min-w-[150px]">发布时间</TableHead>
                            <TableHead class="text-right min-w-[100px]">操作</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        <TableRow v-for="file in launcher.files" :key="file.id">
                            <TableCell class="font-medium">
                                <div class="flex items-center gap-3 min-w-0">
                                    <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded bg-muted/50">
                                        <component :is="getFileIcon(file.name)" class="h-5 w-5 text-muted-foreground" />
                                    </div>
                                    <div class="flex flex-col min-w-0 flex-1">
                                        <div class="overflow-x-auto whitespace-nowrap scrollbar-hide">
                                            <span class="font-medium" :title="file.name">{{ file.name }}</span>
                                        </div>
                                        <span class="md:hidden text-xs text-muted-foreground">{{ formatDate(file.published_at) }}</span>
                                    </div>
                                    <Badge v-if="file.isLatest" variant="secondary" class="bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900 dark:text-green-100 border-transparent h-5 text-[10px] px-1.5 shrink-0">LATEST</Badge>
                                </div>
                            </TableCell>
                            <TableCell>
                                <div class="inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 text-foreground">
                                    {{ file.version }}
                                </div>
                            </TableCell>
                            <TableCell class="hidden md:table-cell text-muted-foreground text-sm">
                                {{ formatDate(file.published_at) }}
                            </TableCell>
                            <TableCell class="text-right">
                                <div class="flex justify-end gap-2">
                                    <Button size="sm" variant="outline" class="h-8 px-2 lg:px-3" as="a" :href="file.downloadUrl">
                                        <Download class="mr-2 h-3.5 w-3.5" />
                                        <span class="hidden lg:inline">下载</span>
                                    </Button>
                                    <Button size="sm" variant="ghost" class="h-8 w-8 px-0" @click="copyUrl(file.downloadUrl)">
                                        <Copy class="h-3.5 w-3.5" />
                                        <span class="sr-only">复制链接</span>
                                    </Button>
                                </div>
                            </TableCell>
                        </TableRow>
                    </TableBody>
                </Table>
            </div>
        </div>
      </TabsContent>
    </TabsRoot>
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
