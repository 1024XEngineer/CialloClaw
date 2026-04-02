<template>
  <div class="orb-window">
    <div class="orb-entity-wrap">
      <div
        class="orb-entity drag-region relative"
        :class="shellClass"
        data-tauri-drag-region
        role="button"
        aria-label="CialloClaw orb"
        @click="openChat"
        @contextmenu.prevent.stop="openMenu"
      >
        <span class="orb-core absolute inset-[4px] rounded-full"></span>
        <span class="orb-pixel-ring absolute inset-[9px] rounded-full"></span>
        <span class="orb-pixel-grid absolute inset-[14px] rounded-full"></span>

        <span class="orb-face relative z-10" data-tauri-drag-region>
          <span class="orb-eye orb-eye-left"></span>
          <span class="orb-eye orb-eye-right"></span>
          <span class="orb-mouth"></span>
        </span>

        <span class="orb-status-dot absolute right-[8px] top-[8px] z-10 h-3 w-3 rounded-full border border-ink/80" :class="statusDotClass"></span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { getCurrentWebviewWindow } from '@tauri-apps/api/webviewWindow'
import { useAppStore } from '@/stores/app'
import { invokeVoid } from '@/services/tauri'

const store = useAppStore()
const orbWindow = getCurrentWebviewWindow()

onMounted(async () => {
  void store.hydrateHome()
  void store.ensureMotionBridge()

  try {
    await orbWindow.setBackgroundColor([0, 0, 0, 0])
    await orbWindow.setShadow(false)
  } catch {
    // ignore in browser preview
  }
})

const statusDotClass = computed(() => {
  switch (store.home?.status) {
    case 'helping':
      return 'bg-mint'
    case 'nudging':
      return 'bg-peach'
    case 'working':
      return 'bg-berry'
    default:
      return 'bg-sky'
  }
})

const shellClass = computed(() => ({
  'orb-entity--snap': store.motion.cue === 'snap',
  'orb-entity--chat': store.motion.cue === 'chat-bloom',
  'orb-entity--nudge': store.motion.cue === 'nudge-bloom',
}))

async function openChat() {
  await invokeVoid('toggle_chat')
}

async function openMenu() {
  await invokeVoid('show_orb_menu')
}
</script>
