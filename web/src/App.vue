<template>
  <v-app>
    <v-app-bar :elevation="2" color="surface">
      <v-app-bar-title class="font-weight-bold">
        柠枺镜像
      </v-app-bar-title>

      <template v-slot:append>
        <v-btn
          variant="text"
          class="mr-2"
          @click="showApiDocs = !showApiDocs"
          :color="showApiDocs ? 'primary' : ''"
        >
          API 文档
        </v-btn>
        
        <v-btn
          variant="flat"
          color="primary"
          class="mr-4"
          @click="manualRefresh"
          :loading="refreshing"
        >
          手动刷新
        </v-btn>

        <v-btn icon @click="toggleTheme">
          <v-icon>{{ theme.global.current.value.dark ? 'mdi-weather-sunny' : 'mdi-weather-night' }}</v-icon>
        </v-btn>
      </template>
    </v-app-bar>

    <v-main class="bg-background">
      <v-container>
        <v-expand-transition>
          <div v-show="showApiDocs">
            <ApiDocs />
          </div>
        </v-expand-transition>
        
        <Announcements />
        
        <VersionList ref="versionListRef" />
        
        <Statistics ref="statsRef" />
        
        <FileBrowser />
      </v-container>
    </v-main>

    <v-footer class="text-center d-flex flex-column py-4 text-medium-emphasis">
      <div>Lemwood Mirror</div>
      <div class="text-caption mt-1">
        备案信息：<a href="https://beian.miit.gov.cn" target="_blank" class="text-decoration-none text-medium-emphasis">新ICP备2024015133号-5</a>
      </div>
    </v-footer>
  </v-app>
</template>

<script setup>
import { ref } from 'vue';
import { useTheme } from 'vuetify';
import { scan } from './services/api';

import ApiDocs from './components/ApiDocs.vue';
import Announcements from './components/Announcements.vue';
import VersionList from './components/VersionList.vue';
import Statistics from './components/Statistics.vue';
import FileBrowser from './components/FileBrowser.vue';

const theme = useTheme();
const showApiDocs = ref(false);
const refreshing = ref(false);

const versionListRef = ref(null);
const statsRef = ref(null);

const toggleTheme = () => {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark';
};

const manualRefresh = async () => {
  refreshing.value = true;
  try {
    await scan();
    // Refresh child components
    if (versionListRef.value) await versionListRef.value.refresh();
    if (statsRef.value) await statsRef.value.refresh();
  } catch (e) {
    console.error('Manual refresh failed:', e);
  } finally {
    refreshing.value = false;
  }
};
</script>

<style>
/* Global background override for Vuetify 3 app to ensure full coverage */
.bg-background {
  background-color: rgb(var(--v-theme-background));
}
</style>