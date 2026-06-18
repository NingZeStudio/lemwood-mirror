<script setup>
import { cva } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground hover:bg-primary/80',
        secondary: 'border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80',
        destructive: 'border-transparent bg-destructive text-destructive-foreground hover:bg-destructive/80',
        outline: 'text-foreground',
        success: 'border-transparent bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
        warning: 'border-transparent bg-warning/15 text-warning',
        info: 'border-transparent bg-blue-500/10 text-blue-600 dark:text-blue-400'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  }
)

defineProps({
  variant: { type: String, default: 'default' },
  class: { type: String, default: '' }
})
</script>

<template>
  <div :class="cn(badgeVariants({ variant }), $props.class)">
    <slot />
  </div>
</template>
