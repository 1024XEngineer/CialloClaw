<template>
  <span class="inline-flex items-center gap-2 rounded-full border-2 border-ink px-3 py-1 text-xs font-semibold" :class="toneClass">
    <span class="h-2.5 w-2.5 rounded-full border border-ink bg-current opacity-80"></span>
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ label: string; tone?: 'sky' | 'mint' | 'peach' | 'berry' }>()

const toneClass = computed(() => {
  switch (props.tone) {
    case 'mint':
      return 'bg-mint text-ink'
    case 'peach':
      return 'bg-peach text-ink'
    case 'berry':
      return 'bg-berry text-ink'
    default:
      return 'bg-sky text-ink'
  }
})
</script>
