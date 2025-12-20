<template>
  <div class="mb-6">
    <h2 class="text-h5 mb-4">数据统计</h2>

    <div v-if="loading" class="d-flex justify-center pa-4">
      <v-progress-circular indeterminate color="primary"></v-progress-circular>
    </div>

    <div v-else>
      <!-- Overview Cards -->
      <v-row class="mb-4">
        <v-col cols="12" sm="6">
          <v-card class="text-center py-4">
            <div class="text-subtitle-1 text-medium-emphasis">总访问量</div>
            <div class="text-h3 font-weight-bold">{{ stats.total_visits?.toLocaleString() || '-' }}</div>
          </v-card>
        </v-col>
        <v-col cols="12" sm="6">
          <v-card class="text-center py-4">
            <div class="text-subtitle-1 text-medium-emphasis">总下载量</div>
            <div class="text-h3 font-weight-bold">{{ stats.total_downloads?.toLocaleString() || '-' }}</div>
          </v-card>
        </v-col>
      </v-row>

      <v-row>
        <!-- Chart -->
        <v-col cols="12" md="12" lg="4">
          <v-card class="h-100">
            <v-card-title>最近 30 天趋势</v-card-title>
            <v-card-text>
              <Bar v-if="chartData" :data="chartData" :options="chartOptions" style="height: 300px" />
              <div v-else class="text-center py-10">暂无数据</div>
            </v-card-text>
          </v-card>
        </v-col>

        <!-- Top Downloads -->
        <v-col cols="12" md="6" lg="4">
          <v-card class="h-100">
            <v-card-title>热门下载 (Top 10)</v-card-title>
            <v-list density="compact">
              <v-list-item v-for="(item, i) in stats.top_downloads" :key="i">
                <v-list-item-title>{{ item.launcher }} {{ item.version }}</v-list-item-title>
                <template v-slot:append>
                  <span class="font-weight-bold">{{ item.count?.toLocaleString() }}</span>
                </template>
              </v-list-item>
              <v-list-item v-if="!stats.top_downloads?.length">
                <v-list-item-title class="text-center text-medium-emphasis">暂无数据</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-card>
        </v-col>

        <!-- Geo Dist -->
        <v-col cols="12" md="6" lg="4">
          <v-card class="h-100">
            <v-card-title>访客地域分布</v-card-title>
             <v-list density="compact">
              <v-list-item v-for="(item, i) in stats.geo_distribution" :key="i">
                <v-list-item-title>{{ item.country || '未知' }}</v-list-item-title>
                <template v-slot:append>
                  <span class="font-weight-bold">{{ item.count?.toLocaleString() }}</span>
                </template>
              </v-list-item>
               <v-list-item v-if="!stats.geo_distribution?.length">
                <v-list-item-title class="text-center text-medium-emphasis">暂无数据</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-card>
        </v-col>
      </v-row>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue';
import { getStats } from '../services/api';
import { Bar } from 'vue-chartjs';
import { Chart as ChartJS, Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale } from 'chart.js';

ChartJS.register(Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale);

const stats = ref({});
const loading = ref(true);

const chartData = computed(() => {
    if (!stats.value.daily_stats || !stats.value.daily_stats.length) return null;
    
    const rawData = [...stats.value.daily_stats].reverse();
    const labels = rawData.map(d => d.date.slice(5)); // MM-DD
    const visits = rawData.map(d => d.visit_count);
    const downloads = rawData.map(d => d.download_count);

    return {
        labels,
        datasets: [
            {
                label: '访问量',
                backgroundColor: 'rgba(136, 136, 136, 0.5)',
                data: visits
            },
            {
                label: '下载量',
                backgroundColor: '#2196F3', // Primary color
                data: downloads
            }
        ]
    };
});

const chartOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
        legend: {
            position: 'top',
        }
    },
    scales: {
        y: {
            beginAtZero: true
        }
    }
};

const loadData = async () => {
    loading.value = true;
    try {
        const res = await getStats();
        stats.value = res.data;
    } catch (e) {
        console.error('Failed to load stats:', e);
    } finally {
        loading.value = false;
    }
};

onMounted(() => {
    loadData();
});

defineExpose({ refresh: loadData });
</script>
