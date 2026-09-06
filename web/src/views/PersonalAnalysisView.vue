<template>
  <div class="page-container animate-fade-in" :aria-busy="loading">
    <PageHeader title="统计分析" description="按时间与账本查看收支、分类、标签和消费习惯。" />

    <section class="surface rounded-2xl p-4 sm:p-5" aria-labelledby="analysis-filter-title">
      <h2 id="analysis-filter-title" class="sr-only">统计筛选</h2>
      <MobileFilterDisclosure v-model="filtersExpanded" panel-id="analysis-filters" title="统计筛选" :summary="analysisFilterSummary" :active-count="analysisFilterCount">
        <form class="grid gap-3 sm:grid-cols-2 xl:grid-cols-[1.25fr_1fr_1fr_auto]" @submit.prevent="applyFilters">
        <label class="filter-field">
          <span>账本</span>
          <select v-model="filters.ledger_id" class="input-field" aria-describedby="ledger-filter-hint">
            <option value="">全部账本</option>
            <option v-for="ledger in ledgerOptions" :key="ledger.id" :value="String(ledger.id)">{{ ledger.name }}</option>
          </select>
          <small id="ledger-filter-hint" class="sr-only">留空将统计全部个人账本</small>
        </label>
        <label class="filter-field">
          <span>开始日期</span>
          <input v-model="filters.start" type="date" class="input-field" />
        </label>
        <label class="filter-field">
          <span>结束日期</span>
          <input v-model="filters.end" type="date" class="input-field" />
        </label>
        <button class="btn-brand min-h-11 self-end xl:min-w-28" :disabled="loading">
          <svg v-if="loading" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle class="opacity-25" cx="12" cy="12" r="9" stroke="currentColor" stroke-width="3"/><path class="opacity-75" d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-linecap="round" stroke-width="3"/></svg>
          {{ loading ? '统计中' : '应用筛选' }}
        </button>
        </form>
        <p v-if="filterError" class="mt-3 text-sm font-medium text-rose-600" role="alert">{{ filterError }}</p>
        <p v-else class="mt-3 text-xs text-muted-foreground">当前范围：{{ analysisFilterSummary }}</p>
      </MobileFilterDisclosure>
    </section>

    <div v-if="loading && !hasLoaded" class="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4" aria-label="正在加载统计数据">
      <div v-for="n in 4" :key="n" class="surface h-36 animate-pulse rounded-2xl bg-muted/60"></div>
    </div>
    <template v-else>
      <div class="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4">
        <article class="money-card">
          <span class="money-label">区间收入</span>
          <strong class="money-value text-emerald-700 dark:text-emerald-300">{{ formatMoney(stats.summary.income_cents) }}</strong>
          <span class="money-meta">共 {{ stats.summary.count }} 笔记录</span>
        </article>
        <article class="money-card">
          <span class="money-label">区间支出</span>
          <strong class="money-value text-rose-700 dark:text-rose-300">{{ formatMoney(stats.summary.expense_cents) }}</strong>
          <span class="money-meta">筛选范围内全部支出</span>
        </article>
        <article class="money-card">
          <span class="money-label">净结余</span>
          <strong :class="stats.summary.balance_cents >= 0 ? 'text-emerald-700 dark:text-emerald-300' : 'text-rose-700 dark:text-rose-300'" class="money-value">{{ formatMoney(stats.summary.balance_cents) }}</strong>
          <span class="money-meta">收入减去支出</span>
        </article>
        <article class="money-card">
          <span class="money-label">储蓄率</span>
          <strong class="money-value">{{ savingRate }}%</strong>
          <span class="money-meta">净结余占收入比例</span>
        </article>
      </div>

      <div class="grid gap-4 xl:grid-cols-2">
        <section class="surface rounded-2xl p-4 sm:p-6 xl:col-span-2" aria-labelledby="trend-title">
          <ChartHeading id="trend-title" title="收支趋势" :description="trendDescription" />
          <div v-if="!trendSeries.length" class="empty-state h-64">
            <EmptyChartIcon />
            <span class="mt-3">当前范围暂无收支趋势</span>
          </div>
          <template v-else>
            <div class="mt-5 flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted-foreground" aria-label="图例">
              <span class="flex items-center gap-2"><i class="w-7 border-t-2 border-dashed border-emerald-600" aria-hidden="true"></i>收入（虚线圆点）</span>
              <span class="flex items-center gap-2"><i class="w-7 border-t-2 border-rose-600" aria-hidden="true"></i>支出（实线方点）</span>
            </div>
            <div class="mt-3 overflow-hidden" role="img" :aria-label="`${trendDescription}。收入与支出的完整数值见数据表。`">
              <svg class="h-auto min-h-64 w-full" :viewBox="`0 0 ${chart.width} ${chart.height}`" aria-hidden="true">
                <g class="text-muted-foreground">
                  <line v-for="tick in yTicks" :key="tick.value" :x1="chart.left" :x2="chart.width - chart.right" :y1="tick.y" :y2="tick.y" stroke="currentColor" stroke-opacity=".18" />
                  <text v-for="tick in yTicks" :key="`label-${tick.value}`" :x="chart.left - 10" :y="tick.y + 4" text-anchor="end" fill="currentColor" font-size="11">{{ compactMoney(tick.value) }}</text>
                  <text v-for="tick in xTicks" :key="tick.index" :x="tick.x" :y="chart.height - 12" text-anchor="middle" fill="currentColor" font-size="11">{{ tick.label }}</text>
                </g>
                <polyline :points="incomeLine" fill="none" stroke="#047857" stroke-dasharray="7 5" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
                <polyline :points="expenseLine" fill="none" stroke="#be123c" stroke-linecap="round" stroke-linejoin="round" stroke-width="3" />
                <g v-for="(item, index) in trendSeries" :key="item.label">
                  <circle :cx="pointX(index)" :cy="pointY(item.income_cents)" r="4" fill="#047857"><title>{{ item.label }}收入 {{ formatMoney(item.income_cents) }}</title></circle>
                  <rect :x="pointX(index) - 4" :y="pointY(item.expense_cents) - 4" width="8" height="8" rx="1" fill="#be123c"><title>{{ item.label }}支出 {{ formatMoney(item.expense_cents) }}</title></rect>
                </g>
              </svg>
            </div>
            <table class="sr-only">
              <caption>收支趋势数据</caption>
              <thead><tr><th scope="col">日期</th><th scope="col">收入</th><th scope="col">支出</th></tr></thead>
              <tbody><tr v-for="item in trendSeries" :key="`trend-table-${item.label}`"><th scope="row">{{ item.label }}</th><td>{{ formatMoney(item.income_cents) }}</td><td>{{ formatMoney(item.expense_cents) }}</td></tr></tbody>
            </table>
          </template>
        </section>

        <RankingChart title="支出分类" description="按支出金额从高到低，最多显示 8 项" :items="categoryRows" empty-text="当前范围暂无分类支出" value-label="金额" />
        <RankingChart title="账本对比" description="不同账本的支出规模与记录数" :items="ledgerRows" empty-text="当前范围暂无账本数据" value-label="支出" />
        <RankingChart title="标签排名" description="按标签关联金额从高到低，最多显示 8 项" :items="tagRows" empty-text="当前范围暂无标签数据" value-label="金额" />
        <RankingChart title="星期分布" description="查看支出集中在一周中的哪些日子" :items="weekdayRows" empty-text="当前范围暂无星期分布" value-label="支出" />
      </div>
    </template>

    <Toast :message="errorMessage" type="error" />
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, type PropType } from 'vue'
import { billingApi } from '../api/billing'
import { apiMessage, formatMoney } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import Toast from '../components/Toast.vue'
import MobileFilterDisclosure from '../components/MobileFilterDisclosure.vue'

