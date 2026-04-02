<template>
  <div class="min-h-screen bg-[#f4ead9] text-ink">
    <div class="chat-shell mx-auto flex min-h-screen max-w-[980px] flex-col gap-4 p-4" :class="chatClass">
      <PixelPanel panel-class="bg-white/90">
        <div class="flex items-start justify-between gap-4">
          <div>
            <div class="text-xs uppercase tracking-[0.3em] text-ink/60">desktop companion agent</div>
            <h1 class="mt-1 text-2xl font-black">CialloClaw 对话承接窗</h1>
            <p class="mt-2 max-w-2xl text-sm text-ink/70">先提示，再确认，后执行。所有展示均来自 Go sidecar 的 mock 能力层，窗口组件只承载呈现与交互。</p>
          </div>
          <StatusPill :label="store.home?.subtitle ?? '低打扰协助中'" :tone="pillTone" />
        </div>
      </PixelPanel>

      <div class="flex flex-1 flex-col gap-4">
        <PixelPanel panel-class="bg-white/90 flex flex-col gap-4 overflow-hidden">
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-black">今日承接场景</h2>
            <button class="pixel-btn pixel-btn-primary" @click="confirmAction">确认执行</button>
          </div>
          <div class="grid gap-3 overflow-y-auto pr-1">
            <article v-for="scenario in store.scenarios" :key="scenario.id" class="rounded-pixel border-2 border-ink bg-cream px-4 py-3">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="text-xs uppercase tracking-[0.25em] text-ink/50">{{ scenario.category }}</div>
                  <h3 class="mt-1 text-lg font-bold">{{ scenario.title }}</h3>
                </div>
                <StatusPill :label="riskLabel(scenario.risk)" :tone="riskTone(scenario.risk)" />
              </div>
              <p class="mt-2 text-sm leading-6 text-ink/80">{{ scenario.summary }}</p>
              <div class="mt-3 rounded-pixel border border-dashed border-ink/30 bg-white/70 p-3 text-sm leading-6 text-ink/70">
                {{ scenario.detail }}
              </div>
              <div class="mt-3 flex items-center justify-between gap-3">
                <span class="text-xs font-semibold uppercase tracking-[0.22em] text-ink/50">建议动作：{{ scenario.suggestedAction }}</span>
                <button class="pixel-btn">查看解释</button>
              </div>
            </article>
          </div>
        </PixelPanel>
      </div>

      <PixelPanel panel-class="bg-white/90">
        <div class="flex gap-3">
          <textarea class="pixel-input min-h-[88px] flex-1 resize-none" placeholder="输入新的指令、追问，或让它帮你承接当前桌面任务现场。"></textarea>
          <div class="flex w-[180px] flex-col gap-2">
            <button class="pixel-btn pixel-btn-primary">发送</button>
            <button class="pixel-btn">总结内容</button>
            <button class="pixel-btn">解释异常</button>
            <button class="pixel-btn">生成提醒</button>
          </div>
        </div>
      </PixelPanel>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import PixelPanel from '@/components/PixelPanel.vue'
import StatusPill from '@/components/StatusPill.vue'
import { useAppStore } from '@/stores/app'

const store = useAppStore()

onMounted(() => {
  void store.hydrateAll()
  void store.ensureMotionBridge()
})

const pillTone = computed(() => {
  switch (store.home?.status) {
    case 'helping':
      return 'mint'
    case 'nudging':
      return 'peach'
    case 'working':
      return 'berry'
    default:
      return 'sky'
  }
})

const chatClass = computed(() => ({
  'chat-shell--from-left': store.motion.edge === 'left' && store.motion.cue === 'chat-bloom',
  'chat-shell--from-right': store.motion.edge === 'right' && store.motion.cue === 'chat-bloom',
}))

function riskTone(risk: 'green' | 'yellow' | 'red') {
  if (risk === 'green') return 'mint'
  if (risk === 'yellow') return 'peach'
  return 'berry'
}

function riskLabel(risk: 'green' | 'yellow' | 'red') {
  if (risk === 'green') return '低风险'
  if (risk === 'yellow') return '需确认'
  return '高风险'
}

async function confirmAction() {
  await store.trigger('confirm', { scene: 'chat-main' })
}
</script>
