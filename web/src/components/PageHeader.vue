<template>
  <div class="flex flex-wrap items-start justify-between gap-3">
    <div class="hidden min-w-0 lg:block">
      <p class="font-mono text-[10px] font-semibold uppercase tracking-[0.18em] text-brand-700 dark:text-brand-300">Bill Ledger</p>
      <h1 class="mt-1 font-display text-2xl font-extrabold tracking-[-0.03em] text-foreground sm:text-3xl">{{ title }}</h1>
      <p v-if="description" class="mt-2 max-w-2xl text-sm leading-relaxed text-muted-foreground">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex w-full flex-wrap items-center gap-2 lg:w-auto lg:shrink-0 [&>*]:min-h-11 [&>*]:flex-1 lg:[&>*]:flex-none">
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
