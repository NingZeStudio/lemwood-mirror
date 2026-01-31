<script setup>
import { ExternalLink, Github, Mail, Server, Code, Layers, Heart, Coffee } from 'lucide-vue-next'
import Card from '@/components/ui/Card.vue'
import CardHeader from '@/components/ui/CardHeader.vue'
import CardTitle from '@/components/ui/CardTitle.vue'
import CardContent from '@/components/ui/CardContent.vue'
import Button from '@/components/ui/Button.vue'
</script>

<template>
  <div class="container max-w-4xl px-4 py-6 md:py-10 space-y-6 md:space-y-8">
    <div class="space-y-2 text-center md:text-left">
      <h1 class="text-3xl md:text-4xl font-extrabold tracking-tight">关于本项目</h1>
      <p class="text-muted-foreground text-base md:text-lg">
        探索 Lemwood Mirror 的幕后故事、技术底座与那些支持我们的朋友。
      </p>
    </div>

    <!-- Angel Investor / Sponsor Section -->
    <Card class="bg-gradient-to-r from-amber-500/10 via-background to-amber-500/5 backdrop-blur-md border-amber-500/20 relative overflow-hidden group">
      <div class="absolute -right-8 -top-8 opacity-10 group-hover:opacity-20 transition-opacity">
          <Heart class="h-40 w-40 text-amber-500 rotate-12" />
      </div>
      <CardHeader>
        <CardTitle class="flex items-center gap-2 text-amber-600 dark:text-amber-400">
            <Coffee class="h-5 w-5" />
            天使投资人 & 赞助商
        </CardTitle>
      </CardHeader>
      <CardContent class="relative z-10">
          <div class="flex flex-col md:flex-row items-center gap-6">
              <div class="flex-1 space-y-4 text-center md:text-left">
                  <h3 class="text-2xl font-bold text-foreground">Ksgf452 <span class="text-sm font-normal text-muted-foreground ml-2">(大好人)</span></h3>
                  <p class="text-muted-foreground leading-relaxed">
                      赞助了 <span class="font-bold text-amber-600 dark:text-amber-500">40 元</span> 用于服务器续费。Ksgf452 是我们的天使投资人，梦泽和柠枺的许多项目都得到了他的慷慨赞助。
                  </p>
                  <div class="flex flex-wrap justify-center md:justify-start gap-4 text-sm text-muted-foreground">
                      <span class="flex items-center gap-1.5"><Mail class="h-4 w-4" /> ksgf452@163.com</span>
                      <span class="flex items-center gap-1.5 border-l pl-4 border-white/10">QQ: 2497302640</span>
                  </div>
              </div>
          </div>
      </CardContent>
    </Card>

    <div class="grid gap-4 md:gap-6 md:grid-cols-2">
        <Card class="bg-background/50 backdrop-blur-sm border-white/10 md:col-span-2">
          <CardHeader class="p-4 md:p-6">
            <CardTitle class="flex items-center gap-2">
                <Layers class="h-5 w-5 text-primary" />
                项目简介
            </CardTitle>
          </CardHeader>
          <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4 text-muted-foreground leading-relaxed">
            <p>
              Lemwood Mirror 是一个自托管的开源镜像服务，专为 Minecraft 社区打造。它致力于提供稳定、高速的启动器及相关工具下载体验。
              通过全自动化的 GitHub Release 追踪系统，我们确保用户能第一时间获取到最新的软件版本，摆脱网络环境的束缚。
            </p>
          </CardContent>
        </Card>

        <Card class="bg-background/50 backdrop-blur-sm border-white/10">
            <CardHeader class="p-4 md:p-6">
                <CardTitle class="flex items-center gap-2">
                    <Server class="h-5 w-5 text-blue-500" />
                    基础设施 & 后端
                </CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4">
                 <div class="flex items-start gap-4">
                     <div class="space-y-1">
                         <h4 class="font-semibold text-foreground">服务器 & 域名备案</h4>
                         <p class="text-sm text-muted-foreground">由 <span class="text-foreground font-medium">柠枺</span> 提供全方位支持。</p>
                         <div class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 text-xs text-muted-foreground mt-1">
                             <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> 3436464181@qq.com</span>
                             <span class="hidden sm:inline border-l h-3"></span>
                             <span>QQ: 3436464181</span>
                         </div>
                     </div>
                 </div>
                 <div class="space-y-1 pt-2 border-t border-dashed border-white/10">
                     <h4 class="font-semibold text-foreground">技术栈</h4>
                     <div class="flex flex-wrap gap-2 text-xs">
                         <span class="px-2 py-1 rounded-md bg-blue-500/10 text-blue-500 font-medium">Golang</span>
                         <span class="px-2 py-1 rounded-md bg-cyan-500/10 text-cyan-500 font-medium">Docker</span>
                     </div>
                 </div>
            </CardContent>
        </Card>

        <Card class="bg-background/50 backdrop-blur-sm border-white/10">
            <CardHeader class="p-4 md:p-6">
                <CardTitle class="flex items-center gap-2">
                    <Code class="h-5 w-5 text-green-500" />
                    前端开发 & 设计
                </CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-4">
                 <div class="flex items-start gap-4">
                     <div class="space-y-1">
                         <h4 class="font-semibold text-foreground">核心开发</h4>
                         <p class="text-sm text-muted-foreground">由 <span class="text-foreground font-medium">梦泽</span> 设计与编码。</p>
                         <div class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 text-xs text-muted-foreground mt-1">
                             <span class="flex items-center gap-1"><Mail class="h-3 w-3" /> mengze2@foxmail.com</span>
                             <span class="hidden sm:inline border-l h-3"></span>
                             <span>QQ: 417158478</span>
                         </div>
                     </div>
                 </div>
                 <div class="space-y-1 pt-2 border-t border-dashed border-white/10">
                     <h4 class="font-semibold text-foreground">技术栈</h4>
                     <div class="flex flex-wrap gap-2 text-xs">
                         <span class="px-2 py-1 rounded-md bg-green-500/10 text-green-500 font-medium">Vue 3</span>
                         <span class="px-2 py-1 rounded-md bg-purple-500/10 text-purple-500 font-medium">Vite</span>
                         <span class="px-2 py-1 rounded-md bg-slate-500/10 text-slate-500 font-medium">Shadcn/Vue</span>
                     </div>
                 </div>
            </CardContent>
        </Card>
    </div>

    <div class="grid gap-4 md:gap-6 md:grid-cols-2">
         <Card class="bg-gradient-to-br from-background/50 to-primary/5 backdrop-blur-sm border-primary/20 hover:border-primary/40 transition-colors">
            <CardHeader class="p-4 md:p-6">
                <CardTitle>同根项目：NingZeLogs</CardTitle>
            </CardHeader>
            <CardContent class="px-4 pb-4 md:px-6 md:pb-6 space-y-3">
                <p class="text-sm text-muted-foreground">
                    一个更好用的 Minecraft 日志分析平台。类似于 mclo.gs，但提供更丰富的功能和更友好的用户体验。
                </p>
                <div class="pt-2 text-center md:text-left">
                    <Button variant="outline" size="sm" as="a" href="https://mclogs.lemwood.icu" target="_blank">
                        访问 mclogs.lemwood.icu <ExternalLink class="ml-2 h-3 w-3" />
                    </Button>
                </div>
            </CardContent>
        </Card>

        <Card class="bg-background/50 backdrop-blur-sm border-white/10 flex flex-col justify-center items-center text-center p-4 md:p-6">
             <Github class="h-8 md:h-10 w-8 md:w-10 mb-2 md:mb-4 text-foreground/80" />
             <h3 class="font-semibold mb-1 md:mb-2 text-sm md:text-base">开源贡献</h3>
             <p class="text-xs md:text-sm text-muted-foreground mb-3 md:mb-4">
                 本项目代码完全开源。欢迎 Star、Fork 或提交 Pull Request。
             </p>
             <Button size="sm" as="a" href="https://github.com/JanePHPDev/lemwood_mirror_NewWeb" target="_blank">
                 前往 GitHub 仓库
             </Button>
        </Card>
    </div>
  </div>
</template>
