<template>
  <fieldset>
    <legend class="text-sm font-semibold">标签 <small class="font-normal text-muted-foreground">可多选，回车可新增</small></legend>
    <div class="mt-2 rounded-2xl border border-border bg-surface p-2 focus-within:ring-3 focus-within:ring-brand-500/25">
      <div v-if="selectedTags.length" class="mb-2 flex flex-wrap gap-2">
        <span v-for="tag in selectedTags" :key="tag.id" class="inline-flex min-h-11 items-center gap-1 rounded-full bg-brand-100 pl-3 pr-1 text-sm font-semibold text-brand-800 dark:bg-brand-800 dark:text-accent">
          {{ tag.name }}
          <button type="button" class="flex h-10 w-10 items-center justify-center rounded-full hover:bg-black/10" :aria-label="`移除标签 ${tag.name}`" @click="remove(tag.id)"><svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="m7 7 10 10M17 7 7 17" stroke-width="2" stroke-linecap="round"/></svg></button>
        </span>
      </div>
      <div class="flex min-h-11 items-center gap-2 px-2">
        <svg class="h-5 w-5 shrink-0 text-muted-foreground" viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true"><path d="M20 13 13 20 4 11V4h7l9 9Z" stroke-width="1.8" stroke-linejoin="round"/><circle cx="8.5" cy="8.5" r="1" fill="currentColor"/></svg>
        <input v-model="query" maxlength="50" class="min-w-0 flex-1 bg-transparent text-base outline-none placeholder:text-muted-foreground" autocomplete="off" placeholder="输入标签名称，按回车添加" @keydown.enter.prevent="commit" @keydown.backspace="removeLast" />
        <span v-if="creating" class="text-xs text-muted-foreground">添加中</span>
      </div>
    </div>
    <div v-if="suggestions.length" class="mt-2 flex flex-wrap gap-2" aria-label="可选标签">
      <button v-for="tag in suggestions" :key="tag.id" type="button" class="min-h-11 touch-manipulation rounded-full border border-border px-3 text-sm text-muted-foreground hover:border-brand-500 hover:text-foreground" @click="add(tag.id)">+ {{ tag.name }}</button>
    </div>
    <p v-if="error" role="alert" class="mt-2 text-sm text-rose-600">{{ error }}</p>
  </fieldset>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { billingApi, type PersonalTag } from '../../api/billing'
import { apiMessage } from '../../utils/billing'

const props = defineProps<{ modelValue: number[]; tags: PersonalTag[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]]; created: [tag: PersonalTag] }>()
const query = ref(''), creating = ref(false), error = ref('')
const selectedTags = computed(() => props.modelValue.map(id => props.tags.find(tag => tag.id === id)).filter((tag): tag is PersonalTag => Boolean(tag)))
const suggestions = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return props.tags.filter(tag => !props.modelValue.includes(tag.id) && (!keyword || tag.name.toLowerCase().includes(keyword))).slice(0, 10)
})

function add(id: number) { if (!props.modelValue.includes(id)) emit('update:modelValue', [...props.modelValue, id]); query.value = ''; error.value = '' }
function remove(id: number) { emit('update:modelValue', props.modelValue.filter(item => item !== id)) }
function removeLast(event: KeyboardEvent) { if (!query.value && props.modelValue.length) { event.preventDefault(); remove(props.modelValue[props.modelValue.length - 1]) } }
async function commit() {
  const name = query.value.trim()
  if (!name || creating.value) return
  const existing = props.tags.find(tag => tag.name.toLowerCase() === name.toLowerCase())
  if (existing) { add(existing.id); return }
  creating.value = true; error.value = ''
  try {
    const response = await billingApi.personalTagCreate({ name })
    const tag = response.data?.data as PersonalTag
    emit('created', tag)
    emit('update:modelValue', [...props.modelValue, tag.id])
    query.value = ''
  } catch (cause) { error.value = apiMessage(cause, '标签添加失败') } finally { creating.value = false }
}
</script>
