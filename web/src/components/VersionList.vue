<template>
  <div class="mb-6">
    <h2 class="text-h5 mb-4">版本信息</h2>
    
    <div v-if="loading" class="d-flex justify-center pa-4">
      <v-progress-circular indeterminate color="primary"></v-progress-circular>
    </div>

    <div v-else-if="!Object.keys(launchers).length" class="text-center text-medium-emphasis">
      暂无数据
    </div>

    <v-expansion-panels v-else multiple v-model="panel">
      <v-expansion-panel v-for="(versions, name) in launchers" :key="name" :value="name">
        <v-expansion-panel-title>
          <div class="d-flex align-center w-100">
            <span class="text-h6 mr-3">{{ name }}</span>
            <v-chip v-if="latestVersions[name]" color="success" size="x-small" label>
              最新: {{ latestVersions[name] }}
            </v-chip>
          </div>
        </v-expansion-panel-title>
        
        <v-expansion-panel-text>
          <v-row>
            <v-col v-for="v in versions" :key="v.tag_name || v.name" cols="12" md="6" lg="4">
              <v-card :variant="isLatest(name, v) ? 'elevated' : 'outlined'" :color="isLatest(name, v) ? 'surface-variant' : ''" class="h-100">
                <v-card-item>
                  <template v-slot:title>
                    <div class="d-flex align-center">
                       {{ v.tag_name || v.name }}
                       <v-chip v-if="isLatest(name, v)" color="success" size="x-small" class="ml-2">Latest</v-chip>
                    </div>
                  </template>
                  <template v-slot:subtitle>
                    发布于：{{ formatDate(v.published_at) }}
                  </template>
                </v-card-item>

                <v-card-text>
                  <v-list density="compact" bg-color="transparent">
                    <v-list-item v-for="asset in v.assets" :key="asset.name" class="px-0">
                      <template v-slot:prepend>
                        <v-icon icon="mdi-package-variant-closed" size="small" class="mr-2"></v-icon>
                      </template>
                      
                      <v-list-item-title>
                         <a :href="getDownloadUrl(name, v, asset)" :download="asset.name" class="text-decoration-none text-primary font-weight-medium">
                           {{ asset.name }}
                         </a>
                      </v-list-item-title>
                      
                      <template v-slot:append>
                        <v-btn icon="mdi-content-copy" size="x-small" variant="text" @click="copyLink(getDownloadUrl(name, v, asset))" :color="copied === getDownloadUrl(name, v, asset) ? 'success' : ''">
                        </v-btn>
                      </template>
                    </v-list-item>
                  </v-list>
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
    
    <v-snackbar v-model="snackbar" :timeout="2000" color="success">
      链接已复制到剪贴板
    </v-snackbar>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { getStatus, getLatest } from '../services/api';

const launchers = ref({});
const latestVersions = ref({});
const loading = ref(true);
const panel = ref([]);
const snackbar = ref(false);
const copied = ref('');

const loadData = async () => {
  loading.value = true;
  try {
    const [statusRes, latestRes] = await Promise.all([getStatus(), getLatest()]);
    
    // Sort versions
    const data = statusRes.data;
    for (const key in data) {
        data[key].sort((a, b) => String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name)));
    }
    
    launchers.value = data;
    latestVersions.value = latestRes.data;
    
    // Open all panels by default
    panel.value = Object.keys(data);
  } catch (e) {
    console.error('Failed to load status:', e);
  } finally {
    loading.value = false;
  }
};

const isLatest = (launcherName, version) => {
    return latestVersions.value[launcherName] === (version.tag_name || version.name);
};

const formatDate = (dateStr) => {
    if (!dateStr) return '未知';
    return new Date(dateStr).toLocaleString();
};

const getDownloadUrl = (launcher, version, asset) => {
    if (asset.url && (asset.url.startsWith('http://') || asset.url.startsWith('https://'))) {
        return asset.url;
    }
    return `/download/${launcher}/${version.tag_name || version.name}/${asset.name}`;
};

const copyLink = (url) => {
    const fullUrl = url.startsWith('http') ? url : window.location.origin + url;
    navigator.clipboard.writeText(fullUrl).then(() => {
        copied.value = url;
        snackbar.value = true;
        setTimeout(() => {
            copied.value = '';
        }, 2000);
    });
};

onMounted(() => {
    loadData();
});

defineExpose({ refresh: loadData });
</script>
