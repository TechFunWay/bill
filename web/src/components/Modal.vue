<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-end justify-center lg:items-center lg:p-4" @keydown.esc="handleClose">
        <button class="absolute inset-0 cursor-default bg-black/55" aria-label="关闭弹窗" @click="handleClose"></button>
        <div ref="dialogRef" role="dialog" aria-modal="true" :aria-labelledby="titleId" tabindex="-1" :class="panelClass" class="modal-panel relative w-full overflow-auto bg-surface px-4 pb-4 pt-2 text-foreground shadow-xl outline-none lg:p-6">
          <div class="mx-auto mb-2 h-1 w-9 rounded-full bg-muted-foreground/30 lg:hidden" aria-hidden="true"></div>
          <div class="mb-3 flex min-h-9 items-center justify-between lg:mb-4">
            <h3 :id="titleId" class="text-lg font-bold text-foreground">{{ title }}</h3>
            <div class="flex items-center gap-1">
              <button v-if="maximizable" @click="maximized = !maximized" class="icon-button" :aria-label="maximized ? '还原弹窗' : '最大化弹窗'">
                <svg v-if="!maximized" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 4h5v5m0-5-7 7M9 20H4v-5m0 5 7-7" /></svg>
                <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 4H4v5m11-5h5v5M9 20H4v-5m11 5h5v-5" /></svg>
              </button>
              <button v-if="closable !== false" @click="handleClose" class="icon-button -mr-2" aria-label="关闭">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
          </div>
          <div>
            <slot />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  closable?: boolean
  size?: 'default' | 'wide' | 'xl'
  maximizable?: boolean
}>(), { closable: true, size: 'default', maximizable: false })

const titleId = computed(() => `modal-title-${String(props.title || 'dialog').replace(/\s+/g, '-')}`)
const maximized = ref(false)
// Sizing lives in a single computed so the maximized state can cleanly
// replace the width/height/rounded classes without fighting CSS specificity.
const panelClass = computed(() => {
  if (maximized.value) return ['max-w-none', 'max-h-none', 'h-full', 'rounded-none']
  const width = props.size === 'default' ? 'max-w-lg' : props.size === 'xl' ? 'max-w-7xl' : 'max-w-6xl'
  return [width, 'max-h-[92dvh]', 'rounded-t-[1.5rem]', 'lg:max-h-[90vh]', 'lg:rounded-2xl']
})
const dialogRef = ref<HTMLElement | null>(null)
watch(() => props.modelValue, async value => {
  if (value) {
    maximized.value = false
    await nextTick()
    dialogRef.value?.focus()
  }
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function handleClose() {
  if (props.closable !== false) {
    emit('update:modelValue', false)
  }
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-active .modal-panel {
  transition: transform 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
.modal-leave-active .modal-panel {
  transition: transform 0.15s ease-in;
}
.modal-enter-from .modal-panel,
.modal-leave-to .modal-panel {
  transform: translateY(100%);
}

.modal-panel {
  padding-bottom: max(1.25rem, env(safe-area-inset-bottom));
}

@media (min-width: 1024px) {
  .modal-enter-from .modal-panel,
  .modal-leave-to .modal-panel {
    transform: scale(0.96);
  }
}
</style>