interface Summary { income_cents: number; expense_cents: number; balance_cents: number; count: number }
interface CategoryStat { category: string; amount_cents: number; count: number }
interface TrendStat { month: string; income_cents: number; expense_cents: number }
interface LedgerStat { ledger_id?: number; id?: number; ledger_name?: string; name?: string; income_cents: number; expense_cents: number; count: number }
interface TagStat { tag_id?: number; id?: number; tag?: string; name?: string; amount_cents: number; count: number }
interface DailyStat { date: string; income_cents: number; expense_cents: number }
interface WeekdayStat { weekday: number | string; amount_cents: number; count: number }
interface AnalysisStats {
  summary: Summary
  categories: CategoryStat[]
  trend: TrendStat[]
  ledgers: LedgerStat[]
  tags: TagStat[]
  daily: DailyStat[]
  weekdays: WeekdayStat[]
}
interface LedgerOption { id: number; name: string }
interface TrendPoint { label: string; income_cents: number; expense_cents: number }
interface RankingRow { key: string; label: string; amount_cents: number; count: number; detail?: string }

const today = new Date()
const firstDay = new Date(today.getFullYear(), today.getMonth(), 1)
const filters = reactive({ start: localDate(firstDay), end: localDate(today), ledger_id: '' })
const applied = reactive({ start: filters.start, end: filters.end, ledger_id: '' })
const loading = ref(false)
const hasLoaded = ref(false)
const filtersExpanded = ref(false)
const errorMessage = ref('')
const filterError = ref('')
const ledgerOptions = ref<LedgerOption[]>([])
const emptySummary = (): Summary => ({ income_cents: 0, expense_cents: 0, balance_cents: 0, count: 0 })
const stats = reactive<AnalysisStats>({ summary: emptySummary(), categories: [], trend: [], ledgers: [], tags: [], daily: [], weekdays: [] })

