<script setup>
import { DialogClose } from 'radix-vue'

const props = defineProps({
  asChild: { type: Boolean, default: false },
  as: { type: String, default: 'button' },
})
</script>

<template>
  <DialogClose v-bind="props">
    <slot />
  </DialogClose>
</template>
