<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="我的账本" description="为不同生活场景建立独立账本，并用封面和顺序快速找到它们。">
      <template #actions>
        <button type="button" class="btn-brand min-h-11" @click="openCreate">
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v14m-7-7h14" /></svg>
          新增账本
        </button>
      </template>
    </PageHeader>

    <section class="surface grid gap-3 rounded-[1.25rem] p-4 sm:grid-cols-[1fr_auto] sm:items-center sm:gap-4 sm:p-6">
      <div>
        <p class="font-display text-lg font-bold">一本账，记录一种生活</p>
        <p class="mt-1 text-sm leading-6 text-muted-foreground">拖动账本卡片即可排序；触屏或键盘用户可以使用卡片下方的前移、后移按钮。</p>
      </div>
      <dl class="grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-muted/70 px-4 py-3 text-center"><dt class="text-xs text-muted-foreground">使用中</dt><dd class="mt-1 font-mono text-xl font-semibold tabular-nums">{{ activeLedgers.length }}</dd></div>
        <div class="rounded-2xl bg-muted/70 px-4 py-3 text-center"><dt class="text-xs text-muted-foreground">已归档</dt><dd class="mt-1 font-mono text-xl font-semibold tabular-nums">{{ archivedLedgers.length }}</dd></div>
      </dl>
    </section>

    <section aria-labelledby="active-ledgers-title">
      <div class="mb-3 flex items-center justify-between gap-3">
        <div><h2 id="active-ledgers-title" class="font-display text-xl font-bold">使用中的账本</h2><p class="mt-1 text-xs text-muted-foreground">排序会同步到个人账单筛选和总览。</p></div>
        <span v-if="reordering" role="status" class="text-xs font-semibold text-brand-600 dark:text-brand-300">正在保存顺序…</span>
      </div>

      <div v-if="loading" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 3" :key="n" class="surface h-72 animate-pulse rounded-[1.25rem] bg-muted/60"></div>
      </div>
      <div v-else-if="!activeLedgers.length" class="empty-state surface min-h-64 rounded-[1.25rem] border-dashed">
        <svg class="h-10 w-10 text-muted-foreground/60" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.7" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6" /></svg>
        <strong class="mt-3 text-foreground">还没有可用账本</strong><span class="mt-1">新增一本账本开始记录</span>
      </div>
      <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="(ledger, index) in activeLedgers"
          :key="ledger.id"
          :draggable="activeLedgers.length > 1 && !reordering"
          :aria-label="`${ledger.name}，第 ${index + 1} 位`"
          :class="dragOverId === ledger.id ? 'ring-4 ring-brand-500/25' : ''"
          class="ledger-card surface group overflow-hidden rounded-[1.25rem] transition duration-200"
          @dragstart="startDrag($event, ledger.id)"
          @dragend="endDrag"
          @dragover.prevent="dragOverId = ledger.id"
          @drop.prevent="dropLedger(ledger.id)"
        >
          <RouterLink :to="ledgerBillsTo(ledger)" :draggable="false" class="block" :aria-label="`查看${ledger.name}下的账单`">
            <div class="ledger-cover" :style="coverStyle(ledger.cover)" role="img" :aria-label="`${ledger.name}账本封面`">
              <div class="ledger-cover-grid" aria-hidden="true"></div>
              <span class="ledger-index">{{ String(index + 1).padStart(2, '0') }}</span>
              <svg class="ledger-mark" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.7" d="M6 3h12v18l-3-2-3 2-3-2-3 2V3Zm3 5h6m-6 4h6" /></svg>
              <div class="ledger-cover-title"><span class="font-mono text-[10px] uppercase tracking-[0.18em] opacity-75">Personal ledger</span><strong>{{ ledger.name }}</strong><span class="ledger-open">查看账单 <svg fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m9 18 6-6-6-6" /></svg></span></div>
            </div>
          </RouterLink>
          <div class="p-4">
            <div class="flex min-w-0 items-start justify-between gap-3">
              <div class="min-w-0"><div class="flex flex-wrap items-center gap-2"><h3 class="truncate font-display text-lg font-bold"><RouterLink :to="ledgerBillsTo(ledger)" class="rounded-sm hover:text-brand-600 dark:hover:text-accent">{{ ledger.name }}</RouterLink></h3><span v-if="ledger.is_default" class="badge bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-accent">默认</span></div><p class="mt-1 text-xs text-muted-foreground">{{ ledger.currency || 'CNY' }} · {{ ledger.transaction_count || 0 }} 笔账单</p></div>
              <button type="button" class="icon-button shrink-0" :aria-label="`编辑${ledger.name}`" @click="openEdit(ledger)"><svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 20h4L19 9l-4-4L4 16v4z" /></svg></button>
            </div>
            <div class="mt-4 flex items-center justify-between border-t border-border pt-3">
              <div class="flex items-center gap-1" aria-label="调整账本顺序">
                <button type="button" class="icon-button" :disabled="index === 0 || reordering" :aria-label="`${ledger.name}向前移动`" @click="moveLedger(index, -1)"><svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m15 18-6-6 6-6" /></svg></button>
                <button type="button" class="icon-button" :disabled="index === activeLedgers.length - 1 || reordering" :aria-label="`${ledger.name}向后移动`" @click="moveLedger(index, 1)"><svg fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m9 18 6-6-6-6" /></svg></button>
                <span class="hidden cursor-grab items-center gap-1 px-2 text-xs text-muted-foreground lg:flex"><svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-width="2" d="M8 6h.01M8 12h.01M8 18h.01M16 6h.01M16 12h.01M16 18h.01" /></svg>拖动</span>
              </div>
              <button type="button" class="min-h-11 rounded-full px-3 text-sm font-bold text-rose-600 disabled:pointer-events-none disabled:opacity-35" :disabled="ledger.is_default" :title="ledger.is_default ? '默认账本不能归档' : undefined" @click="askArchive(ledger)">归档</button>
            </div>
          </div>
        </article>
      </div>
    </section>

    <section v-if="archivedLedgers.length" class="surface rounded-[1.25rem] p-4 sm:p-6" aria-labelledby="archived-ledgers-title">
      <div><h2 id="archived-ledgers-title" class="font-display text-lg font-bold">已归档账本</h2><p class="mt-1 text-sm text-muted-foreground">历史账单仍会保留，恢复后会排在使用中账本的最后。</p></div>
      <div class="mt-4 divide-y divide-border">
        <div v-for="ledger in archivedLedgers" :key="ledger.id" class="flex min-h-16 items-center gap-3 py-3">
          <span class="h-11 w-16 shrink-0 rounded-xl border border-white/10" :style="coverStyle(ledger.cover)" aria-hidden="true"></span>
          <div class="min-w-0 flex-1"><p class="truncate font-bold">{{ ledger.name }}</p><p class="text-xs text-muted-foreground">{{ ledger.transaction_count || 0 }} 笔账单</p></div>
          <button type="button" class="btn-ghost min-h-11 px-4" @click="restoreLedger(ledger)">恢复</button>
        </div>
      </div>
    </section>

    <Modal v-model="showForm" :title="editing ? '编辑账本' : '新增账本'">
      <form class="space-y-5" @submit.prevent="saveLedger">
        <label class="form-field"><span>账本名称</span><input ref="nameInput" v-model.trim="form.name" required maxlength="100" class="input-field" placeholder="例如：旅行账本" /></label>
        <label class="form-field"><span>币种</span><select v-model="form.currency" required class="input-field"><option value="CNY">人民币 CNY</option></select><small>个人账本当前统一使用人民币统计。</small></label>

        <fieldset>
          <legend class="text-sm font-semibold">账本封面</legend>
          <div class="mt-2 grid grid-cols-3 gap-2" role="radiogroup" aria-label="选择封面样式">
            <button v-for="preset in coverPresets" :key="preset.key" type="button" role="radio" :aria-checked="!coverFile && form.cover === preset.key" :aria-label="preset.label" :class="!coverFile && form.cover === preset.key ? 'ring-4 ring-brand-500/30' : 'hover:-translate-y-0.5'" class="h-16 rounded-xl border border-border transition duration-200" :style="coverStyle(preset.key)" @click="selectPreset(preset.key)"><span class="sr-only">{{ preset.label }}</span></button>
          </div>
          <label class="mt-3 flex min-h-12 cursor-pointer items-center justify-center gap-2 rounded-2xl border border-dashed border-border bg-muted/35 px-4 text-sm font-bold transition-colors hover:border-brand-400 hover:bg-muted">
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4-4 4 4 4-5 4 5M5 20h14a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2Z" /></svg>
            {{ hasCustomCover ? '更换自定义封面' : '上传自定义封面' }}
            <input type="file" accept="image/jpeg,image/png,image/webp" class="sr-only" @change="onCoverFile" />
          </label>
          <p class="mt-1 text-xs text-muted-foreground">支持 JPEG、PNG、WebP，建议横向图片，最大 10 MB。</p>
          <div v-if="coverPreview" class="mt-3 h-32 overflow-hidden rounded-2xl border border-border" :style="coverPreview" role="img" aria-label="自定义封面预览"></div>
        </fieldset>

        <p v-if="formError" role="alert" class="text-sm font-medium text-rose-600">{{ formError }}</p>
        <div class="flex gap-3 pt-1"><button type="button" class="btn-ghost min-h-11 flex-1" @click="showForm = false">取消</button><button type="submit" class="btn-brand min-h-11 flex-1" :disabled="saving">{{ saving ? (coverFile ? '上传并保存中…' : '保存中…') : '保存账本' }}</button></div>
      </form>
    </Modal>

    <ConfirmDialog v-model="showArchive" title="归档账本？" :message="archiveMessage" confirm-text="确认归档" confirm-type="danger" @confirm="archiveLedger" />
    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { billingApi, type PersonalLedger } from '../api/billing'