const savingRate = computed(() => stats.summary.income_cents ? Math.round(stats.summary.balance_cents / stats.summary.income_cents * 100) : 0)
const scopeLabel = computed(() => applied.ledger_id ? (ledgerOptions.value.find(item => String(item.id) === applied.ledger_id)?.name || '指定账本') : '全部账本')
const dateRangeLabel = computed(() => `${applied.start || '不限开始日期'} 至 ${applied.end || '不限结束日期'}`)
const analysisFilterSummary = computed(() => `${scopeLabel.value} · ${dateRangeLabel.value}`)
const analysisFilterCount = computed(() => [applied.ledger_id, applied.start, applied.end].filter(Boolean).length)
const trendSeries = computed<TrendPoint[]>(() => stats.daily.map(item => ({ label: item.date, income_cents: item.income_cents, expense_cents: item.expense_cents })))
const trendDescription = computed(() => '按日展示筛选区间内的收入与支出')

const categoryRows = computed(() => stats.categories.slice().sort((a, b) => b.amount_cents - a.amount_cents).slice(0, 8).map((item, index) => ({ key: `${item.category}-${index}`, label: item.category || '未分类', amount_cents: item.amount_cents, count: item.count })))
const ledgerRows = computed(() => stats.ledgers.slice().sort((a, b) => b.expense_cents - a.expense_cents).map((item, index) => ({ key: String(item.ledger_id ?? item.id ?? index), label: item.ledger_name || item.name || '未命名账本', amount_cents: item.expense_cents, count: item.count, detail: item.income_cents ? `收入 ${formatMoney(item.income_cents)}` : undefined })))
const tagRows = computed(() => stats.tags.slice().sort((a, b) => b.amount_cents - a.amount_cents).slice(0, 8).map((item, index) => ({ key: String(item.tag_id ?? item.id ?? index), label: item.tag || item.name || '未命名标签', amount_cents: item.amount_cents, count: item.count })))
const weekdayRows = computed(() => stats.weekdays.filter(item => item.amount_cents > 0 || item.count > 0).map((item, index) => ({ key: String(item.weekday ?? index), label: weekdayLabel(item.weekday), amount_cents: item.amount_cents, count: item.count })).sort((a, b) => b.amount_cents - a.amount_cents))

