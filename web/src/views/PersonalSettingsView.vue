<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="分类与标签" description="管理个人账单使用的分类和标签；账本请前往《我的账本》。">
      <template #actions><RouterLink to="/admin/personal" class="btn-ghost min-h-11">返回个人账单</RouterLink></template>
    </PageHeader>

    <div class="surface grid grid-cols-2 gap-1 rounded-2xl p-1" role="tablist" aria-label="资料类型">
      <button v-for="tab in tabs" :key="tab.key" type="button" role="tab" :aria-selected="active === tab.key" :class="active === tab.key ? 'bg-brand-800 text-white shadow-soft dark:bg-accent dark:text-brand-900' : 'text-muted-foreground hover:bg-muted'" class="min-h-11 rounded-xl px-3 text-sm font-bold transition-colors duration-200" @click="active = tab.key">{{ tab.label }}</button>
    </div>

    <section class="surface rounded-2xl p-4 sm:p-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div><h2 class="font-display text-xl font-bold">{{ currentTab.title }}</h2><p class="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">{{ currentTab.description }}</p></div>
        <button class="btn-brand min-h-11 shrink-0" @click="openCreate">添加{{ currentTab.itemLabel }}</button>
      </div>

      <div v-if="loading" class="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-3"><div v-for="n in 3" :key="n" class="h-32 animate-pulse rounded-2xl bg-muted"></div></div>
      <div v-else-if="!currentItems.length" class="empty-state mt-6 min-h-52 rounded-2xl border border-dashed border-border bg-muted/20"><strong class="text-foreground">还没有{{ currentTab.itemLabel }}</strong><span class="mt-2">点击右上角开始添加</span></div>
      <div v-else class="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <article v-for="item in currentItems" :key="item.id" :class="active === 'categories' && item.parent_id ? 'ml-5 border-l-4 border-l-brand-500 sm:ml-8' : ''" class="rounded-2xl border border-border bg-background/30 p-4">
          <div class="flex min-w-0 items-start justify-between gap-3">
            <div class="min-w-0"><div class="flex flex-wrap items-center gap-2"><h3 class="truncate font-bold">{{ item.name }}</h3><span v-if="active === 'categories'" :class="item.kind === 'income' ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300' : 'bg-rose-500/10 text-rose-700 dark:text-rose-300'" class="badge">{{ item.kind === 'income' ? '收入' : '支出' }}</span><span v-if="active === 'categories'" class="badge bg-muted text-muted-foreground">{{ item.parent_id ? '二级' : '一级' }}</span></div><p class="mt-2 text-xs text-muted-foreground">{{ itemMeta(item) }}</p></div>
            <div class="flex shrink-0"><button class="icon-button" :aria-label="`编辑${item.name}`" @click="openEdit(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20h4L19 9l-4-4L4 16v4z"/></svg></button><button class="icon-button text-rose-600" :aria-label="`删除${item.name}`" @click="askDelete(item)"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7h16M9 7V4h6v3m-9 0 1 14h10l1-14"/></svg></button></div>
          </div>
        </article>
      </div>
    </section>

    <aside class="rounded-2xl border border-border bg-muted/35 p-4 text-sm leading-6 text-muted-foreground"><strong class="text-foreground">数据保护：</strong>分类或标签删除后，历史账单仍会保留。接口拒绝操作时会展示具体原因。</aside>

    <Modal v-model="showForm" :title="`${editing ? '编辑' : '添加'}${currentTab.itemLabel}`">
      <form class="space-y-4" @submit.prevent="save">
        <label v-if="active === 'categories'" class="form-field"><span>上级分类 <small>可选</small></span><select ref="parentSelect" v-model.number="form.parent_id" class="input-field" @change="syncKindFromParent"><option :value="0">无，作为一级分类</option><option v-for="item in availableParents" :key="item.id" :value="item.id">{{ item.kind === 'income' ? '收入' : '支出' }} · {{ item.name }}</option></select><small class="text-xs text-muted-foreground">选择上级后会自动继承其收支类型，并作为二级分类。</small></label>
        <label class="form-field"><span>{{ active === 'categories' ? '分类名称' : '标签名称' }}</span><input ref="nameInput" v-model="form.name" required maxlength="40" class="input-field" :placeholder="currentTab.placeholder" /></label>
        <fieldset v-if="active === 'categories'"><legend class="text-sm font-semibold">收支类型</legend><div class="mt-2 grid grid-cols-2 gap-2 rounded-xl bg-muted p-1" role="radiogroup"><button type="button" role="radio" :disabled="kindLocked" :aria-checked="form.kind === 'expense'" :class="form.kind === 'expense' ? 'bg-surface text-rose-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold disabled:opacity-50" @click="form.kind = 'expense'">支出</button><button type="button" role="radio" :disabled="kindLocked" :aria-checked="form.kind === 'income'" :class="form.kind === 'income' ? 'bg-surface text-emerald-600 shadow-sm' : 'text-muted-foreground'" class="min-h-11 rounded-lg text-sm font-semibold disabled:opacity-50" @click="form.kind = 'income'">收入</button></div><small v-if="form.parent_id" class="mt-1 block text-xs text-muted-foreground">二级分类的收支类型跟随上级分类。</small><small v-else-if="editing?.transaction_count" class="mt-1 block text-xs text-muted-foreground">已有账单使用时不能修改收支类型。</small></fieldset>
        <p v-if="formError" role="alert" class="text-sm text-rose-600">{{ formError }}</p>
        <div class="flex gap-3 pt-2"><button type="button" class="btn-ghost min-h-11 flex-1" @click="showForm = false">取消</button><button class="btn-brand min-h-11 flex-1" :disabled="saving">{{ saving ? '保存中' : '保存' }}</button></div>
      </form>
    </Modal>
    <ConfirmDialog v-model="showDelete" :title="`删除${currentTab.itemLabel}？`" :message="deleteMessage" confirm-text="确认删除" confirm-type="danger" @confirm="deleteItem" />
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { billingApi, type BillKind, type PersonalCategory, type PersonalTag } from '../api/billing'
import { apiMessage } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import Toast from '../components/Toast.vue'

