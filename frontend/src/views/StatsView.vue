<script setup>
import { ref, onMounted, computed } from 'vue'
import { getStats } from '@/services/api'
import { useDark } from '@vueuse/core'
import { globalConfig } from '@/lib/globalConfig'
import {
  Activity,
  ArrowDown,
  ArrowUp,
  BarChart3,
  Download,
  Eye,
  Globe,
  HardDrive,
  MapPin,
  Server,
  TrendingUp
} from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardContent from '@/components/ui/CardContent.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import Skeleton from '@/components/ui/Skeleton.vue'

import { use, registerMap } from 'echarts/core'
import worldMap from '@/assets/world.json'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, MapChart, LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent
} from 'echarts/components'
import VChart from 'vue-echarts'

use([
  CanvasRenderer, BarChart, MapChart, LineChart,
  TitleComponent, TooltipComponent, LegendComponent, GridComponent, VisualMapComponent
])

const stats = ref({})
const loading = ref(true)
const mapLoaded = ref(false)
const isDark = useDark()

const formatBytes = (bytes) => {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const diskPercentage = computed(() => {
  if (!stats.value.disk || !stats.value.disk.total) return 0
  return Math.round((stats.value.disk.used / stats.value.disk.total) * 100)
})

const avgDailyVisits = computed(() => {
  if (!stats.value.total_days || stats.value.total_days === 0) return 0
  return stats.value.total_visits / stats.value.total_days
})

const avgDailyDownloads = computed(() => {
  if (!stats.value.total_days || stats.value.total_days === 0) return 0
  return stats.value.total_downloads / stats.value.total_days
})

const avgDailyRepoDownloads = computed(() => {
  if (!stats.value.total_days || stats.value.total_days === 0) return 0
  return (stats.value.total_repo_downloads || 0) / stats.value.total_days
})

const visitGrowth = computed(() => {
  const avg = avgDailyVisits.value
  const recent = (stats.value.last_30_visits || 0) / 30
  if (avg === 0) return 0
  return ((recent - avg) / avg * 100)
})

const downloadGrowth = computed(() => {
  const avg = avgDailyDownloads.value
  const recent = (stats.value.last_30_downloads || 0) / 30
  if (avg === 0) return 0
  return ((recent - avg) / avg * 100)
})

const repoDownloadGrowth = computed(() => {
  const avg = avgDailyRepoDownloads.value
  const recent = (stats.value.last_30_repo_downloads || 0) / 30
  if (avg === 0) return 0
  return ((recent - avg) / avg * 100)
})

const mapCountryName = (name) => {
  if (!name) return ''
  const nameMap = {
    '中国': 'China', 'CN': 'China', 'China': 'China',
    '美国': 'United States', 'USA': 'United States', 'US': 'United States',
    '英国': 'United Kingdom', 'UK': 'United Kingdom',
    '俄罗斯': 'Russia', 'Russian Federation': 'Russia',
    '德国': 'Germany', '法国': 'France', '日本': 'Japan',
    '韩国': 'South Korea', '加拿大': 'Canada', '澳大利亚': 'Australia',
    '巴西': 'Brazil', '印度': 'India', '新加坡': 'Singapore'
  }
  return nameMap[name] || nameMap[name.trim()] || name
}

const mapOption = computed(() => {
  const textColor = isDark.value ? '#a1a1aa' : '#52525b'
  const borderColor = isDark.value ? '#27272a' : '#e4e4e7'
  const areaColor = isDark.value ? '#27272a' : '#f4f4f5'

  const data = (stats.value.geo_distribution || []).map(item => ({
    name: mapCountryName(item.country),
    value: item.count
  }))

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: isDark.value ? '#18181b' : '#ffffff',
      borderColor: borderColor,
      textStyle: { color: isDark.value ? '#fafafa' : '#09090b' },
      formatter: params => `${params.name}: ${Number.isFinite(params.value) ? params.value : 0}`
    },
    visualMap: {
      min: 0,
      max: data.length ? Math.max(...data.map(d => d.value)) : 100,
      left: 'left',
      top: 'bottom',
      text: ['多', '少'],
      calculable: true,
      inRange: { color: ['#e0f2f1', '#0f766e'] },
      textStyle: { color: textColor }
    },
    series: [{
      name: 'Visits',
      type: 'map',
      map: 'world',
      roam: true,
      emphasis: {
        label: { show: false },
        itemStyle: { areaColor: '#f97316' }
      },
      itemStyle: {
        areaColor: areaColor,
        borderColor: borderColor
      },
      data: data
    }]
  }
})

