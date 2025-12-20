<template>
  <div class="mb-6">
    <h2 class="text-h5 mb-4">文件浏览</h2>
    
    <v-card>
      <v-card-text>
        <div class="d-flex gap-2 align-center mb-4">
          <v-text-field
            v-model="path"
            label="相对路径"
            placeholder="例如：."
            variant="outlined"
            density="compact"
            hide-details
            @keyup.enter="loadFiles"
          ></v-text-field>
          <v-btn color="primary" @click="loadFiles" :loading="loading">
            浏览
          </v-btn>
        </div>

        <v-card variant="tonal" class="pa-0">
          <v-code class="d-block pa-4" style="white-space: pre-wrap; max-height: 500px; overflow: auto; background-color: rgba(0,0,0,0.05);">{{ fileContent }}</v-code>
        </v-card>
      </v-card-text>
    </v-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { getFiles } from '../services/api';

const path = ref('.');
const fileContent = ref('');
const loading = ref(false);

const loadFiles = async () => {
    loading.value = true;
    try {
        const res = await getFiles(path.value);
        fileContent.value = JSON.stringify(res.data, null, 2);
    } catch (e) {
        fileContent.value = '加载文件列表失败。';
        console.error(e);
    } finally {
        loading.value = false;
    }
};

onMounted(() => {
    loadFiles();
});
</script>
