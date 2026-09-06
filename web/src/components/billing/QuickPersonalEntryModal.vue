<template>
  <Modal :model-value="modelValue" title="记一笔" @update:model-value="emit('update:modelValue', $event)">
    <form class="space-y-4" @submit.prevent="save">
      <div class="grid grid-cols-2 gap-2 rounded-xl bg-muted p-1" role="radiogroup" aria-label="账单类型">
        <button
          type="button"
          role="radio"
          :aria-checked="form.kind === 'expense'"
          :class="form.kind === 'expense' ? 'bg-surface text-destructive shadow-sm' : 'text-muted-foreground'"
          class="min-h-11 rounded-lg text-sm font-semibold transition-colors duration-200"
          @click="setKind('expense')"
        >
          支出
        </button>
        <button
          type="button"
          role="radio"
          :aria-checked="form.kind === 'income'"
          :class="form.kind === 'income' ? 'bg-surface text-emerald-700 shadow-sm dark:text-emerald-300' : 'text-muted-foreground'"
          class="min-h-11 rounded-lg text-sm font-semibold transition-colors duration-200"
          @click="setKind('income')"
        >
          收入
        </button>
      </div>

      <label class="form-field">
        <span>所属账本</span>
        <select v-model.number="form.ledger_id" required class="input-field" :disabled="resourcesLoading">
          <option :value="0" disabled>{{ resourcesLoading ? '加载中' : '请选择账本' }}</option>
          <option v-for="item in ledgers" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
      </label>

      <label class="form-field">
        <span>收支账户</span>
        <select v-model.number="form.account_id" required class="input-field" :disabled="resourcesLoading">
          <option :value="0" disabled>{{ resourcesLoading ? '加载中' : '请选择账户' }}</option>
          <option v-for="item in accounts" :key="item.id" :value="item.id">{{ item.name }} · {{ accountDetail(item) }}</option>
        </select>
      </label>
      <p v-if="!resourcesLoading && !accounts.length" class="rounded-xl bg-amber-500/10 px-3 py-2 text-sm text-amber-800 dark:text-amber-200">
        还没有可用账户，请先前往 <RouterLink to="/admin/personal/accounts" class="font-semibold underline">账户管理</RouterLink> 添加。
      </p>

      <label class="form-field">
        <span>金额</span>
        <div class="relative">
          <span class="absolute left-4 top-1/2 -translate-y-1/2 font-semibold text-muted-foreground">¥</span>
          <input
            ref="amountInput"
            v-model="form.amount"
            required
            inputmode="decimal"
            autocomplete="off"
            class="input-field pl-9 font-mono text-lg font-semibold tabular-nums"
            placeholder="0.00"
          />
        </div>
      </label>

      <CategoryPicker v-model="form.category" :kind="form.kind" :categories="categories" />
      <PersonalTagInput v-model="form.tag_ids" :tags="tags" @created="handleTagCreated" />

      <label class="form-field">
        <span>发生时间</span>
        <input v-model="form.occurred_at" required type="datetime-local" class="input-field" />
      </label>

      <label class="form-field">
        <span>备注 <small>可选</small></span>
        <textarea v-model="form.note" class="input-field min-h-20 resize-y" maxlength="500" placeholder="记录这笔钱的用途"></textarea>
      </label>

      <p v-if="formError" role="alert" class="text-sm text-destructive">{{ formError }}</p>
      <div class="flex gap-3 pt-2">
        <button type="button" class="btn-ghost min-h-11 flex-1" @click="emit('update:modelValue', false)">取消</button>
        <button class="btn-brand min-h-11 flex-1" :disabled="saving">
          {{ saving ? '保存中' : '保存账单' }}
        </button>
      </div>
    </form>
  </Modal>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { billingApi, type BillKind, type PersonalAccount, type PersonalCategory, type PersonalLedger, type PersonalTag } from '../../api/billing'
