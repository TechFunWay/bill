<template>
  <svg :class="sizeClass" viewBox="0 0 40 40" aria-hidden="true">
    <circle cx="20" cy="20" r="19" :fill="bank?.color || '#596174'" />
    <circle cx="20" cy="20" r="16.5" fill="none" stroke="white" stroke-opacity=".28" />
    <text x="20" y="21" fill="white" text-anchor="middle" dominant-baseline="middle" :font-size="label.length > 1 ? 11 : 17" font-weight="700">{{ label }}</text>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BankOption } from '../../api/billing'

const props = withDefaults(defineProps<{ bank?: BankOption; size?: 'sm' | 'md' }>(), { size: 'md' })
const label = computed(() => props.bank?.short || '银')
const sizeClass = computed(() => props.size === 'sm' ? 'h-7 w-7 shrink-0' : 'h-10 w-10 shrink-0')
</script>