import { apiMessage } from '../utils/billing'
import PageHeader from '../components/PageHeader.vue'
import Modal from '../components/Modal.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import Toast from '../components/Toast.vue'

const coverPresets = [
  { key: 'coral', label: '珊瑚暖阳' },
  { key: 'ocean', label: '深海蓝' },
  { key: 'forest', label: '森林绿' },
  { key: 'violet', label: '暮光紫' },
  { key: 'amber', label: '琥珀金' },
  { key: 'slate', label: '石墨灰' },
] as const

const presetBackgrounds: Record<string, string> = {
  coral: 'linear-gradient(135deg, #ff9b76 0%, #f47052 48%, #b53f4f 100%)',
  ocean: 'linear-gradient(135deg, #4f7cac 0%, #252d5b 52%, #151b3b 100%)',
  forest: 'linear-gradient(135deg, #69a887 0%, #26785e 48%, #15483c 100%)',
  violet: 'linear-gradient(135deg, #a78bdb 0%, #665191 50%, #37294f 100%)',
  amber: 'linear-gradient(135deg, #f5c66b 0%, #cc7a39 52%, #79452e 100%)',
  slate: 'linear-gradient(135deg, #7a8295 0%, #3b4051 50%, #202330 100%)',
}

const ledgers = ref<PersonalLedger[]>([])
const loading = ref(false)
const saving = ref(false)
const reordering = ref(false)
const showForm = ref(false)
const showArchive = ref(false)
const editing = ref<PersonalLedger | null>(null)
const archiving = ref<PersonalLedger | null>(null)
const nameInput = ref<HTMLInputElement | null>(null)
const draggedId = ref<number | null>(null)
const dragOverId = ref<number | null>(null)
const coverFile = ref<File | null>(null)
const coverFilePreview = ref('')
const form = reactive({ name: '', currency: 'CNY', cover: 'coral' })
const formError = ref('')
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })

