<template>
  <div class="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
    <div class="hidden min-w-0 lg:block">
      <h1 class="font-display text-xl font-extrabold tracking-[-0.02em] text-foreground">{{ title }}</h1>
      <p v-if="description" class="mt-0.5 max-w-2xl text-[13px] leading-snug text-muted-foreground">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex w-full flex-wrap items-center gap-1.5 lg:w-auto lg:shrink-0 [&>*]:min-h-9 [&>*]:flex-1 lg:[&>*]:flex-none">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string
    description?: string
  }>(),
  { description: '' },
)
</script>
