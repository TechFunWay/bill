<template>
  <div>
    <div v-if="!compact" class="mb-2 flex items-center justify-between gap-3">
      <div>
        <span class="text-sm font-semibold text-foreground">账单图片 <small class="font-normal text-muted-foreground">可选</small></span>
        <p v-if="!readonly" class="mt-0.5 text-xs text-muted-foreground">支持 JPG、PNG、GIF、WebP，最多 9 张，单张不超过 20 MB</p>
      </div>
      <span class="shrink-0 text-xs tabular-nums text-muted-foreground">{{ modelValue.length }}/9</span>
    </div>

    <div v-if="modelValue.length" :class="compact ? 'flex flex-wrap gap-1.5' : 'grid grid-cols-3 gap-2 sm:grid-cols-4'">
      <button
        v-for="(item, index) in visibleItems"
        :key="item.id || item.path"
        type="button"
        :class="compact ? 'h-10 w-10 rounded-lg' : 'group aspect-square rounded-xl'"
        class="relative overflow-hidden border border-border bg-muted outline-none transition hover:border-brand-400 focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
        :aria-label="`预览图片：${item.name}`"
        @click="openPreview(item)"
      >
        <img :src="assetUrl(item.path)" :alt="item.name" class="h-full w-full object-cover" loading="lazy" />
        <span v-if="!compact" class="absolute inset-x-0 bottom-0 truncate bg-black/55 px-2 py-1 text-left text-[11px] text-white opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">{{ item.name }}</span>
      </button>
      <button v-if="compact && modelValue.length > compactLimit" type="button" class="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-muted text-xs font-semibold text-muted-foreground hover:text-foreground" @click="openPreview(modelValue[compactLimit])">+{{ modelValue.length - compactLimit }}</button>
    </div>

    <div v-if="!readonly && !compact" class="mt-2 grid gap-2" :class="modelValue.length ? '' : 'mt-0'">
      <input ref="fileInput" class="sr-only" type="file" accept="image/jpeg,image/png,image/gif,image/webp" multiple @change="handleFiles" />
      <button type="button" class="flex min-h-12 items-center justify-center gap-2 rounded-xl border border-dashed border-border bg-muted/40 px-4 text-sm font-semibold text-muted-foreground transition hover:border-brand-400 hover:bg-brand-50 hover:text-brand-700 dark:hover:bg-brand-900/20" :disabled="uploading || modelValue.length >= maxAttachments" @click="fileInput?.click()">
        <svg v-if="!uploading" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M4 16.5V19a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2.5M12 3v13m-4-9 4-4 4 4"/></svg>
        <svg v-else class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="9" stroke="currentColor" stroke-width="3"/><path class="opacity-75" fill="currentColor" d="M21 12a9 9 0 0 0-9-9v3a6 6 0 0 1 6 6z"/></svg>
        {{ uploading ? '图片上传中…' : modelValue.length ? '继续添加图片' : '选择多张图片' }}
      </button>
      <div v-if="modelValue.length" class="flex flex-wrap gap-2">
        <button v-for="item in modelValue" :key="`remove-${item.id || item.path}`" type="button" class="inline-flex max-w-full items-center gap-1 rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/30" :aria-label="`移除图片：${item.name}`" @click="remove(item)">
          <span class="max-w-48 truncate">{{ item.name }}</span>
          <svg class="h-3.5 w-3.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-width="2" d="M6 6l12 12M18 6 6 18"/></svg>
        </button>
      </div>
    </div>
    <p v-if="errorMessage && !compact" role="alert" class="mt-2 text-xs text-rose-600">{{ errorMessage }}</p>

    <Modal v-model="previewOpen" :title="previewItem?.name || '图片预览'" size="wide">
      <div v-if="previewItem" class="space-y-4">
        <div class="flex min-h-64 items-center justify-center overflow-hidden rounded-xl bg-black/90 p-2 sm:min-h-[28rem]">
          <img :src="assetUrl(previewItem.path)" :alt="previewItem.name" class="max-h-[68vh] max-w-full object-contain" />
        </div>
        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p class="truncate text-sm text-muted-foreground">{{ previewItem.name }}</p>
          <a class="btn-brand min-h-11 shrink-0" :href="assetUrl(previewItem.path)" :download="previewItem.name || '账单图片'">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" d="M12 3v12m-4-4 4 4 4-4M5 20h14"/></svg>
            下载原图
          </a>
        </div>
      </div>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { billingApi, type BillAttachment } from '../../api/billing'
import { apiMessage } from '../../utils/billing'
import Modal from '../Modal.vue'

const maxAttachments = 9
const maxFileSize = 20 * 1024 * 1024
const acceptedTypes = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp'])
const compactLimit = 3

const props = withDefaults(defineProps<{
  modelValue: BillAttachment[]
  readonly?: boolean
  compact?: boolean
}>(), { readonly: false, compact: false })

const emit = defineEmits<{
  'update:modelValue': [value: BillAttachment[]]
  uploading: [value: boolean]
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const errorMessage = ref('')
const previewOpen = ref(false)
const previewItem = ref<BillAttachment | null>(null)
const visibleItems = computed(() => props.compact ? props.modelValue.slice(0, compactLimit) : props.modelValue)

function assetUrl(path: string) {
  if (/^(https?:)?\/\//.test(path)) return path
  return `${import.meta.env.BASE_URL}${path.replace(/^\/+/, '')}`
}

function openPreview(item: BillAttachment) {
  previewItem.value = item
  previewOpen.value = true
}

function remove(item: BillAttachment) {
  emit('update:modelValue', props.modelValue.filter(current => current.path !== item.path))
  errorMessage.value = ''
}

async function handleFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const selected = Array.from(input.files || [])
  input.value = ''
  if (!selected.length) return
  const remaining = maxAttachments - props.modelValue.length
  if (remaining <= 0) {
    errorMessage.value = '每笔账单最多上传 9 张图片'
    return
  }
  const candidates = selected.slice(0, remaining)
  const valid = candidates.filter(file => acceptedTypes.has(file.type) && file.size > 0 && file.size <= maxFileSize)
  const rejectedCount = selected.length - valid.length
  uploading.value = true
  emit('uploading', true)
  errorMessage.value = ''
  try {
    const results = await Promise.allSettled(valid.map(async file => {
      const response = await billingApi.uploadImage(file)
      const uploadedPath = response.data?.data?.path
      if (!uploadedPath) throw new Error('上传结果缺少图片路径')
      return { path: uploadedPath, name: file.name } as BillAttachment
    }))
    const uploaded = results.flatMap(result => result.status === 'fulfilled' ? [result.value] : [])
    if (uploaded.length) emit('update:modelValue', [...props.modelValue, ...uploaded])
    const failedCount = results.length - uploaded.length
    const messages: string[] = []
    if (selected.length > remaining) messages.push(`已达到 9 张上限，忽略 ${selected.length - remaining} 张`)
    if (rejectedCount) messages.push(`${rejectedCount} 张格式或大小不符合要求`)
    if (failedCount) {
      const firstFailure = results.find(result => result.status === 'rejected')
      messages.push(`${failedCount} 张上传失败${firstFailure?.status === 'rejected' ? `：${apiMessage(firstFailure.reason)}` : ''}`)
    }
    errorMessage.value = messages.join('；')
  } finally {
    uploading.value = false
    emit('uploading', false)
  }
}

</script>