const activeLedgers = computed(() => ledgers.value.filter(item => !item.archived))
const archivedLedgers = computed(() => ledgers.value.filter(item => item.archived))
const hasCustomCover = computed(() => Boolean(coverFile.value || form.cover.startsWith('/uploads/')))
const coverPreview = computed(() => coverFilePreview.value ? imageCoverStyle(coverFilePreview.value) : form.cover.startsWith('/uploads/') ? coverStyle(form.cover) : null)
const archiveMessage = computed(() => archiving.value ? `归档“${archiving.value.name}”后将不能继续记账，但历史账单和统计都会保留。` : '')

function listData<T>(response: any): T[] { const data = response.data?.data; return Array.isArray(data) ? data : data?.items || [] }
function notify(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => { toast.message = message; toast.type = type }) }
function assetUrl(path: string) { return `${import.meta.env.BASE_URL}${path.replace(/^\/+/, '')}` }
function imageCoverStyle(url: string) { return { backgroundImage: `linear-gradient(135deg, rgba(17,18,26,.15), rgba(17,18,26,.5)), url("${url}")`, backgroundPosition: 'center', backgroundSize: 'cover' } }
function coverStyle(cover = 'coral') { return cover.startsWith('/uploads/') ? imageCoverStyle(assetUrl(cover)) : { background: presetBackgrounds[cover] || presetBackgrounds.coral } }
function ledgerBillsTo(ledger: PersonalLedger) { return { path: '/admin/personal', query: { ledger_id: String(ledger.id) } } }
function clearFilePreview() { if (coverFilePreview.value) URL.revokeObjectURL(coverFilePreview.value); coverFilePreview.value = ''; coverFile.value = null }