const chart = { width: 720, height: 280, left: 64, right: 18, top: 22, bottom: 42 }
const maxTrend = computed(() => Math.max(1, ...trendSeries.value.flatMap(item => [item.income_cents, item.expense_cents])))
const yTicks = computed(() => [maxTrend.value, Math.round(maxTrend.value / 2), 0].map(value => ({ value, y: pointY(value) })))
const xTicks = computed(() => {
  const count = trendSeries.value.length
  if (!count) return []
  const step = Math.max(1, Math.ceil(count / 6))
  return trendSeries.value.map((item, index) => ({ index, x: pointX(index), label: axisDate(item.label) })).filter((_, index) => index % step === 0 || index === count - 1)
})
const incomeLine = computed(() => trendSeries.value.map((item, index) => `${pointX(index)},${pointY(item.income_cents)}`).join(' '))
const expenseLine = computed(() => trendSeries.value.map((item, index) => `${pointX(index)},${pointY(item.expense_cents)}`).join(' '))

function localDate(value: Date) {
  const offset = value.getTimezoneOffset() * 60_000
  return new Date(value.getTime() - offset).toISOString().slice(0, 10)
}
function numberValue(value: unknown) { const parsed = Number(value); return Number.isFinite(parsed) ? parsed : 0 }
function normalize(raw: Partial<AnalysisStats> | undefined): AnalysisStats {
  const summaryRaw = raw?.summary || emptySummary()
  const income = numberValue(summaryRaw.income_cents)
  const expense = numberValue(summaryRaw.expense_cents)
  return {
    summary: { income_cents: income, expense_cents: expense, balance_cents: Number.isFinite(Number(summaryRaw.balance_cents)) ? Number(summaryRaw.balance_cents) : income - expense, count: numberValue(summaryRaw.count) },
    categories: Array.isArray(raw?.categories) ? raw.categories : [],
    trend: Array.isArray(raw?.trend) ? raw.trend : [],
    ledgers: Array.isArray(raw?.ledgers) ? raw.ledgers : [],
    tags: Array.isArray(raw?.tags) ? raw.tags : [],
    daily: Array.isArray(raw?.daily) ? raw.daily : [],
    weekdays: Array.isArray(raw?.weekdays) ? raw.weekdays : [],
  }
}
function rememberLedgers(items: LedgerStat[]) {
  const incoming = items.flatMap((item) => {
    const id = numberValue(item.ledger_id ?? item.id)
    const name = item.ledger_name || item.name
    return id && name ? [{ id, name }] : []
  })
  const merged = new Map(ledgerOptions.value.map(item => [item.id, item]))
  incoming.forEach(item => merged.set(item.id, item))
  ledgerOptions.value = [...merged.values()]
}
function applyFilters() {
  filterError.value = ''
  if (filters.start && filters.end && filters.start > filters.end) { filterError.value = '开始日期不能晚于结束日期'; return }
  Object.assign(applied, filters)
  filtersExpanded.value = false
  load()
}
async function load() {
  loading.value = true
  errorMessage.value = ''
  try {
    const params = { start: applied.start, end: applied.end, ledger_id: applied.ledger_id || undefined }
    const response = await billingApi.personalStats(params)
    const next = normalize(response.data.data as Partial<AnalysisStats>)
    Object.assign(stats, next)
    rememberLedgers(next.ledgers)
  } catch (error) {
    errorMessage.value = apiMessage(error, '统计加载失败')
  } finally {
    loading.value = false
    hasLoaded.value = true
  }
}
async function loadLedgerOptions() {
  try {
    const response = await billingApi.personalLedgers(true)
    const data = response.data?.data
    const items = (Array.isArray(data) ? data : data?.items || []) as Array<{ id?: number; name?: string; archived?: boolean }>
    ledgerOptions.value = items.filter(item => item.id && item.name).map(item => ({ id: numberValue(item.id), name: `${item.name}${item.archived ? '（已归档）' : ''}` }))
  } catch {
    // Stats aggregates remain a usable fallback when the resource list is unavailable.
  }
}
function pointX(index: number) {
  const usable = chart.width - chart.left - chart.right
  return trendSeries.value.length <= 1 ? chart.left + usable / 2 : chart.left + index * usable / (trendSeries.value.length - 1)
}
function pointY(value: number) {
  const usable = chart.height - chart.top - chart.bottom
  return chart.top + usable - Math.max(0, value) / maxTrend.value * usable
}
function compactMoney(cents: number) {
  const yuan = cents / 100
  return `¥${new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 }).format(yuan)}`
}
function axisDate(value: string) { return /^\d{4}-\d{2}-\d{2}/.test(value) ? value.slice(5, 10).replace('-', '/') : value.slice(0, 7) }
function weekdayLabel(value: number | string) {
  if (typeof value === 'string' && !/^\d+$/.test(value)) return value
  const labels = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']
  return labels[((Number(value) % 7) + 7) % 7] || String(value)
}

