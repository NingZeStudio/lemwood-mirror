<template>
  <v-card class="mb-6" variant="outlined">
    <v-card-title>API 文档</v-card-title>
    <v-card-text>
      <p class="mb-4">本镜像服务提供以下 API 供开发者使用：</p>

      <v-expansion-panels variant="accordion">
        <v-expansion-panel v-for="(endpoint, index) in endpoints" :key="index">
          <v-expansion-panel-title>
            <v-chip size="small" :color="endpoint.method === 'GET' ? 'primary' : 'warning'" class="mr-3">
              {{ endpoint.method }}
            </v-chip>
            {{ endpoint.path }}
            <v-spacer></v-spacer>
            <span class="text-caption text-medium-emphasis">{{ endpoint.title }}</span>
          </v-expansion-panel-title>
          <v-expansion-panel-text>
            {{ endpoint.desc }}
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </v-card-text>
  </v-card>
</template>

<script setup>
const endpoints = [
  { method: 'GET', path: '/api/status', title: '获取所有版本状态', desc: '返回所有启动器及其版本的详细信息。' },
  { method: 'GET', path: '/api/status/{launcher}', title: '获取指定启动器状态', desc: '返回指定启动器的所有版本信息。' },
  { method: 'GET', path: '/api/latest', title: '获取最新版本信息', desc: '返回所有启动器的最新版本号及相关信息。' },
  { method: 'GET', path: '/api/latest/{launcher}', title: '获取指定启动器最新版本', desc: '返回指定启动器的最新版本信息。' },
  { method: 'GET', path: '/api/stats', title: '获取统计数据', desc: '返回访问量、下载量、排行和地域分布等统计数据。' },
  { method: 'POST', path: '/api/scan', title: '触发手动扫描', desc: '触发服务器重新扫描上游仓库更新。' },
];
</script>
