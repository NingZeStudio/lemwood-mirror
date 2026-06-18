<script setup>
import { ExternalLink, Link2 } from 'lucide-vue-next'
import Badge from '@/components/ui/Badge.vue'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardDescription from '@/components/ui/CardDescription.vue'
import CardContent from '@/components/ui/CardContent.vue'
import { friendLinksConfig } from '@/lib/friendLinksConfig'
</script>

<template>
  <Card v-if="friendLinksConfig.enabled && friendLinksConfig.links?.length" class="md:col-span-2">
    <CardHeader class="p-4 md:p-6">
      <CardTitle class="flex items-center gap-2">
        <Link2 class="h-5 w-5 text-primary" />
        {{ friendLinksConfig.title }}
      </CardTitle>
      <CardDescription v-if="friendLinksConfig.description">
        {{ friendLinksConfig.description }}
      </CardDescription>
    </CardHeader>

    <CardContent class="px-4 pb-4 md:px-6 md:pb-6">
      <div class="grid gap-3 sm:grid-cols-2">
        <a
          v-for="link in friendLinksConfig.links"
          :key="link.url"
          :href="link.url"
          target="_blank"
          rel="noopener noreferrer"
          class="group rounded-lg border bg-background p-4 transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 space-y-2">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="font-semibold text-foreground">{{ link.name }}</h3>
                <Badge v-if="link.tag" variant="secondary">{{ link.tag }}</Badge>
              </div>
              <p v-if="link.description" class="text-sm leading-relaxed text-muted-foreground">
                {{ link.description }}
              </p>
            </div>
            <ExternalLink class="mt-1 h-4 w-4 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
          </div>
        </a>
      </div>
    </CardContent>
  </Card>
</template>