type ResourceKey = 'categories' | 'tags'
type ResourceItem = (PersonalCategory | PersonalTag) & { kind?: BillKind; parent_id?: number; parent_name?: string; transaction_count?: number }
const tabs = [
  { key: 'categories' as const, label: '分类', title: '分类管理', itemLabel: '分类', description: '支持一级、二级收入和支出分类；删除分类不会影响历史账单。', placeholder: '例如：宠物用品' },
  { key: 'tags' as const, label: '标签', title: '账单标签', itemLabel: '标签', description: '标签可跨账本与分类使用，适合项目、人物或场景维度。', placeholder: '例如：出差' },
]
const active = ref<ResourceKey>('categories')
const categories = ref<PersonalCategory[]>([]), tags = ref<PersonalTag[]>([])
const loading = ref(false), saving = ref(false), showForm = ref(false), showDelete = ref(false)
const editing = ref<ResourceItem | null>(null), deleting = ref<ResourceItem | null>(null), nameInput = ref<HTMLInputElement | null>(null), parentSelect = ref<HTMLSelectElement | null>(null)
const form = reactive({ name: '', kind: 'expense' as BillKind, parent_id: 0 })
const formError = ref('')
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const currentTab = computed(() => tabs.find(tab => tab.key === active.value)!)
const orderedCategories = computed(() => {
  const result: PersonalCategory[] = []
  for (const kind of ['expense', 'income'] as BillKind[]) {
    const roots = categories.value.filter(item => item.kind === kind && !item.parent_id).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
    for (const root of roots) {
      result.push(root, ...categories.value.filter(item => item.parent_id === root.id).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')))
    }
  }
  return result
})
const currentItems = computed<ResourceItem[]>(() => active.value === 'categories' ? orderedCategories.value : tags.value)
const availableParents = computed(() => categories.value.filter(item => !item.parent_id && item.id !== editing.value?.id && (!editing.value?.transaction_count || item.kind === editing.value.kind)))
const kindLocked = computed(() => Boolean(form.parent_id || editing.value?.transaction_count))
const deleteMessage = computed(() => { const item = deleting.value; if (!item) return ''; const usage = item.transaction_count ? `当前关联 ${item.transaction_count} 笔账单，` : ''; return `${usage}删除后无法恢复，历史账单本身不会被删除。` })

function listData<T>(response: any): T[] { const data = response.data?.data; return Array.isArray(data) ? data : data?.items || [] }
function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
function itemMeta(item: ResourceItem) { const usage = item.transaction_count !== undefined ? `${item.transaction_count} 笔账单正在使用` : active.value === 'tags' ? '可跨账本使用' : '自定义分类'; return active.value === 'categories' && item.parent_name ? `${item.parent_name} / ${item.name} · ${usage}` : usage }
async function load() { loading.value = true; try { const [categoryRes, tagRes] = await Promise.all([billingApi.personalCategories(), billingApi.personalTags()]); categories.value = listData(categoryRes); tags.value = listData(tagRes) } catch (error) { notify(apiMessage(error, '资料加载失败'), 'error') } finally { loading.value = false } }
async function focusFirstField() { await nextTick(); await nextTick(); if (active.value === 'categories') parentSelect.value?.focus(); else nameInput.value?.focus() }
function syncKindFromParent() { const parent = categories.value.find(item => item.id === form.parent_id); if (parent) form.kind = parent.kind }
function openCreate() { editing.value = null; Object.assign(form, { name: '', kind: 'expense', parent_id: 0 }); formError.value = ''; showForm.value = true; focusFirstField() }
function openEdit(item: ResourceItem) { editing.value = item; Object.assign(form, { name: item.name, kind: item.kind || 'expense', parent_id: item.parent_id || 0 }); formError.value = ''; showForm.value = true; focusFirstField() }
async function save() { const name = form.name.trim(); if (!name) { formError.value = '请输入名称'; return } saving.value = true; formError.value = ''; try { const id = editing.value?.id; if (active.value === 'categories') id ? await billingApi.personalCategoryUpdate(id, { name, kind: form.kind, parent_id: form.parent_id }) : await billingApi.personalCategoryCreate({ name, kind: form.kind, parent_id: form.parent_id }); else id ? await billingApi.personalTagUpdate(id, { name }) : await billingApi.personalTagCreate({ name }); showForm.value = false; notify(`${currentTab.value.itemLabel}已${id ? '更新' : '添加'}`); await load() } catch (error) { formError.value = apiMessage(error) } finally { saving.value = false } }
function askDelete(item: ResourceItem) { deleting.value = item; showDelete.value = true }
async function deleteItem() { if (!deleting.value) return; try { const id = deleting.value.id; if (active.value === 'categories') await billingApi.personalCategoryDelete(id); else await billingApi.personalTagDelete(id); notify(`${currentTab.value.itemLabel}已删除`); await load() } catch (error) { notify(apiMessage(error, '删除失败'), 'error') } }
watch(active, () => { showForm.value = false; showDelete.value = false; editing.value = null; deleting.value = null })
onMounted(load)
</script>
