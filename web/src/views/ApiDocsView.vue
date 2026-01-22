<script setup>
import { ref, computed, nextTick } from 'vue';
import { Search, Menu, Copy, Check, Terminal } from 'lucide-vue-next';
import { useClipboard } from '@vueuse/core';
import hljs from 'highlight.js/lib/core';
import json from 'highlight.js/lib/languages/json';
import bash from 'highlight.js/lib/languages/bash';
import 'highlight.js/styles/atom-one-dark.css'; // Or another dark theme

import Button from '@/components/ui/Button.vue';
import Input from '@/components/ui/Input.vue';
import Badge from '@/components/ui/Badge.vue';
import { Sheet, SheetTrigger, SheetContent } from '@/components/ui/sheet';
import { cn } from '@/lib/utils';

hljs.registerLanguage('json', json);
hljs.registerLanguage('bash', bash);

const searchQuery = ref('');
const isNavOpen = ref(false);
const { copy, copied } = useClipboard();
const copiedState = ref({});

const endpoints = [
  {
      method: 'GET',
      path: '/api/status',
      title: '获取所有版本状态',
      desc: '返回所有启动器及完整版本列表，包含版本号、发布时间、下载链接。此接口数据量较大，建议仅在初始化时调用。',
      response: `[
  {
    "hmcl": [
      {
        "tag_name": "v3.5.9",
        "name": "HMCL v3.5.9",
        "published_at": "2024-01-15T10:30:00Z",
        "assets": [
          {
            "name": "HMCL-3.5.9.exe",
            "size": 2856128,
            "url": "https://..."
          }
        ]
      }
    ]
  }
]`
  },
  {
      method: 'GET',
      path: '/api/status/{launcher}',
      title: '获取指定启动器状态',
      desc: '返回特定启动器的历史版本信息。',
      params: [
          { name: 'launcher', type: 'string', required: true, desc: '启动器标识 (如 hmcl, pcl2)' }
      ],
      response: `[
  {
    "tag_name": "v3.5.9",
    "published_at": "2024-01-15T10:30:00Z",
    "assets": []
  }
]`
  },
  {
      method: 'GET',
      path: '/api/latest',
      title: '获取所有最新版本',
      desc: '快速检查所有启动器的最新版本号，适合用于检测更新。',
      response: `{ 
  "hmcl": "v3.5.9",
  "pcl2": "Snapshot-20240115",
  "bakaxl": "v3.5.1"
}`
  },
  {
      method: 'GET',
      path: '/api/latest/{launcher}',
      title: '获取指定启动器最新版本',
      desc: '查询单个启动器的最新发布版本详情。',
      params: [
          { name: 'launcher', type: 'string', required: true, desc: '启动器标识' }
      ],
      response: `{ 
  "tag_name": "v3.5.9",
  "name": "HMCL v3.5.9",
  "published_at": "2024-01-15T10:30:00Z"
}`
  },
  {
      method: 'GET',
      path: '/api/stats',
      title: '获取统计数据',
      desc: '获取站点的访问统计、下载量、热门排行、地域分布等数据。',
      response: `{ 
  "totalDownloads": 152304,
  "totalVisits": 89234,
  "topDownloads": [...] 
}`
  },
  {
      method: 'POST',
      path: '/api/scan',
      title: '触发手动扫描',
      desc: '强制同步上游仓库检查新版本。此接口受频率限制。',
      response: `{ 
  "success": true,
  "message": "扫描完成"
}`
  },
];

const getMethodColor = (method) => ({
  GET: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300 border-blue-200 dark:border-blue-800',
  POST: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300 border-green-200 dark:border-green-800',
  PUT: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300 border-orange-200 dark:border-orange-800',
  DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300 border-red-200 dark:border-red-800'
}[method] || 'bg-gray-100 text-gray-700 border-gray-200');

const filteredEndpoints = computed(() => {
  const query = searchQuery.value.toLowerCase().trim();
  if (!query) return endpoints;
  return endpoints.filter(e => 
    e.path.toLowerCase().includes(query) ||
    e.title.toLowerCase().includes(query) ||
    e.desc.toLowerCase().includes(query)
  );
});

const copyCode = (text, id) => {
  copy(text);
  copiedState.value[id] = true;
  setTimeout(() => {
      copiedState.value[id] = false;
  }, 2000);
};

const scrollTo = (index) => {
    const el = document.getElementById(`endpoint-${index}`);
    if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
        isNavOpen.value = false;
    }
}

const highlightCode = (code, lang) => {
    return hljs.highlight(code, { language: lang }).value;
};
</script>