const trendOption = computed(() => {
  const textColor = isDark.value ? '#a1a1aa' : '#52525b'
  const splitLineColor = isDark.value ? '#27272a' : '#e4e4e7'

  if (!stats.value.daily_stats) return {}

  const rawData = [...stats.value.daily_stats].reverse()
  const dates = rawData.map(d => d.date.slice(5))
  const visits = rawData.map(d => d.visit_count)
  const downloads = rawData.map(d => d.download_count)
  const repoDownloads = rawData.map(d => d.repo_download_count || 0)
  const traffic = rawData.map(d => d.traffic_bytes || 0)
  const repoTraffic = rawData.map(d => d.repo_traffic_bytes || 0)

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' },
      backgroundColor: isDark.value ? '#18181b' : '#ffffff',
      borderColor: splitLineColor,
      textStyle: { color: isDark.value ? '#fafafa' : '#09090b' },
      formatter: (params) => {
        let html = params[0].axisValue
        params.forEach(p => {
          const val = p.seriesName.includes('流量') ? formatBytes(p.value) : p.value
          html += `<br/>${p.marker} ${p.seriesName}: ${val}`
        })
        return html
      }
    },
    legend: {
      data: ['访问量', '下载量', 'Repo 拉取量', '下载流量', 'Repo 流量'],
      textStyle: { color: textColor },
      bottom: 0
    },
    grid: {
      left: '10px', right: '10px', bottom: '30px', top: '10px', containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLine: { lineStyle: { color: splitLineColor } },
      axisLabel: { color: textColor }
    },
    yAxis: [
      {
        type: 'value',
        splitLine: { lineStyle: { type: 'dashed', color: splitLineColor } },
        axisLine: { show: false },
        axisLabel: { color: textColor }
      },
      {
        type: 'value',
        splitLine: { show: false },
        axisLine: { show: false },
        axisLabel: {
          color: textColor,
          formatter: (v) => formatBytes(v)
        }
      }
    ],
    series: [
      {
        name: '访问量',
        type: 'line',
        smooth: true,
        symbol: 'none',
        data: visits,
        itemStyle: { color: '#f97316' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(249, 115, 22, 0.2)' },
              { offset: 1, color: 'rgba(249, 115, 22, 0)' }
            ]
          }
        }
      },
      {
        name: '下载量',
        type: 'bar',
        barWidth: '40%',
        data: downloads,
        itemStyle: { color: '#0ea5e9', borderRadius: [4, 4, 0, 0] }
      },
      {
        name: 'Repo 拉取量',
        type: 'line',
        smooth: true,
        symbol: 'none',
        data: repoDownloads,
        itemStyle: { color: '#8b5cf6' }
      },
      {
        name: '下载流量',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        data: traffic,
        itemStyle: { color: '#10b981' }
      },
      {
        name: 'Repo 流量',
        type: 'line',
        smooth: true,
        symbol: 'none',
        yAxisIndex: 1,
        data: repoTraffic,
        itemStyle: { color: '#f59e0b' }
      }
    ]
  }
})

