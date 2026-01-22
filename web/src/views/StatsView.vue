<script setup>
import { ref, onMounted, computed } from 'vue';
import { getStats } from '@/services/api';
import { useDark } from '@vueuse/core';
import { Eye, Download, Server, Globe, MapPin, TrendingUp, Activity, BarChart3 } from 'lucide-vue-next';
import Card from '@/components/ui/Card.vue';
import CardHeader from '@/components/ui/CardHeader.vue';
import CardTitle from '@/components/ui/CardTitle.vue';
import CardContent from '@/components/ui/CardContent.vue';
import CardDescription from '@/components/ui/CardDescription.vue';
import Skeleton from '@/components/ui/Skeleton.vue';

// ECharts imports
import { use, registerMap } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { BarChart, MapChart, LineChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent
} from 'echarts/components';
import VChart from 'vue-echarts';

use([
  CanvasRenderer,
  BarChart,
  MapChart,
  LineChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent
]);

const stats = ref({});
const loading = ref(true);
const mapLoaded = ref(false);
const isDark = useDark();

const mapCountryName = (name) => {
    if (!name) return '';
    const nameMap = {
        '中国': 'China', 'CN': 'China', 'China': 'China',
        '美国': 'United States', 'USA': 'United States', 'US': 'United States',
        '英国': 'United Kingdom', 'UK': 'United Kingdom',
        '俄罗斯': 'Russia', 'Russian Federation': 'Russia',
        '德国': 'Germany', '法国': 'France', '日本': 'Japan',
        '韩国': 'South Korea', '加拿大': 'Canada', '澳大利亚': 'Australia',
        '巴西': 'Brazil', '印度': 'India', '新加坡': 'Singapore'
    };
    return nameMap[name] || nameMap[name.trim()] || name; 
};

const mapOption = computed(() => {
    const textColor = isDark.value ? '#a1a1aa' : '#52525b';
    const borderColor = isDark.value ? '#27272a' : '#e4e4e7';
    const areaColor = isDark.value ? '#27272a' : '#f4f4f5';

    const data = (stats.value.geo_distribution || []).map(item => ({
        name: mapCountryName(item.country),
        value: item.count
    }));

    return {
        backgroundColor: 'transparent',
        tooltip: {
            trigger: 'item',
            backgroundColor: isDark.value ? '#18181b' : '#ffffff',
            borderColor: borderColor,
            textStyle: { color: isDark.value ? '#fafafa' : '#09090b' },
            formatter: params => `${params.name}: ${Number.isFinite(params.value) ? params.value : 0} (Visits)`
        },
        visualMap: {
            min: 0,
            max: data.length ? Math.max(...data.map(d => d.value)) : 100,
            left: 'left',
            top: 'bottom',
            text: ['High', 'Low'],
            calculable: true,
            inRange: { color: ['#e0f2f1', '#0f766e'] },
            textStyle: { color: textColor }
        },
        series: [
            {
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
            }
        ]
    };
});

const trendOption = computed(() => {
    const textColor = isDark.value ? '#a1a1aa' : '#52525b';
    const splitLineColor = isDark.value ? '#27272a' : '#e4e4e7';

    if (!stats.value.daily_stats) return {};

    const rawData = [...stats.value.daily_stats].reverse();
    const dates = rawData.map(d => d.date.slice(5));
    const visits = rawData.map(d => d.visit_count);
    const downloads = rawData.map(d => d.download_count);

    return {
        backgroundColor: 'transparent',
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'line' },
            backgroundColor: isDark.value ? '#18181b' : '#ffffff',
            borderColor: splitLineColor,
            textStyle: { color: isDark.value ? '#fafafa' : '#09090b' }
        },
        legend: {
            data: ['访问量', '下载量'],
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
        yAxis: {
            type: 'value',
            splitLine: { lineStyle: { type: 'dashed', color: splitLineColor } },
            axisLine: { show: false },
            axisLabel: { color: textColor }
        },
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
                        colorStops: [{ offset: 0, color: 'rgba(249, 115, 22, 0.2)' }, { offset: 1, color: 'rgba(249, 115, 22, 0)' }]
                    }
                }
            },
            {
                name: '下载量',
                type: 'bar',
                barWidth: '60%',
                data: downloads,
                itemStyle: { color: '#0ea5e9', borderRadius: [4, 4, 0, 0] }
            }
        ]
    };
});