async function load() {
  loading.value = true
  try { ledgers.value = listData<PersonalLedger>(await billingApi.personalLedgers(true)) }
  catch (error) { notify(apiMessage(error, '账本加载失败'), 'error') }
  finally { loading.value = false }
}

async function focusName() { await nextTick(); nameInput.value?.focus() }
function openCreate() { editing.value = null; clearFilePreview(); Object.assign(form, { name: '', currency: 'CNY', cover: 'coral' }); formError.value = ''; showForm.value = true; focusName() }
function openEdit(ledger: PersonalLedger) { editing.value = ledger; clearFilePreview(); Object.assign(form, { name: ledger.name, currency: ledger.currency || 'CNY', cover: ledger.cover || 'coral' }); formError.value = ''; showForm.value = true; focusName() }
function selectPreset(cover: string) { clearFilePreview(); form.cover = cover }
function onCoverFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) { formError.value = '封面仅支持 JPEG、PNG 或 WebP 图片'; return }
  if (file.size > 10 * 1024 * 1024) { formError.value = '封面图片不能超过 10 MB'; return }
  clearFilePreview()
  coverFile.value = file
  coverFilePreview.value = URL.createObjectURL(file)
  formError.value = ''
}

async function saveLedger() {
  const name = form.name.trim()
  if (!name) { formError.value = '请输入账本名称'; return }
  saving.value = true
  formError.value = ''
  try {
    let cover = form.cover
    if (coverFile.value) {
      const uploaded = await billingApi.uploadImage(coverFile.value)
      cover = uploaded.data?.data?.path
      if (!cover) throw new Error('封面上传失败')
    }
    const payload = { name, currency: form.currency, cover }
    if (editing.value) await billingApi.personalLedgerUpdate(editing.value.id, payload)
    else await billingApi.personalLedgerCreate(payload)
    showForm.value = false
    notify(editing.value ? '账本已更新' : '账本已创建')
    await load()
  } catch (error) { formError.value = apiMessage(error, error instanceof Error ? error.message : '账本保存失败') }
  finally { saving.value = false }
}

