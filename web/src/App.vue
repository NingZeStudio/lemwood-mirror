<script setup>
import DefaultLayout from '@/layouts/DefaultLayout.vue'
import CookiesConsent from '@/components/CookiesConsent.vue'
import AnnouncementDialog from '@/components/AnnouncementDialog.vue'
</script>

<template>
  <DefaultLayout>
    <router-view />
  </DefaultLayout>
  <CookiesConsent />
  <AnnouncementDialog />
</template>