onMounted(async () => {
  try {
    const [statsRes] = await Promise.all([
      getStats()
    ])
    registerMap('world', worldMap)
    mapLoaded.value = true
    stats.value = statsRes.data

    document.title = `统计信息 - ${globalConfig.site.nameFull}`
    const desc = `查看${globalConfig.site.name}的访问统计、下载统计和地理分布数据`
    const metaDescription = document.querySelector('meta[name="description"]')
    const metaOgDescription = document.querySelector('meta[property="og:description"]')
    const metaTwitterDescription = document.querySelector('meta[property="twitter:description"]')
    if (metaDescription) metaDescription.setAttribute('content', desc)
    if (metaOgDescription) metaOgDescription.setAttribute('content', '统计信息 - ' + desc)
    if (metaTwitterDescription) metaTwitterDescription.setAttribute('content', '统计信息 - ' + desc)
  } catch (e) {
    console.error('Failed to load data', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">数据洞察</h1>
      <p class="mt-1 text-sm text-muted-foreground">站点访问、下载与地理分布统计概览。</p>
    </div>

    <div v-if="loading" class="space-y-6">
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-4">
        <Card v-for="i in 8" :key="i" class="shadow-sm">
          <CardHeader class="pb-2">
            <Skeleton class="h-4 w-20" />
          </CardHeader>
          <CardContent>
            <Skeleton class="h-8 w-16" />
            <Skeleton class="mt-1 h-3 w-28" />
          </CardContent>
        </Card>
      </div>
      <div class="grid gap-4 md:grid-cols-7">
        <Card class="col-span-7 lg:col-span-4 h-[420px] shadow-sm">
          <CardHeader><Skeleton class="h-5 w-32" /></CardHeader>
          <CardContent class="h-full"><Skeleton class="h-full w-full rounded" /></CardContent>
        </Card>
        <Card class="col-span-7 lg:col-span-3 h-[420px] shadow-sm">
          <CardHeader><Skeleton class="h-5 w-32" /></CardHeader>
          <CardContent class="h-full"><Skeleton class="h-full w-full rounded" /></CardContent>
        </Card>
      </div>
      <Card class="shadow-sm">
        <CardHeader><Skeleton class="h-5 w-40" /></CardHeader>
        <CardContent class="h-[350px]"><Skeleton class="h-full w-full rounded" /></CardContent>
      </Card>
    </div>

    <template v-else>
      <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-4">
        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">总访问量</CardTitle>
            <Eye class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="flex items-baseline gap-2">
              <span class="text-2xl font-bold">{{ stats.total_visits?.toLocaleString() || '-' }}</span>
              <span
                v-if="visitGrowth !== 0"
                class="inline-flex items-center gap-0.5 text-xs font-medium"
                :class="visitGrowth >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
              >
                <ArrowUp v-if="visitGrowth >= 0" class="h-3 w-3" />
                <ArrowDown v-else class="h-3 w-3" />
                {{ Math.abs(visitGrowth).toFixed(1) }}%
              </span>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">近 30 日 {{ stats.last_30_visits?.toLocaleString() || 0 }} 次访问</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">总下载量</CardTitle>
            <Download class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="flex items-baseline gap-2">
              <span class="text-2xl font-bold">{{ stats.total_downloads?.toLocaleString() || '-' }}</span>
              <span
                v-if="downloadGrowth !== 0"
                class="inline-flex items-center gap-0.5 text-xs font-medium"
                :class="downloadGrowth >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
              >
                <ArrowUp v-if="downloadGrowth >= 0" class="h-3 w-3" />
                <ArrowDown v-else class="h-3 w-3" />
                {{ Math.abs(downloadGrowth).toFixed(1) }}%
              </span>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">近 30 日 {{ stats.last_30_downloads?.toLocaleString() || 0 }} 次下载</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Repo 拉取量</CardTitle>
            <Download class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="flex items-baseline gap-2">
              <span class="text-2xl font-bold">{{ stats.total_repo_downloads?.toLocaleString() || '-' }}</span>
              <span
                v-if="repoDownloadGrowth !== 0"
                class="inline-flex items-center gap-0.5 text-xs font-medium"
                :class="repoDownloadGrowth >= 0 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
              >
                <ArrowUp v-if="repoDownloadGrowth >= 0" class="h-3 w-3" />
                <ArrowDown v-else class="h-3 w-3" />
                {{ Math.abs(repoDownloadGrowth).toFixed(1) }}%
              </span>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">近 30 日 {{ stats.last_30_repo_downloads?.toLocaleString() || 0 }} 次拉取</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">磁盘占用</CardTitle>
            <Server class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="flex items-baseline gap-2">
              <span class="text-2xl font-bold">{{ diskPercentage }}%</span>
            </div>
            <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-secondary">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="diskPercentage > 90 ? 'bg-red-500' : diskPercentage > 75 ? 'bg-amber-500' : 'bg-green-500'"
                :style="{ width: `${diskPercentage}%` }"
              ></div>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">已用 {{ formatBytes(stats.disk?.used) }} / 共 {{ formatBytes(stats.disk?.total) }}</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">总流量</CardTitle>
            <HardDrive class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ formatBytes(stats.total_traffic_bytes) }}</div>
            <p class="mt-1 text-xs text-muted-foreground">近 30 日 {{ formatBytes(stats.last_30_traffic_bytes) }} 下载流量</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">Repo 流量</CardTitle>
            <HardDrive class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ formatBytes(stats.total_repo_traffic_bytes) }}</div>
            <p class="mt-1 text-xs text-muted-foreground">近 30 日 {{ formatBytes(stats.last_30_repo_traffic_bytes) }} 拉取流量</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">运行天数</CardTitle>
            <Activity class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ stats.total_days || '-' }}</div>
            <p class="mt-1 text-xs text-muted-foreground">自系统上线以来</p>
          </CardContent>
        </Card>

        <Card class="shadow-sm transition-shadow hover:shadow-md">
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">覆盖地区</CardTitle>
            <Globe class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ stats.geo_distribution?.length || '-' }}</div>
            <p class="mt-1 text-xs text-muted-foreground">全球访问来源国家/地区</p>
          </CardContent>
        </Card>
      </div>

      <div class="grid gap-4 md:grid-cols-7">
        <Card class="col-span-7 lg:col-span-4 shadow-sm">
          <CardHeader>
            <CardTitle class="flex items-center gap-2 text-base">
              <MapPin class="h-4 w-4 text-primary" />
              全球访问分布
            </CardTitle>
            <CardDescription>实时监控全球范围内的用户访问来源</CardDescription>
          </CardHeader>
          <CardContent class="pl-2">
            <div class="h-[350px] w-full">
              <VChart class="chart" :option="mapOption" autoresize />
            </div>
          </CardContent>
        </Card>

        <Card class="col-span-7 lg:col-span-3 flex flex-col shadow-sm">
          <CardHeader>
            <CardTitle class="flex items-center gap-2 text-base">
              <TrendingUp class="h-4 w-4 text-green-500" />
              热门资源排行
            </CardTitle>
            <CardDescription>下载量最高的启动器版本</CardDescription>
          </CardHeader>
          <CardContent class="flex-1 overflow-hidden">
            <div class="h-[168px] space-y-3 overflow-y-auto pr-2 custom-scrollbar">
              <div v-for="(item, i) in stats.top_downloads" :key="`download-${i}`" class="flex items-center gap-3 rounded-lg border bg-card p-3 transition-colors hover:bg-accent/50">
                <div
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-bold"
                  :class="i < 3 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'"
                >
                  {{ i + 1 }}
                </div>
                <div class="min-w-0 flex-1 space-y-0.5">
                  <p class="truncate text-sm font-medium leading-none">{{ item.launcher }}</p>
                </div>
                <div class="shrink-0 text-sm font-bold tabular-nums">{{ item.count.toLocaleString() }}</div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card class="col-span-7 lg:col-span-3 flex flex-col shadow-sm">
          <CardHeader>
            <CardTitle class="flex items-center gap-2 text-base">
              <TrendingUp class="h-4 w-4 text-violet-500" />
              热门 Repo 排行
            </CardTitle>
            <CardDescription>拉取量最高的 Git 镜像仓库</CardDescription>
          </CardHeader>
          <CardContent class="flex-1 overflow-hidden">
            <div class="h-[168px] space-y-3 overflow-y-auto pr-2 custom-scrollbar">
              <div v-for="(item, i) in stats.top_repo_downloads" :key="`repo-${i}`" class="flex items-center gap-3 rounded-lg border bg-card p-3 transition-colors hover:bg-accent/50">
                <div
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-bold"
                  :class="i < 3 ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'"
                >
                  {{ i + 1 }}
                </div>
                <div class="min-w-0 flex-1 space-y-0.5">
                  <p class="truncate text-sm font-medium leading-none">{{ item.repo_name }}</p>
                </div>
                <div class="shrink-0 text-sm font-bold tabular-nums">{{ item.count.toLocaleString() }}</div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card class="shadow-sm">
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <BarChart3 class="h-4 w-4 text-orange-500" />
            最近 30 天趋势
          </CardTitle>
          <CardDescription>每日访问量与下载量的变化趋势</CardDescription>
        </CardHeader>
        <CardContent class="pl-2">
          <div class="h-[350px] w-full">
            <VChart class="chart" :option="trendOption" autoresize />
          </div>
        </CardContent>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.chart {
  height: 100%;
  width: 100%;
}
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: hsl(var(--muted));
  border-radius: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb:hover {
  background: hsl(var(--muted-foreground) / 0.5);
}
</style>