import { apiMessage, moneyToCents, toLocalInput } from '../../utils/billing'
import Modal from '../Modal.vue'
import CategoryPicker from './CategoryPicker.vue'
import PersonalTagInput from './PersonalTagInput.vue'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const amountInput = ref<HTMLInputElement | null>(null)
const saving = ref(false)
const resourcesLoading = ref(false)
const formError = ref('')
const ledgers = ref<PersonalLedger[]>([])
const accounts = ref<PersonalAccount[]>([])
const categories = ref<PersonalCategory[]>([])
const tags = ref<PersonalTag[]>([])
const form = reactive({
  ledger_id: 0,
  account_id: 0,
  kind: 'expense' as BillKind,
  amount: '',
  category: '',
  tag_ids: [] as number[],
  note: '',
  occurred_at: toLocalInput(),
})
const currentCategories = computed(() => categories.value.filter(item => item.kind === form.kind).map(item => item.name))

function resetForm() {
  Object.assign(form, { ledger_id: ledgers.value[0]?.id || 0, account_id: accounts.value[0]?.id || 0, kind: 'expense', amount: '', category: '', tag_ids: [], note: '', occurred_at: toLocalInput() })
  formError.value = ''
}

function accountDetail(item: PersonalAccount) { return item.type === 'bank_card' ? `${item.institution} · ${item.card_kind === 'credit' ? '信用卡' : item.card_kind === 'debit' ? '储蓄卡' : '银行卡'} · ${item.identifier ? `尾号 ${item.identifier}` : '未填写卡号'}` : item.institution }

function setKind(kind: BillKind) {
  form.kind = kind
  if (!currentCategories.value.includes(form.category)) form.category = ''
}

function handleTagCreated(tag: PersonalTag) { if (!tags.value.some(item => item.id === tag.id)) tags.value.push(tag) }

async function save() {
  const amount = moneyToCents(form.amount)
  if (!form.ledger_id) {
    formError.value = '请先创建并选择个人账本'
    return
  }
  if (!form.account_id) {
    formError.value = '请先添加并选择收支账户'
    return
  }
  if (amount <= 0) {
    formError.value = '请输入大于 0 的金额'
    amountInput.value?.focus()
    return
  }
  if (!form.category) {
    formError.value = '请选择分类'
    return
  }

  saving.value = true
  formError.value = ''
  try {
    await billingApi.personalCreate({
      ledger_id: form.ledger_id,
      account_id: form.account_id,
      kind: form.kind,
      amount_cents: amount,
      category: form.category,
      note: form.note.trim(),
      occurred_at: new Date(form.occurred_at).toISOString(),
      tag_ids: form.tag_ids,
    })
    emit('update:modelValue', false)
    emit('saved')
  } catch (error) {
    formError.value = apiMessage(error)
  } finally {
    saving.value = false
  }
}

watch(() => props.modelValue, async value => {
  if (!value) return
  resetForm()
  resourcesLoading.value = true
  try {
    const [ledgerRes, accountRes, categoryRes, tagRes] = await Promise.all([billingApi.personalLedgers(), billingApi.personalAccounts(), billingApi.personalCategories(), billingApi.personalTags()])
    const ledgerData = ledgerRes.data?.data
    const categoryData = categoryRes.data?.data
    const tagData = tagRes.data?.data
    ledgers.value = Array.isArray(ledgerData) ? ledgerData : ledgerData?.items || []
    const accountData = accountRes.data?.data
    accounts.value = Array.isArray(accountData) ? accountData : accountData?.items || []
    categories.value = Array.isArray(categoryData) ? categoryData : categoryData?.items || []
    tags.value = Array.isArray(tagData) ? tagData : tagData?.items || []
    form.ledger_id = ledgers.value[0]?.id || 0
    form.account_id = accounts.value[0]?.id || 0
  } catch (error) {
    formError.value = apiMessage(error, '账本资料加载失败')
  } finally {
    resourcesLoading.value = false
  }
  await nextTick()
  window.setTimeout(() => amountInput.value?.focus(), 50)
})
</script>
