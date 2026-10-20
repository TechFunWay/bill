<template>
  <Modal :model-value="modelValue" :title="step === 'select' ? '导入账单' : '确认导入'" size="xl" maximizable @update:model-value="value => emit('update:modelValue', value)">
    <template v-if="step === 'select'">
      <div class="space-y-4">
        <p class="text-sm leading-6 text-muted-foreground">
          支持导入支付宝「交易流水证明 / 交易明细」与微信「微信支付账单明细」导出的文件。重复的账单会自动识别并跳过，导入前可以逐笔核对账本、账户与分类。
        </p>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="rounded-xl bg-muted p-3 text-sm leading-6">
            <p class="font-semibold text-foreground">支付宝导出</p>
            <p class="mt-1 text-muted-foreground">我的 → 账单 → 右上角「···」→ 开具交易流水证明 → 用于个人对账 → 电子版 CSV，收到邮件后解压上传其中的 CSV。</p>
          </div>
          <div class="rounded-xl bg-muted p-3 text-sm leading-6">
            <p class="font-semibold text-foreground">微信导出</p>
            <p class="mt-1 text-muted-foreground">我 → 服务 → 钱包 → 账单 → 常见问题 → 下载账单 → 用于个人对账，解压密码在「微信支付」公众号查看，解压后的 xlsx 或 CSV 均可上传。</p>
          </div>
        </div>
        <label class="form-field">
          <span>账单文件</span>
          <input ref="fileInput" type="file" accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" class="input-field py-2 file:mr-3 file:rounded-lg file:border-0 file:bg-brand-100 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-brand-700" @change="handleFileChange" />
        </label>
        <p class="text-xs leading-5 text-muted-foreground">每笔账单都可在下一步单独调整账本、账户与分类后再导入。暂不支持直接上传加密压缩包。</p>
        <p v-if="parsing" class="text-sm text-brand-600">正在解析账单文件…</p>
        <p v-if="error" role="alert" class="text-sm text-rose-600">{{ error }}</p>
      </div>
    </template>

    <template v-else>
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2 rounded-xl bg-muted p-3 text-sm">
          <span class="font-semibold">{{ sourceLabel }}账单共 {{ rows.length }} 笔</span>
          <span class="badge bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-accent">已选 {{ checkedRows.length }}</span>
          <span v-if="duplicateCount" class="badge bg-amber-500/10 text-amber-700 dark:text-amber-300">重复 {{ duplicateCount }}</span>
          <span v-if="skippedCount" class="badge border border-border bg-surface text-muted-foreground">跳过 {{ skippedCount }}</span>
          <span v-if="checkedRows.length" class="ml-auto text-xs text-muted-foreground">支出 {{ formatMoney(checkedExpenseCents) }} · 收入 {{ formatMoney(checkedIncomeCents) }}</span>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label class="form-field"><span>默认账本 <small>应用到全部</small></span><select v-model.number="defaultLedgerId" class="input-field" @change="applyLedgerToAll"><option v-for="item in ledgers" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          <label class="form-field"><span>默认账户 <small>应用到全部</small></span><select v-model.number="defaultAccountId" class="input-field" @change="applyAccountToAll"><option v-for="item in accounts" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        </div>

        <div class="max-h-[46vh] divide-y divide-border overflow-y-auto rounded-xl border border-border">
          <div v-for="row in rows" :key="row.line" class="flex items-start gap-3 px-3 py-2.5" :class="row.checked ? '' : 'opacity-60'">
            <input type="checkbox" v-model="row.checked" class="mt-1.5 h-4 w-4 shrink-0 accent-brand-600" :aria-label="`选择第 ${row.line} 行账单`" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="badge px-2 py-0.5 leading-4" :class="row.kind === 'income' ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' : 'bg-rose-500/10 text-rose-700 dark:text-rose-300'">{{ row.kind === 'income' ? '收入' : '支出' }}</span>
                <select v-model="row.category" class="input-field h-8 w-28 px-2 py-0 text-sm" :disabled="!row.checked" :aria-label="`第 ${row.line} 行分类`">
                  <option value="" disabled>分类</option>
                  <option v-for="name in categoryNames(row.kind)" :key="name" :value="name">{{ name }}</option>
                </select>
                <select v-model.number="row.ledger_id" class="input-field h-8 w-32 px-2 py-0 text-sm" :disabled="!row.checked" :aria-label="`第 ${row.line} 行账本`">
                  <option v-for="item in ledgers" :key="item.id" :value="item.id">{{ item.name }}</option>
                </select>
                <select v-model.number="row.account_id" class="input-field h-8 w-36 px-2 py-0 text-sm" :disabled="!row.checked" :aria-label="`第 ${row.line} 行收支账户`">
                  <option v-for="item in accounts" :key="item.id" :value="item.id">{{ item.name }}</option>
                </select>
                <strong class="ml-auto whitespace-nowrap tabular-nums" :class="row.kind === 'income' ? 'text-emerald-600' : 'text-foreground'">{{ row.kind === 'income' ? '+' : '-' }}{{ formatMoney(row.amount_cents) }}</strong>
              </div>
              <p class="mt-0.5 truncate text-xs leading-4 text-muted-foreground">{{ row.occurred_at }}<template v-if="row.note"> · {{ row.note }}</template></p>
              <p v-if="row.checked && !row.category" class="mt-0.5 text-xs font-semibold text-rose-600">请选择分类</p>
              <p v-else-if="row.checked && !row.category_matched" class="mt-0.5 text-xs text-amber-600 dark:text-amber-400">账单分类不完全匹配，建议核对</p>
              <p v-if="!row.checked && row.skip_reason" class="mt-0.5 text-xs text-amber-600 dark:text-amber-400">{{ row.skip_reason }}</p>
            </div>
          </div>
        </div>

        <p v-if="missingCategoryCount" class="text-sm text-rose-600">还有 {{ missingCategoryCount }} 笔未选择分类，选好分类后才能导入。</p>
        <p v-if="error" role="alert" class="text-sm text-rose-600">{{ error }}</p>
        <div class="flex flex-col gap-2 sm:flex-row">
          <button type="button" class="btn-ghost min-h-9 flex-1" :disabled="importing" @click="backToSelect">重新选择文件</button>
          <button type="button" class="btn-brand min-h-9 flex-1" :disabled="importing || !checkedRows.length || missingCategoryCount > 0" @click="confirmImport">{{ importing ? '导入中' : `导入 ${checkedRows.length} 笔账单` }}</button>
        </div>
      </div>
    </template>
  </Modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { billingApi, type PersonalAccount, type PersonalCategory, type PersonalImportParseResult, type PersonalImportRow, type PersonalLedger } from '../../api/billing'