const ChartHeading = defineComponent({
  props: { id: { type: String, required: true }, title: { type: String, required: true }, description: { type: String, required: true } },
  setup(props) { return () => h('div', [h('h2', { id: props.id, class: 'font-display text-lg font-bold' }, props.title), h('p', { class: 'mt-1 text-xs text-muted-foreground' }, props.description)]) },
})
const EmptyChartIcon = defineComponent({
  setup() { return () => h('svg', { class: 'h-10 w-10 text-muted-foreground/60', viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'aria-hidden': 'true' }, [h('path', { d: 'M4 19V9m6 10V5m6 14v-7m4 7H2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '1.5' })]) },
})
const RankingChart = defineComponent({
  props: {
    title: { type: String, required: true }, description: { type: String, required: true },
    items: { type: Array as PropType<RankingRow[]>, required: true }, emptyText: { type: String, required: true }, valueLabel: { type: String, required: true },
  },
  setup(props) {
    return () => {
      const max = Math.max(1, ...props.items.map(item => item.amount_cents))
      return h('section', { class: 'surface rounded-2xl p-4 sm:p-6', 'aria-label': props.title }, [
        h(ChartHeading, { id: `chart-${props.title}`, title: props.title, description: props.description }),
        props.items.length ? h('div', { class: 'mt-6 space-y-5' }, props.items.map((item, index) => h('div', { key: item.key }, [
          h('div', { class: 'mb-2 flex items-start justify-between gap-3 text-sm' }, [
            h('div', { class: 'min-w-0' }, [h('span', { class: 'block truncate font-semibold' }, `${index + 1}. ${item.label}`), item.detail ? h('span', { class: 'mt-0.5 block text-xs text-muted-foreground' }, item.detail) : null]),
            h('span', { class: 'shrink-0 text-right font-mono text-xs tabular-nums text-muted-foreground' }, `${formatMoney(item.amount_cents)} · ${item.count} 笔`),
          ]),
          h('div', { class: 'h-2.5 overflow-hidden rounded-full bg-muted', role: 'img', 'aria-label': `${item.label}，${props.valueLabel}${formatMoney(item.amount_cents)}，${item.count} 笔` }, [
            h('div', { class: ['h-full rounded-full bg-brand-600 dark:bg-accent', index % 3 === 1 ? 'opacity-75' : index % 3 === 2 ? 'opacity-55' : ''], style: { width: `${Math.max(2, item.amount_cents / max * 100)}%` } }),
          ]),
        ]))) : h('div', { class: 'empty-state h-64' }, [h(EmptyChartIcon), h('span', { class: 'mt-3' }, props.emptyText)]),
        props.items.length ? h('table', { class: 'sr-only' }, [h('caption', `${props.title}数据`), h('thead', [h('tr', [h('th', { scope: 'col' }, '项目'), h('th', { scope: 'col' }, props.valueLabel), h('th', { scope: 'col' }, '笔数')])]), h('tbody', props.items.map(item => h('tr', { key: `table-${item.key}` }, [h('th', { scope: 'row' }, item.label), h('td', formatMoney(item.amount_cents)), h('td', String(item.count))])))]) : null,
      ])
    }
  },
})

onMounted(() => { loadLedgerOptions(); load() })
</script>