<template>
  <div class="flex flex-col lg:flex-row gap-8 min-h-[calc(100vh-4rem)]">
    <!-- Desktop Sidebar -->
    <aside class="hidden lg:block w-64 shrink-0 self-start sticky top-6 max-h-[calc(100vh-6rem)] overflow-y-auto pr-2">
        <div class="mb-6">
            <h4 class="font-semibold mb-2 px-2">API 概览</h4>
            <div class="relative">
                <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground" />
                <Input 
                    v-model="searchQuery" 
                    placeholder="筛选接口..." 
                    class="h-8 pl-8 text-xs"
                />
            </div>
        </div>
        <nav class="space-y-1">
             <button
                v-for="(endpoint, i) in filteredEndpoints" 
                :key="i"
                @click="scrollTo(i)"
                class="flex items-center w-full px-2 py-1.5 text-sm rounded-md hover:bg-muted/60 transition-colors text-left group"
             >
                <span :class="['inline-block w-10 text-[10px] text-center font-bold rounded px-1 py-0.5 mr-2 shrink-0 border uppercase', getMethodColor(endpoint.method)]">
                    {{ endpoint.method }}
                </span>
                <span class="truncate text-muted-foreground group-hover:text-foreground transition-colors">{{ endpoint.title }}</span>
             </button>
        </nav>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 min-w-0 space-y-12 pb-12">
        <div class="flex items-center justify-between lg:hidden mb-6">
             <h1 class="text-3xl font-bold tracking-tight">API 文档</h1>
             <Sheet v-model:open="isNavOpen">
                  <SheetTrigger as-child>
                      <Button variant="outline" size="icon">
                          <Menu class="h-4 w-4" />
                      </Button>
                  </SheetTrigger>
                  <SheetContent side="left" class="w-[85%] sm:w-[385px] p-0">
                      <div class="p-6 pb-2">
                          <div class="font-semibold mb-4 text-lg">目录</div>
                           <div class="relative mb-4">
                                <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                                <Input 
                                    v-model="searchQuery" 
                                    placeholder="筛选接口..." 
                                    class="pl-9" 
                                />
                            </div>
                      </div>
                      <nav class="space-y-1 px-4 overflow-y-auto max-h-[calc(100vh-140px)]">
                         <button
                            v-for="(endpoint, i) in filteredEndpoints" 
                            :key="i"
                            @click="scrollTo(i)"
                            class="flex items-center w-full px-2 py-3 text-sm font-medium rounded-md hover:bg-muted transition-colors text-left border-b border-muted/40 last:border-0"
                         >
                            <span :class="['inline-block w-12 text-center text-xs font-bold rounded px-1 py-0.5 mr-3 shrink-0 border', getMethodColor(endpoint.method)]">{{ endpoint.method }}</span>
                            <div class="flex flex-col overflow-hidden text-left">
                                <span class="font-medium truncate">{{ endpoint.path }}</span>
                                <span class="text-xs text-muted-foreground truncate">{{ endpoint.title }}</span>
                            </div>
                         </button>
                      </nav>
                  </SheetContent>
              </Sheet>
        </div>
        
        <div class="hidden lg:block space-y-4 border-b pb-8">
             <h1 class="text-4xl font-extrabold tracking-tight lg:text-5xl">API 文档</h1>
             <p class="text-lg text-muted-foreground max-w-2xl">
                 Lemwood Mirror 提供了一套简单、强大的 RESTful API，用于获取启动器版本信息、下载链接及站点统计数据。所有接口的基础路径为 <code class="bg-muted px-1.5 py-0.5 rounded text-sm font-mono border">/api</code>。
             </p>
        </div>

        <div 
            v-if="!filteredEndpoints.length" 
            class="text-center py-12 border rounded-xl border-dashed bg-muted/5"
        >
           <Search class="mx-auto h-12 w-12 text-muted-foreground/30 mb-4" />
           <p class="text-muted-foreground">未找到匹配的接口</p>
        </div>

        <div 
            v-for="(endpoint, i) in filteredEndpoints" 
            :key="i" 
            :id="`endpoint-${i}`"
            class="scroll-mt-24 group"
        >
            <div class="flex flex-col gap-6">
                <!-- Header -->
                <div class="space-y-3">
                    <div class="flex items-center gap-3">
                         <Badge :class="cn('px-2 py-1 text-xs font-bold border uppercase', getMethodColor(endpoint.method))">
                            {{ endpoint.method }}
                         </Badge>
                         <h2 class="text-2xl font-bold font-mono tracking-tight break-all">{{ endpoint.path }}</h2>
                    </div>
                    <p class="text-lg text-muted-foreground leading-relaxed">{{ endpoint.desc }}</p>
                </div>

                <!-- Parameters Table if exists -->
                <div v-if="endpoint.params" class="border rounded-lg overflow-hidden">
                    <div class="bg-muted/40 px-4 py-2 border-b text-sm font-medium">请求参数</div>
                    <div class="overflow-x-auto">
                        <table class="w-full text-sm">
                            <thead class="bg-muted/10">
                                <tr class="border-b text-left">
                                    <th class="py-2 px-4 font-medium text-muted-foreground w-32">参数名</th>
                                    <th class="py-2 px-4 font-medium text-muted-foreground w-24">类型</th>
                                    <th class="py-2 px-4 font-medium text-muted-foreground w-20">必填</th>
                                    <th class="py-2 px-4 font-medium text-muted-foreground">说明</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="param in endpoint.params" :key="param.name" class="border-b last:border-0 hover:bg-muted/5">
                                    <td class="py-3 px-4 font-mono text-primary">{{ param.name }}</td>
                                    <td class="py-3 px-4 text-muted-foreground font-mono text-xs">{{ param.type }}</td>
                                    <td class="py-3 px-4">
                                        <span v-if="param.required" class="text-red-500 font-medium text-xs">Yes</span>
                                        <span v-else class="text-muted-foreground text-xs">No</span>
                                    </td>
                                    <td class="py-3 px-4 text-muted-foreground">{{ param.desc }}</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>

                <!-- Code Examples -->
                <div class="grid gap-6 xl:grid-cols-2">
                    <!-- Request Example -->
                    <div class="space-y-2 min-w-0">
                        <div class="flex items-center justify-between px-1">
                             <span class="text-sm font-medium text-muted-foreground flex items-center gap-2">
                                 <Terminal class="h-4 w-4" /> cURL Request
                             </span>
                             <Button variant="ghost" size="icon" class="h-7 w-7" @click="copyCode(`curl -X ${endpoint.method} 'https://mirror.lemwood.icu${endpoint.path}'`, `curl-${i}`)">
                                 <Check v-if="copiedState[`curl-${i}`]" class="h-3.5 w-3.5 text-green-500" />
                                 <Copy v-else class="h-3.5 w-3.5 text-muted-foreground" />
                             </Button>
                        </div>
                        <div class="rounded-lg border bg-[#0d1117] overflow-hidden group/code relative">
                            <div class="p-4 overflow-x-auto custom-scrollbar">
                                <pre><code class="font-mono text-sm text-[#c9d1d9]" v-html="highlightCode(
                                    `curl -X ${endpoint.method} \
  'https://mirror.lemwood.icu${endpoint.path}'`,
                                    'bash'
                                )"></code></pre>
                            </div>
                        </div>
                    </div>

                    <!-- Response Example -->
                    <div class="space-y-2 min-w-0" v-if="endpoint.response">
                        <div class="flex items-center justify-between px-1">
                             <span class="text-sm font-medium text-muted-foreground">Response Example</span>
                             <Button variant="ghost" size="icon" class="h-7 w-7" @click="copyCode(endpoint.response, `res-${i}`)">
                                 <Check v-if="copiedState[`res-${i}`]" class="h-3.5 w-3.5 text-green-500" />
                                 <Copy v-else class="h-3.5 w-3.5 text-muted-foreground" />
                             </Button>
                        </div>
                         <div class="rounded-lg border bg-[#0d1117] overflow-hidden">
                            <div class="p-4 overflow-x-auto custom-scrollbar max-h-[300px]">
                                <pre><code class="font-mono text-sm text-[#c9d1d9]" v-html="highlightCode(endpoint.response, 'json')"></code></pre>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <div v-if="i < filteredEndpoints.length - 1" class="my-12 border-b border-dashed" />
        </div>
    </main>
  </div>
</template>

<style scoped>
/* Scrollbar styling for code blocks */
.custom-scrollbar::-webkit-scrollbar {
  height: 8px;
  width: 8px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: #484f58;
}

/* Atom One Dark inspired colors for manual overrides if hljs css fails or needs tweaking */
:deep(.hljs-string) { color: #a5d6ff; }
:deep(.hljs-attr) { color: #79c0ff; }
:deep(.hljs-keyword) { color: #ff7b72; }
:deep(.hljs-number) { color: #79c0ff; }
:deep(.hljs-literal) { color: #79c0ff; }
</style>
