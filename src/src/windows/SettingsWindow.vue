<template>
  <div class="min-h-screen bg-[#efe3cc] p-4 text-ink">
    <div class="mx-auto flex max-w-[860px] flex-col gap-4">
      <PixelPanel panel-class="bg-white/90">
        <div class="flex items-center justify-between gap-4">
          <div>
            <div class="text-xs uppercase tracking-[0.28em] text-ink/50">control panel</div>
            <h1 class="mt-1 text-2xl font-black">CialloClaw 设置</h1>
            <p class="mt-2 text-sm text-ink/70">窗口策略、提醒风格、mock 模式与未来 Agent 接入边界都集中在这里。</p>
          </div>
          <button class="pixel-btn" @click="closeWindow">关闭</button>
        </div>
      </PixelPanel>

      <div class="grid gap-4">
        <PixelPanel v-for="section in store.settings" :key="section.id" panel-class="bg-white/90">
          <h2 class="text-lg font-black">{{ section.title }}</h2>
          <p class="mt-1 text-sm text-ink/60">{{ section.description }}</p>
          <div class="mt-4 grid gap-3">
            <div v-for="item in section.items" :key="item.id" class="rounded-pixel border-2 border-ink bg-[#fffaf1] p-3">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <div class="font-bold">{{ item.label }}</div>
                  <div class="mt-1 text-sm text-ink/65">{{ item.description }}</div>
                </div>
                <template v-if="item.type === 'toggle'">
                  <span class="rounded-full border-2 border-ink px-3 py-1 text-xs font-bold" :class="item.value ? 'bg-mint' : 'bg-[#efe6d7]'">
                    {{ item.value ? '开启' : '关闭' }}
                  </span>
                </template>
                <template v-else-if="item.type === 'choice'">
                  <span class="rounded-full border-2 border-ink bg-sky px-3 py-1 text-xs font-bold">{{ item.value }}</span>
                </template>
                <template v-else>
                  <span class="rounded-full border-2 border-ink bg-cream px-3 py-1 text-xs font-bold">说明</span>
                </template>
              </div>
            </div>
          </div>
        </PixelPanel>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import PixelPanel from '@/components/PixelPanel.vue'
import { useAppStore } from '@/stores/app'
import { invokeVoid } from '@/services/tauri'

const store = useAppStore()

onMounted(() => {
  void store.hydrateSettings()
})

async function closeWindow() {
  await invokeVoid('hide_settings')
}
</script>
