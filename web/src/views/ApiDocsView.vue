<script setup>
import { computed, onMounted, ref } from 'vue'
import { Search, Menu, Copy, Check, Terminal } from 'lucide-vue-next'
import { useClipboard } from '@vueuse/core'
import hljs from 'highlight.js/lib/core'
import json from 'highlight.js/lib/languages/json'
import bash from 'highlight.js/lib/languages/bash'
import 'highlight.js/styles/github-dark.css'
import Button from '@/components/ui/Button.vue'
import Input from '@/components/ui/Input.vue'
import Badge from '@/components/ui/Badge.vue'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import { Sheet, SheetTrigger, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/ui/sheet'
import Table from '@/components/ui/table/Table.vue'
import TableHeader from '@/components/ui/table/TableHeader.vue'
import TableBody from '@/components/ui/table/TableBody.vue'
import TableRow from '@/components/ui/table/TableRow.vue'
import TableHead from '@/components/ui/table/TableHead.vue'
import TableCell from '@/components/ui/table/TableCell.vue'
import { cn } from '@/lib/utils'

hljs.registerLanguage('json', json)
hljs.registerLanguage('bash', bash)

const searchQuery = ref('')
const isNavOpen = ref(false)
const copiedState = ref({})
const { copy } = useClipboard()

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
  }
]

const methodClasses = {
  GET: 'bg-blue-500/10 text-blue-700 dark:text-blue-300 border-blue-500/20',
  POST: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20',
  PUT: 'bg-amber-500/10 text-amber-700 dark:text-amber-300 border-amber-500/20',
  DELETE: 'bg-destructive/10 text-destructive border-destructive/20'
}

const getMethodClass = (method) => methodClasses[method] || 'bg-muted text-muted-foreground border-border'

const filteredEndpoints = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return endpoints
  return endpoints.filter((e) =>
    e.path.toLowerCase().includes(query) ||
    e.title.toLowerCase().includes(query) ||
    e.desc.toLowerCase().includes(query)
  )
})

const copyCode = async (text, id) => {
  await copy(text)
  copiedState.value[id] = true
  setTimeout(() => {
    copiedState.value[id] = false
  }, 2000)
}

const scrollTo = (index) => {
  const el = document.getElementById(`endpoint-${index}`)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    isNavOpen.value = false
  }
}

const highlightCode = (code, lang) => hljs.highlight(code, { language: lang }).value

onMounted(() => {
  document.title = 'API 文档 - 柠枺镜像状态'
  updateMetaDescription('柠枺镜像站 API 接口文档，提供版本查询、文件下载、统计信息等接口')
})

const updateMetaDescription = (desc) => {
  const metaDescription = document.querySelector('meta[name="description"]')
  const metaOgDescription = document.querySelector('meta[property="og:description"]')
  const metaTwitterDescription = document.querySelector('meta[property="twitter:description"]')

  if (metaDescription) metaDescription.setAttribute('content', desc)
  if (metaOgDescription) metaOgDescription.setAttribute('content', 'API 文档 - ' + desc)
  if (metaTwitterDescription) metaTwitterDescription.setAttribute('content', 'API 文档 - ' + desc)
}
</script>