import { apiMessage, formatMoney } from '../../utils/billing'
import Modal from '../Modal.vue'

const props = defineProps<{
  modelValue: boolean
  ledgers: PersonalLedger[]
  accounts: PersonalAccount[]
  categories: PersonalCategory[]
  defaultLedgerId?: number
  defaultAccountId?: number
}>()

const emit = defineEmits<{ (event: 'update:modelValue', value: boolean): void; (event: 'imported', count: number): void }>()

const step = ref<'select' | 'preview'>('select')
const parsing = ref(false)
const importing = ref(false)
const error = ref('')
const source = ref<PersonalImportParseResult['source'] | ''>('')
interface EditableImportRow extends PersonalImportRow { checked: boolean; ledger_id: number; account_id: number }
const rows = ref<EditableImportRow[]>([])
const defaultLedgerId = ref(0)
const defaultAccountId = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)

const sourceLabel = computed(() => source.value === 'alipay' ? '支付宝' : source.value === 'wechat' ? '微信' : '')
const checkedRows = computed(() => rows.value.filter(row => row.checked))
const duplicateCount = computed(() => rows.value.filter(row => row.status === 'duplicate' && !row.checked).length)
const skippedCount = computed(() => rows.value.filter(row => row.status === 'skipped' && !row.checked).length)
const missingCategoryCount = computed(() => checkedRows.value.filter(row => !row.category).length)
const checkedExpenseCents = computed(() => checkedRows.value.filter(row => row.kind === 'expense').reduce((sum, row) => sum + row.amount_cents, 0))
const checkedIncomeCents = computed(() => checkedRows.value.filter(row => row.kind === 'income').reduce((sum, row) => sum + row.amount_cents, 0))

function categoryNames(kind: PersonalImportRow['kind']) { return props.categories.filter(item => item.kind === kind).map(item => item.name) }
function fallbackLedgerId() { return props.defaultLedgerId || props.ledgers[0]?.id || 0 }
function fallbackAccountId() { return props.defaultAccountId || props.accounts[0]?.id || 0 }
function applyLedgerToAll() { rows.value.forEach(row => { row.ledger_id = defaultLedgerId.value }) }
function applyAccountToAll() { rows.value.forEach(row => { row.account_id = defaultAccountId.value }) }

function resetState() {
  step.value = 'select'
  source.value = ''
  rows.value = []
  error.value = ''
  parsing.value = false
  importing.value = false
  if (fileInput.value) fileInput.value.value = ''
}

watch(() => props.modelValue, value => {
  if (value) {
    resetState()
    defaultLedgerId.value = fallbackLedgerId()
    defaultAccountId.value = fallbackAccountId()
  }
})

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  parsing.value = true
  error.value = ''
  try {
    const res = await billingApi.personalImportParse(file)
    const result = res.data?.data as PersonalImportParseResult
    source.value = result.source
    rows.value = (result.rows || []).map(row => ({ ...row, checked: row.status === 'import', ledger_id: defaultLedgerId.value || fallbackLedgerId(), account_id: defaultAccountId.value || fallbackAccountId() }))
    if (!rows.value.some(row => row.status === 'import' || row.status === 'duplicate')) {
      error.value = '文件里没有可导入的账单（可能全部是中性交易或已完成导入）'
      return
    }
    step.value = 'preview'
  } catch (caught) {
    error.value = apiMessage(caught, '解析账单文件失败')
  } finally {
    parsing.value = false
  }
}

function backToSelect() {
  step.value = 'select'
  rows.value = []
  error.value = ''
  if (fileInput.value) fileInput.value.value = ''
}

async function confirmImport() {
  if (!checkedRows.value.length || missingCategoryCount.value) return
  importing.value = true
  error.value = ''
  try {
    const res = await billingApi.personalImportConfirm({
      source: source.value as PersonalImportParseResult['source'],
      rows: checkedRows.value.map(({ occurred_at, kind, amount_cents, category, note, external_id, ledger_id, account_id }) => ({ occurred_at, kind, amount_cents, category, note, external_id, ledger_id, account_id })),
    })
    emit('imported', res.data?.data?.imported || 0)
    emit('update:modelValue', false)
  } catch (caught) {
    error.value = apiMessage(caught, '导入账单失败')
  } finally {
    importing.value = false
  }
}
</script>