async function persistOrder(next: PersonalLedger[]) {
  const previous = [...activeLedgers.value]
  const archived = [...archivedLedgers.value]
  ledgers.value = [...next.map((item, index) => ({ ...item, sort_order: index })), ...archived]
  reordering.value = true
  try { await billingApi.personalLedgerOrder(next.map(item => item.id)); notify('账本顺序已保存') }
  catch (error) { ledgers.value = [...previous, ...archived]; notify(apiMessage(error, '顺序保存失败'), 'error') }
  finally { reordering.value = false }
}
function moveLedger(index: number, direction: -1 | 1) { const next = [...activeLedgers.value]; const target = index + direction; if (target < 0 || target >= next.length) return; [next[index], next[target]] = [next[target], next[index]]; persistOrder(next) }
function startDrag(event: DragEvent, id: number) { if ((event.target as HTMLElement).closest('button')) { event.preventDefault(); return }; draggedId.value = id; event.dataTransfer?.setData('text/plain', String(id)); if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move' }
function endDrag() { draggedId.value = null; dragOverId.value = null }
function dropLedger(targetId: number) { const sourceId = draggedId.value; endDrag(); if (!sourceId || sourceId === targetId || reordering.value) return; const next = [...activeLedgers.value]; const sourceIndex = next.findIndex(item => item.id === sourceId); const targetIndex = next.findIndex(item => item.id === targetId); if (sourceIndex < 0 || targetIndex < 0) return; const [moved] = next.splice(sourceIndex, 1); next.splice(targetIndex, 0, moved); persistOrder(next) }

function askArchive(ledger: PersonalLedger) { archiving.value = ledger; showArchive.value = true }
async function archiveLedger() { if (!archiving.value) return; try { await billingApi.personalLedgerDelete(archiving.value.id); notify('账本已归档'); await load() } catch (error) { notify(apiMessage(error, '账本归档失败'), 'error') } }
async function restoreLedger(ledger: PersonalLedger) { try { await billingApi.personalLedgerRestore(ledger.id); notify('账本已恢复'); await load() } catch (error) { notify(apiMessage(error, '账本恢复失败'), 'error') } }

watch(showForm, value => { if (!value) clearFilePreview() })
onMounted(load)
</script>

<style scoped>
.ledger-card[draggable="true"] { cursor: grab; }
.ledger-card[draggable="true"]:active { cursor: grabbing; }
.ledger-card:hover { transform: translateY(-2px); box-shadow: 0 18px 44px rgb(var(--color-foreground) / .09); }
.ledger-cover { position: relative; min-height: 10rem; overflow: hidden; color: white; isolation: isolate; }
.ledger-cover::after { content: ''; position: absolute; inset: 0; z-index: -1; background: linear-gradient(145deg, transparent 35%, rgba(0,0,0,.22)); }
.ledger-cover-grid { position: absolute; inset: 0; opacity: .22; background-image: linear-gradient(rgba(255,255,255,.22) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.14) 1px, transparent 1px); background-size: 28px 28px; mask-image: linear-gradient(135deg, #000, transparent 72%); }
.ledger-index { position: absolute; left: 1rem; top: .9rem; font-family: 'IBM Plex Mono', monospace; font-size: .7rem; letter-spacing: .14em; opacity: .78; }
.ledger-mark { position: absolute; right: 1rem; top: .9rem; width: 1.6rem; height: 1.6rem; opacity: .9; }
.ledger-cover-title { position: absolute; inset: auto 1rem 1rem; display: flex; flex-direction: column; gap: .2rem; text-shadow: 0 2px 12px rgba(0,0,0,.3); }
.ledger-cover-title strong { max-width: 90%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: 'Manrope', sans-serif; font-size: 1.35rem; font-weight: 800; letter-spacing: -.025em; }
.ledger-open { display: inline-flex; min-height: 2.75rem; align-items: center; gap: .25rem; align-self: flex-start; margin-top: .2rem; font-size: .75rem; font-weight: 700; }
.ledger-open svg { width: 1rem; height: 1rem; transition: transform 180ms ease; }
a:hover .ledger-open svg { transform: translateX(3px); }
.icon-button:disabled { pointer-events: none; opacity: .3; }
@media (prefers-reduced-motion: reduce) { .ledger-card:hover { transform: none; } }
@media (max-width: 639px) {
  .ledger-cover { min-height: 7.5rem; }
  .ledger-cover-title { bottom: .65rem; }
  .ledger-cover-title strong { font-size: 1.1rem; }
  .ledger-open { min-height: 2rem; }
}
</style>
