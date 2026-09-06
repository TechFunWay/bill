<template>
  <div class="grid gap-3 sm:grid-cols-2">
    <label class="form-field"><span>一级分类</span><select v-model.number="rootID" class="input-field" @change="selectRoot"><option :value="0" disabled>请选择一级分类</option><option v-for="item in roots" :key="item.id" :value="item.id">{{ item.name }}</option><option v-if="legacy" :value="-1">{{ legacy }}（历史分类）</option></select></label>
    <label v-if="children.length" class="form-field"><span>二级分类</span><select v-model.number="childID" class="input-field" @change="selectChild"><option :value="0" disabled>请选择二级分类</option><option v-for="item in children" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { BillKind, PersonalCategory } from '../../api/billing'

const props = defineProps<{ modelValue: string; kind: BillKind; categories: PersonalCategory[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const rootID = ref(0), childID = ref(0)
const kindCategories = computed(() => props.categories.filter(item => item.kind === props.kind))
const roots = computed(() => kindCategories.value.filter(item => !item.parent_id).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')))
const children = computed(() => kindCategories.value.filter(item => item.parent_id === rootID.value).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')))
const legacy = computed(() => props.modelValue && !kindCategories.value.some(item => item.name === props.modelValue) ? props.modelValue : '')

function sync() {
  const selected = kindCategories.value.find(item => item.name === props.modelValue)
  if (!selected) {
    if (!props.modelValue && roots.value.some(item => item.id === rootID.value)) return
    rootID.value = legacy.value ? -1 : 0; childID.value = 0; return
  }
  rootID.value = selected.parent_id || selected.id
  childID.value = selected.parent_id ? selected.id : 0
}
function selectRoot() {
  if (rootID.value === -1) { emit('update:modelValue', legacy.value); return }
  childID.value = 0
  const root = roots.value.find(item => item.id === rootID.value)
  const hasChildren = kindCategories.value.some(item => item.parent_id === rootID.value)
  emit('update:modelValue', root && !hasChildren ? root.name : '')
}
function selectChild() { emit('update:modelValue', children.value.find(item => item.id === childID.value)?.name || '') }
watch(() => [props.modelValue, props.kind, props.categories] as const, sync, { immediate: true, deep: true })
</script>
