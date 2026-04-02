<template>
  <div class="min-h-screen bg-transparent p-3">
    <PixelPanel panel-class="nudge-shell bg-white/95 backdrop-blur-sm" :class="nudgeClass">
      <div class="flex items-start gap-3">
        <div class="mt-1 flex h-10 w-10 items-center justify-center rounded-full border-2 border-ink bg-mint text-lg">✦</div>
        <div class="flex-1">
          <div class="text-xs uppercase tracking-[0.25em] text-ink/50">低打扰提示</div>
          <h1 class="mt-1 text-base font-black">{{ current?.title ?? '新的帮助机会' }}</h1>
          <p class="mt-2 text-sm leading-6 text-ink/75">{{ current?.summary ?? '可以查看当前桌面任务的承接建议。' }}</p>
          <div class="mt-3 flex gap-2">
            <button class="pixel-btn pixel-btn-primary" @click="view">查看</button>
            <button class="pixel-btn" @click="later">稍后</button>
            <button class="pixel-btn" @click="dismiss">忽略</button>
          </div>
        </div>
      </div>
    </PixelPanel>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import PixelPanel from '@/components/PixelPanel.vue'
import { useAppStore } from '@/stores/app'
import { invokeVoid } from '@/services/tauri'

const store = useAppStore()

onMounted(() => {
  void store.hydrateNudges()
  void store.ensureMotionBridge()
})

const current = computed(() => store.nudges[0])
const nudgeClass = computed(() => ({
  'nudge-shell--from-left': store.motion.edge === 'left' && store.motion.cue === 'nudge-bloom',
  'nudge-shell--from-right': store.motion.edge === 'right' && store.motion.cue === 'nudge-bloom',
}))

async function view() {
  await invokeVoid('focus_chat')
  await invokeVoid('hide_nudge')
}

async function later() {
  await invokeVoid('hide_nudge')
}

async function dismiss() {
  await invokeVoid('hide_nudge')
}
</script>
