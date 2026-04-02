<template>
  <section class="rounded-pixel border-2 border-ink bg-cream/95 p-4 shadow-pixel" :class="panelClass">
    <slot />
  </section>
</template>

<script setup lang="ts">
defineProps<{ panelClass?: string }>()
</script>
