<template>
  <div ref="root" class="relative">
    <div class="relative">
      <BankLogo v-if="selected" :bank="selected" size="sm" class="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2" />
      <svg v-else class="pointer-events-none absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><circle cx="11" cy="11" r="7" stroke-width="2"/><path d="m20 20-4-4" stroke-width="2" stroke-linecap="round"/></svg>
      <input
        ref="input"
        v-model="query"
        class="input-field pr-11"
        :class="selected ? 'pl-12' : 'pl-11'"
        role="combobox"
        autocomplete="off"
        :aria-expanded="open"
        aria-controls="bank-picker-options"
        aria-autocomplete="list"
        :placeholder="loading ? '银行列表加载中' : '输入银行名称或缩写搜索'"
        :disabled="loading"
        @focus="openPicker"
        @input="onInput"
        @keydown="onKeydown"
        @blur="closeLater"
      />
      <button v-if="query" type="button" class="absolute right-1 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-xl text-muted-foreground hover:bg-muted" aria-label="清空银行搜索" @mousedown.prevent @click="clear"><svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="m6 6 12 12M18 6 6 18" stroke-width="2" stroke-linecap="round"/></svg></button>
    </div>
    <div v-if="open" id="bank-picker-options" role="listbox" class="absolute z-40 mt-2 max-h-64 w-full overflow-y-auto overscroll-contain rounded-2xl border border-border bg-surface p-1.5 shadow-xl">
      <button
        v-for="(bank, index) in filtered"
        :key="bank.code"
        type="button"
        role="option"
        :aria-selected="bank.name === modelValue"
        :class="index === activeIndex ? 'bg-muted' : ''"
        class="flex min-h-12 w-full touch-manipulation items-center gap-3 rounded-xl px-3 text-left text-sm hover:bg-muted focus-visible:bg-muted"
        @mousedown.prevent
        @mouseenter="activeIndex = index"
        @click="select(bank)"
      >
        <BankLogo :bank="bank" size="sm" />
        <span class="min-w-0 flex-1 truncate font-semibold">{{ bank.name }}</span>
        <span class="font-mono text-xs text-muted-foreground">{{ bank.code }}</span>
      </button>
      <p v-if="!filtered.length" class="px-4 py-6 text-center text-sm text-muted-foreground">没有匹配的银行，可选择“其他银行”</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { BankOption } from '../../api/billing'
import BankLogo from './BankLogo.vue'

const props = defineProps<{ modelValue: string; banks: BankOption[]; loading?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const root = ref<HTMLElement | null>(null), input = ref<HTMLInputElement | null>(null)
const query = ref(''), open = ref(false), activeIndex = ref(0)
const selected = computed(() => props.banks.find(bank => bank.name === props.modelValue))
const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword || selected.value?.name === query.value) return props.banks
  return props.banks.filter(bank => `${bank.name} ${bank.code} ${bank.short}`.toLowerCase().includes(keyword))
})

function openPicker() { query.value = selected.value?.name || query.value; open.value = true; activeIndex.value = Math.max(0, filtered.value.findIndex(bank => bank.name === props.modelValue)) }
function closeLater() { window.setTimeout(() => { if (!root.value?.contains(document.activeElement)) { open.value = false; query.value = selected.value?.name || '' } }, 100) }
function onInput() { if (query.value !== selected.value?.name) emit('update:modelValue', ''); open.value = true; activeIndex.value = 0 }
function select(bank: BankOption) { emit('update:modelValue', bank.name); query.value = bank.name; open.value = false; input.value?.focus() }
function clear() { query.value = ''; emit('update:modelValue', ''); open.value = true; activeIndex.value = 0; input.value?.focus() }
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') { event.preventDefault(); open.value = true; activeIndex.value = Math.min(activeIndex.value + 1, filtered.value.length - 1) }
  else if (event.key === 'ArrowUp') { event.preventDefault(); activeIndex.value = Math.max(activeIndex.value - 1, 0) }
  else if (event.key === 'Enter' && open.value && filtered.value[activeIndex.value]) { event.preventDefault(); select(filtered.value[activeIndex.value]) }
  else if (event.key === 'Escape') { open.value = false; query.value = selected.value?.name || ''; input.value?.blur() }
}
watch(() => props.modelValue, value => { query.value = props.banks.find(bank => bank.name === value)?.name || value })
watch(() => props.banks, () => { query.value = selected.value?.name || props.modelValue }, { immediate: true })
</script>
