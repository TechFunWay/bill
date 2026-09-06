<template>
  <div>
    <button
      ref="toggleRef"
      type="button"
      class="flex min-h-12 w-full items-center justify-between gap-3 rounded-xl px-1 text-left transition-colors duration-200 active:bg-muted/55 lg:hidden"
      :aria-expanded="modelValue"
      :aria-controls="panelId"
      @click="emit('update:modelValue', !modelValue)"
    >
      <span class="min-w-0">
        <strong class="block text-sm font-bold text-foreground">{{ title }}</strong>
        <small class="mt-0.5 block truncate text-xs text-muted-foreground">{{ summary }}</small>
      </span>
      <span class="flex shrink-0 items-center gap-2">
        <span v-if="activeCount > 0" class="flex h-6 min-w-6 items-center justify-center rounded-full bg-brand-100 px-1.5 text-[11px] font-bold text-brand-700 dark:bg-brand-800 dark:text-accent">{{ activeCount }}</span>
        <svg :class="modelValue ? 'rotate-180' : ''" class="h-5 w-5 text-muted-foreground transition-transform duration-200" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m6 9 6 6 6-6" />
        </svg>
      </span>
    </button>

    <div
      :id="panelId"
      :class="modelValue ? 'grid-rows-[1fr] opacity-100' : 'invisible pointer-events-none grid-rows-[0fr] opacity-0 lg:visible lg:pointer-events-auto lg:grid-rows-[1fr] lg:opacity-100'"
      class="grid transition-[grid-template-rows,opacity] duration-200 ease-out"
    >
      <div class="min-h-0 overflow-hidden">
        <div class="pt-3 lg:pt-0">
          <slot />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  panelId: string
  title: string
  summary: string
  activeCount?: number
}>(), { activeCount: 0 })

const toggleRef = ref<HTMLButtonElement | null>(null)

watch(() => props.modelValue, async (expanded, wasExpanded) => {
  if (!expanded && wasExpanded && window.matchMedia('(max-width: 1023px)').matches) {
    await nextTick()
    toggleRef.value?.focus({ preventScroll: true })
  }
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>
