<template>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" aria-hidden="true">
    <path
      v-for="(path, index) in paths"
      :key="index"
      :d="path"
      stroke-linecap="round"
      stroke-linejoin="round"
      stroke-width="1.8"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BillKind } from '../../api/billing'

const props = defineProps<{ category: string; kind: BillKind }>()

const iconGroups: Array<[string[], string[]]> = [
  [['餐饮', '聚餐'], ['M7 3v8m3-8v8M5 7h7m-3 4v10M17 3v18m0-18c2.2 2.1 2.4 6.1 0 8h3']],
  [['交通', '旅行'], ['M5 17h14M7 17l1 3m8-3-1 3M6 13V7c0-2 2-3 6-3s6 1 6 3v6c0 2-1 4-3 4H9c-2 0-3-2-3-4Zm1-3h10M9 13h.01M15 13h.01']],
  [['购物', '采购'], ['M5 8h14l-1 12H6L5 8Zm3 0a4 4 0 0 1 8 0']],
  [['居住', '房租', '住宿', '水电'], ['m3 11 9-8 9 8v9H3v-9Zm6 9v-6h6v6']],
  [['娱乐', '门票', '活动'], ['M4 7h16v10H4V7Zm4 3h.01M12 10h4m-8 4h8']],
  [['医疗'], ['M9 3h6v6h6v6h-6v6H9v-6H3V9h6V3Z']],
  [['教育'], ['m3 9 9-5 9 5-9 5-9-5Zm4 3v5c3 2 7 2 10 0v-5']],
]

const moneyPaths = ['M12 2v20m5-16H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6']
const otherPaths = ['M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6']

const paths = computed(() => {
  const match = iconGroups.find(([categories]) => categories.includes(props.category))
  if (match) return match[1]
  if (props.kind === 'income' || ['理财', '报销', '礼金', '退款', '补贴', '共同收入'].includes(props.category)) return moneyPaths
  return otherPaths
})
</script>