onMounted(async () => {
    try {
        const [mapRes, statsRes] = await Promise.all([
             fetch('https://cdn.jsdelivr.net/npm/echarts@4.9.0/map/json/world.json').then(r => r.json()),
             getStats()
        ]);
        registerMap('world', mapRes);
        mapLoaded.value = true;
        stats.value = statsRes.data;
    } catch (e) {
        console.error('Failed to load data', e);
    } finally {
        loading.value = false;
    }
});
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
        <h2 class="text-3xl font-bold tracking-tight">数据洞察</h2>
    </div>

    <div v-if="loading" class="space-y-6">
       <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          <Card v-for="i in 4" :key="i">
            <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
              <Skeleton class="h-4 w-24" />
              <Skeleton class="h-4 w-4 rounded-full" />
            </CardHeader>
            <CardContent>
              <Skeleton class="h-8 w-16" />
              <Skeleton class="h-3 w-32 mt-1" />
            </CardContent>
          </Card>
       </div>
       <div class="grid gap-4 md:grid-cols-7">
          <Card class="col-span-4 h-[450px]">
             <CardHeader><Skeleton class="h-6 w-32" /></CardHeader>
             <CardContent class="h-full p-6"><Skeleton class="h-full w-full" /></CardContent>
          </Card>
          <Card class="col-span-3 h-[450px]">
             <CardHeader><Skeleton class="h-6 w-32" /></CardHeader>
             <CardContent class="h-full p-6"><Skeleton class="h-full w-full" /></CardContent>
          </Card>
       </div>
    </div>

    <div v-else class="space-y-6">
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">总访问量</CardTitle>
            <Eye class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ stats.total_visits?.toLocaleString() || '-' }}</div>
            <p class="text-xs text-muted-foreground">
              实时累计访问人次
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">总下载量</CardTitle>
            <Download class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ stats.total_downloads?.toLocaleString() || '-' }}</div>
            <p class="text-xs text-muted-foreground">
              所有版本累计下载
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">运行天数</CardTitle>
            <Activity class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ Object.keys(stats.daily_stats || {}).length || '-' }}</div>
            <p class="text-xs text-muted-foreground">
              自系统上线以来
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle class="text-sm font-medium">覆盖地区</CardTitle>
            <Globe class="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div class="text-2xl font-bold">{{ stats.geo_distribution?.length || '-' }}</div>
            <p class="text-xs text-muted-foreground">
              全球访问来源国家/地区
            </p>
          </CardContent>
        </Card>
      </div>

      <div class="grid gap-4 md:grid-cols-7">
        <Card class="col-span-7 lg:col-span-4">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
                 <MapPin class="h-4 w-4 text-primary" />
                 全球访问分布
            </CardTitle>
            <CardDescription>
                实时监控全球范围内的用户访问来源
            </CardDescription>
          </CardHeader>
          <CardContent class="pl-2">
             <div class="h-[350px] w-full">
                <VChart class="chart" :option="mapOption" autoresize />
            </div>
          </CardContent>
        </Card>
        
        <Card class="col-span-7 lg:col-span-3 flex flex-col">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
                <TrendingUp class="h-4 w-4 text-green-500" />
                热门资源排行
            </CardTitle>
            <CardDescription>
                下载量最高的启动器版本
            </CardDescription>
          </CardHeader>
          <CardContent class="flex-1 overflow-hidden">
              <div class="space-y-4 h-[350px] overflow-y-auto pr-2 custom-scrollbar">
                  <div v-for="(item, i) in stats.top_downloads" :key="i" class="flex items-center group">
                    <div 
                        class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold mr-4 transition-colors group-hover:bg-primary group-hover:text-primary-foreground"
                        :class="i < 3 ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'"
                    >
                        {{ i + 1 }}
                    </div>
                    <div class="space-y-1 flex-1 min-w-0">
                      <p class="text-sm font-medium leading-none truncate">{{ item.launcher }}</p>
                      <p class="text-xs text-muted-foreground truncate">{{ item.version }}</p>
                    </div>
                    <div class="font-bold text-sm tabular-nums">{{ item.count.toLocaleString() }}</div>
                  </div>
              </div>
          </CardContent>
        </Card>
      </div>

      <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
                <BarChart3 class="h-4 w-4 text-orange-500" />
                最近 30 天趋势
            </CardTitle>
             <CardDescription>
                每日访问量与下载量的变化趋势
            </CardDescription>
          </CardHeader>
          <CardContent class="pl-2">
             <div class="h-[350px] w-full">
               <VChart class="chart" :option="trendOption" autoresize />
             </div>
          </CardContent>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.chart {
  height: 100%;
  width: 100%;
}
/* Custom Scrollbar for top downloads */
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