<template>
  <div class="space-y-6">
    <Card>
      <CardHeader class="space-y-2">
        <CardTitle class="text-3xl">API 文档</CardTitle>
        <CardDescription>
          Lemwood Mirror 提供一套简单、直观的 RESTful API，用于获取版本信息、下载链接和站点统计数据。
        </CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="relative w-full sm:max-w-sm">
          <Search class="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input v-model="searchQuery" placeholder="筛选接口..." class="pl-9" />
        </div>
        <div class="lg:hidden">
          <Sheet v-model:open="isNavOpen">
            <SheetTrigger as-child>
              <Button variant="outline">
                <Menu class="mr-2 h-4 w-4" />
                目录
              </Button>
            </SheetTrigger>
            <SheetContent side="left" class="w-[85%] p-0 sm:w-[385px]">
              <SheetHeader class="p-6 pb-3 text-left">
                <SheetTitle>API 目录</SheetTitle>
                <SheetDescription>快速跳转到指定接口。</SheetDescription>
              </SheetHeader>
              <div class="px-6 pb-4">
                <Input v-model="searchQuery" placeholder="筛选接口..." />
              </div>
              <nav class="space-y-1 px-4 pb-6">
                <button
                  v-for="(endpoint, i) in filteredEndpoints"
                  :key="i"
                  type="button"
                  class="flex w-full items-start gap-3 rounded-md px-3 py-3 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
                  @click="scrollTo(i)"
                >
                  <Badge :class="cn('shrink-0 border', getMethodClass(endpoint.method))">{{ endpoint.method }}</Badge>
                  <div class="min-w-0">
                    <div class="truncate font-medium">{{ endpoint.path }}</div>
                    <div class="truncate text-xs text-muted-foreground">{{ endpoint.title }}</div>
                  </div>
                </button>
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </CardContent>
    </Card>

    <div class="flex gap-8">
      <aside class="sticky top-6 hidden h-fit w-72 shrink-0 lg:block">
        <Card>
          <CardHeader class="space-y-2 pb-3">
            <CardTitle class="text-lg">API 目录</CardTitle>
            <CardDescription>点击接口快速定位。</CardDescription>
          </CardHeader>
          <CardContent class="space-y-1">
            <button
              v-for="(endpoint, i) in filteredEndpoints"
              :key="i"
              type="button"
              class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground"
              @click="scrollTo(i)"
            >
              <Badge :class="cn('shrink-0 border', getMethodClass(endpoint.method))">{{ endpoint.method }}</Badge>
              <span class="truncate text-muted-foreground">{{ endpoint.title }}</span>
            </button>
          </CardContent>
        </Card>
      </aside>

      <main class="min-w-0 flex-1 space-y-8 pb-12">
        <div v-if="!filteredEndpoints.length" class="rounded-lg border border-dashed py-12 text-center text-muted-foreground">
          <Search class="mx-auto mb-4 h-12 w-12 opacity-30" />
          <p>未找到匹配的接口</p>
        </div>

        <Card
          v-for="(endpoint, i) in filteredEndpoints"
          :key="i"
          :id="`endpoint-${i}`"
          class="scroll-mt-24"
        >
          <CardHeader class="space-y-3">
            <div class="flex flex-wrap items-center gap-3">
              <Badge :class="cn('border font-bold uppercase', getMethodClass(endpoint.method))">{{ endpoint.method }}</Badge>
              <h2 class="text-2xl font-bold tracking-tight break-all">{{ endpoint.path }}</h2>
            </div>
            <CardDescription class="text-base">{{ endpoint.desc }}</CardDescription>
          </CardHeader>

          <CardContent class="space-y-6">
            <div v-if="endpoint.params" class="space-y-3 rounded-lg border">
              <div class="border-b bg-muted/40 px-4 py-3 text-sm font-medium">请求参数</div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead class="w-32">参数名</TableHead>
                    <TableHead class="w-24">类型</TableHead>
                    <TableHead class="w-20">必填</TableHead>
                    <TableHead>说明</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="param in endpoint.params" :key="param.name">
                    <TableCell class="font-mono text-primary">{{ param.name }}</TableCell>
                    <TableCell class="font-mono text-xs text-muted-foreground">{{ param.type }}</TableCell>
                    <TableCell>
                      <span v-if="param.required" class="text-sm font-medium text-destructive">Yes</span>
                      <span v-else class="text-sm text-muted-foreground">No</span>
                    </TableCell>
                    <TableCell class="text-muted-foreground">{{ param.desc }}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>

            <div class="grid gap-6 xl:grid-cols-2">
              <div class="space-y-2 min-w-0">
                <div class="flex items-center justify-between px-1">
                  <span class="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                    <Terminal class="h-4 w-4" /> cURL Request
                  </span>
                  <Button variant="ghost" size="icon" class="h-8 w-8" @click="copyCode(`curl -X ${endpoint.method} 'https://miawa.cn${endpoint.path}'`, `curl-${i}`)">
                    <Check v-if="copiedState[`curl-${i}`]" class="h-4 w-4 text-emerald-500" />
                    <Copy v-else class="h-4 w-4 text-muted-foreground" />
                  </Button>
                </div>
                <div class="overflow-hidden rounded-lg border bg-slate-950">
                  <div class="overflow-x-auto p-4">
                    <pre><code class="font-mono text-sm text-slate-50" v-html="highlightCode(`curl -X ${endpoint.method} \
  'https://miawa.cn${endpoint.path}'`, 'bash')"></code></pre>
                  </div>
                </div>
              </div>

              <div v-if="endpoint.response" class="space-y-2 min-w-0">
                <div class="flex items-center justify-between px-1">
                  <span class="text-sm font-medium text-muted-foreground">Response Example</span>
                  <Button variant="ghost" size="icon" class="h-8 w-8" @click="copyCode(endpoint.response, `res-${i}`)">
                    <Check v-if="copiedState[`res-${i}`]" class="h-4 w-4 text-emerald-500" />
                    <Copy v-else class="h-4 w-4 text-muted-foreground" />
                  </Button>
                </div>
                <div class="overflow-hidden rounded-lg border bg-slate-950">
                  <div class="max-h-[320px] overflow-x-auto overflow-y-auto p-4">
                    <pre><code class="font-mono text-sm text-slate-50" v-html="highlightCode(endpoint.response, 'json')"></code></pre>
                  </div>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </main>
    </div>
  </div>
</template>

<style scoped>
:deep(.hljs) {
  background: transparent;
}

:deep(.hljs-string) {
  color: #a5d6ff;
}

:deep(.hljs-attr) {
  color: #79c0ff;
}

:deep(.hljs-keyword) {
  color: #ff7b72;
}

:deep(.hljs-number) {
  color: #79c0ff;
}

:deep(.hljs-literal) {
  color: #79c0ff;
}
</style>
