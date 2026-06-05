<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import { opsAPI, type OpsRequestDetailsParams, type OpsRequestDetail } from '@/api/admin/ops'
import { parseTimeRangeMinutes, formatDateTime } from '../utils/opsFormatters'

export interface OpsRequestDetailsPreset {
  title: string
  kind?: OpsRequestDetailsParams['kind']
  sort?: OpsRequestDetailsParams['sort']
  min_duration_ms?: number
  max_duration_ms?: number
}

interface Props {
  modelValue: boolean
  timeRange: string
  preset: OpsRequestDetailsPreset
  platform?: string
  groupId?: number | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'openErrorDetail', errorId: number): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const items = ref<OpsRequestDetail[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const filters = ref({
  startLocal: '',
  endLocal: '',
  kind: 'all' as NonNullable<OpsRequestDetailsParams['kind']>,
  sort: 'created_at_desc' as NonNullable<OpsRequestDetailsParams['sort']>,
  userId: '',
  apiKeyId: '',
  accountId: '',
  requestId: '',
  model: '',
  query: ''
})

const close = () => emit('update:modelValue', false)

const rangeLabel = computed(() => {
  const minutes = parseTimeRangeMinutes(props.timeRange)
  if (minutes >= 60) return t('admin.ops.requestDetails.rangeHours', { n: Math.round(minutes / 60) })
  return t('admin.ops.requestDetails.rangeMinutes', { n: minutes })
})

function buildTimeParams(): Pick<OpsRequestDetailsParams, 'start_time' | 'end_time'> {
  if (filters.value.startLocal || filters.value.endLocal) {
    return {
      start_time: localDateTimeToISOString(filters.value.startLocal),
      end_time: localDateTimeToISOString(filters.value.endLocal)
    }
  }

  const minutes = parseTimeRangeMinutes(props.timeRange)
  const endTime = new Date()
  const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)
  return {
    start_time: startTime.toISOString(),
    end_time: endTime.toISOString()
  }
}

function formatLocalDateTime(date: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function localDateTimeToISOString(value: string) {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function resetFilters() {
  const minutes = parseTimeRangeMinutes(props.timeRange)
  const endTime = new Date()
  const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)
  filters.value = {
    startLocal: formatLocalDateTime(startTime),
    endLocal: formatLocalDateTime(endTime),
    kind: props.preset.kind ?? 'all',
    sort: props.preset.sort ?? 'created_at_desc',
    userId: '',
    apiKeyId: '',
    accountId: '',
    requestId: '',
    model: '',
    query: ''
  }
}

function parsePositiveInt(value: string): number | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  const parsed = Number.parseInt(trimmed, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
}

const fetchData = async () => {
  if (!props.modelValue) return
  loading.value = true
  try {
    const params: OpsRequestDetailsParams = {
      ...buildTimeParams(),
      page: page.value,
      page_size: pageSize.value,
      kind: filters.value.kind,
      sort: filters.value.sort
    }

    const platform = (props.platform || '').trim()
    if (platform) params.platform = platform
    if (typeof props.groupId === 'number' && props.groupId > 0) params.group_id = props.groupId

    if (typeof props.preset.min_duration_ms === 'number') params.min_duration_ms = props.preset.min_duration_ms
    if (typeof props.preset.max_duration_ms === 'number') params.max_duration_ms = props.preset.max_duration_ms

    const userId = parsePositiveInt(filters.value.userId)
    const apiKeyId = parsePositiveInt(filters.value.apiKeyId)
    const accountId = parsePositiveInt(filters.value.accountId)
    if (userId) params.user_id = userId
    if (apiKeyId) params.api_key_id = apiKeyId
    if (accountId) params.account_id = accountId
    if (filters.value.requestId.trim()) params.request_id = filters.value.requestId.trim()
    if (filters.value.model.trim()) params.model = filters.value.model.trim()
    if (filters.value.query.trim()) params.q = filters.value.query.trim()

    const res = await opsAPI.listRequestDetails(params)
    items.value = res.items || []
    total.value = res.total || 0
  } catch (e: any) {
    console.error('[OpsRequestDetailsModal] Failed to fetch request details', e)
    appStore.showError(e?.message || t('admin.ops.requestDetails.failedToLoad'))
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      page.value = 1
      pageSize.value = 10
      resetFilters()
      fetchData()
    }
  }
)

watch(
  () => [
    props.timeRange,
    props.platform,
    props.groupId,
    props.preset.kind,
    props.preset.sort,
    props.preset.min_duration_ms,
    props.preset.max_duration_ms
  ],
  () => {
    if (!props.modelValue) return
    page.value = 1
    fetchData()
  }
)

function handlePageChange(next: number) {
  page.value = next
  fetchData()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  fetchData()
}

function applyFilters() {
  page.value = 1
  fetchData()
}

function handleResetFilters() {
  page.value = 1
  resetFilters()
  fetchData()
}

async function handleCopyRequestId(requestId: string) {
  const ok = await copyToClipboard(requestId, t('admin.ops.requestDetails.requestIdCopied'))
  if (ok) return
  // `useClipboard` already shows toast on failure; this keeps UX consistent with older ops modal.
  appStore.showWarning(t('admin.ops.requestDetails.copyFailed'))
}

function openErrorDetail(errorId: number | null | undefined) {
  if (!errorId) return
  close()
  emit('openErrorDetail', errorId)
}

const kindBadgeClass = (kind: string) => {
  if (kind === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
}

function formatMs(value: number | null | undefined) {
  return typeof value === 'number' ? `${value} ms` : '-'
}

function identityLine(id: number | null | undefined, name?: string) {
  if (!id && !name) return '-'
  return `${name || '-'}${id ? ` #${id}` : ''}`
}

function endpointPath(row: OpsRequestDetail) {
  const inbound = row.inbound_endpoint || '-'
  const upstream = row.upstream_endpoint || '-'
  return `${inbound} -> ${upstream}`
}

function modelPath(row: OpsRequestDetail) {
  const requested = row.requested_model || row.model || '-'
  const upstream = row.upstream_model || row.model || '-'
  return requested === upstream ? requested : `${requested} -> ${upstream}`
}

function problemLabel(row: OpsRequestDetail) {
  const pieces = [row.phase, row.error_owner, row.error_source, row.severity].filter(Boolean)
  return pieces.length ? pieces.join(' / ') : '-'
}
</script>

<template>
  <BaseDialog :show="modelValue" :title="props.preset.title || t('admin.ops.requestDetails.title')" width="full" @close="close">
    <template #default>
      <div class="flex h-full min-h-0 flex-col">
        <div class="mb-4 flex flex-shrink-0 flex-col gap-3">
          <div class="flex items-center justify-between gap-3">
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.ops.requestDetails.rangeLabel', { range: rangeLabel }) }}
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="handleResetFilters">重置</button>
              <button type="button" class="btn btn-primary btn-sm" @click="applyFilters">查询</button>
              <button type="button" class="btn btn-secondary btn-sm" @click="fetchData">
                {{ t('common.refresh') }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900 md:grid-cols-2 xl:grid-cols-4">
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              开始时间
              <input v-model="filters.startLocal" type="datetime-local" step="1" class="input input-sm w-full font-mono text-xs" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              结束时间
              <input v-model="filters.endLocal" type="datetime-local" step="1" class="input input-sm w-full font-mono text-xs" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              类型
              <select v-model="filters.kind" class="input input-sm w-full text-xs">
                <option value="all">全部</option>
                <option value="success">成功</option>
                <option value="error">失败</option>
              </select>
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              排序
              <select v-model="filters.sort" class="input input-sm w-full text-xs">
                <option value="created_at_desc">时间倒序</option>
                <option value="duration_desc">耗时倒序</option>
              </select>
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              用户 ID
              <input v-model="filters.userId" type="number" min="1" class="input input-sm w-full text-xs" placeholder="user_id" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              API Key ID
              <input v-model="filters.apiKeyId" type="number" min="1" class="input input-sm w-full text-xs" placeholder="api_key_id" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              账号 ID
              <input v-model="filters.accountId" type="number" min="1" class="input input-sm w-full text-xs" placeholder="account_id" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400">
              关键词
              <input v-model.trim="filters.query" type="search" class="input input-sm w-full text-xs" placeholder="请求ID / 邮箱 / 账号 / endpoint" @keydown.enter="applyFilters" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400 xl:col-span-2">
              请求 ID
              <input v-model.trim="filters.requestId" type="search" class="input input-sm w-full font-mono text-xs" placeholder="request_id" @keydown.enter="applyFilters" />
            </label>
            <label class="flex flex-col gap-1 text-xs font-medium text-gray-500 dark:text-gray-400 xl:col-span-2">
              模型
              <input v-model.trim="filters.model" type="search" class="input input-sm w-full text-xs" placeholder="model" @keydown.enter="applyFilters" />
            </label>
          </div>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex flex-1 items-center justify-center py-16">
          <div class="flex flex-col items-center gap-3">
            <svg class="h-8 w-8 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            <span class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
          </div>
        </div>

        <!-- Table -->
        <div v-else class="flex min-h-0 flex-1 flex-col">
          <div v-if="items.length === 0" class="rounded-xl border border-dashed border-gray-200 p-10 text-center dark:border-dark-700">
            <div class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ t('admin.ops.requestDetails.empty') }}</div>
            <div class="mt-1 text-xs text-gray-400">{{ t('admin.ops.requestDetails.emptyHint') }}</div>
          </div>

          <div v-else class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
            <div class="min-h-0 flex-1 overflow-auto">
              <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="sticky top-0 z-10 bg-gray-50 dark:bg-dark-900">
                <tr>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.requestDetails.table.time') }}
                  </th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.requestDetails.table.kind') }}
                  </th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">用户 / Key</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">账号 / 分组</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">平台</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">模型链路</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">Endpoint 链路</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">质量</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">问题节点</th>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.requestDetails.table.requestId') }}
                  </th>
                  <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.requestDetails.table.actions') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
                <tr v-for="(row, idx) in items" :key="idx" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                  <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    {{ formatDateTime(row.created_at) }}
                  </td>
                  <td class="whitespace-nowrap px-4 py-3">
                    <span class="rounded-full px-2 py-1 text-[10px] font-bold" :class="kindBadgeClass(row.kind)">
                      {{ row.kind === 'error' ? t('admin.ops.requestDetails.kind.error') : t('admin.ops.requestDetails.kind.success') }}
                    </span>
                  </td>
                  <td class="min-w-[210px] px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div class="truncate text-gray-700 dark:text-gray-200" :title="row.user_email || ''">
                      {{ row.user_email || (row.user_id ? `用户 #${row.user_id}` : '-') }}
                    </div>
                    <div class="mt-1 truncate text-[11px] text-gray-400" :title="identityLine(row.api_key_id, row.api_key_name)">
                      {{ identityLine(row.api_key_id, row.api_key_name) }}
                    </div>
                  </td>
                  <td class="min-w-[180px] px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div class="truncate text-gray-700 dark:text-gray-200" :title="identityLine(row.account_id, row.account_name)">
                      {{ identityLine(row.account_id, row.account_name) }}
                    </div>
                    <div class="mt-1 truncate text-[11px] text-gray-400" :title="identityLine(row.group_id, row.group_name)">
                      {{ identityLine(row.group_id, row.group_name) }}
                    </div>
                  </td>
                  <td class="whitespace-nowrap px-4 py-3 text-xs font-medium text-gray-700 dark:text-gray-200">
                    {{ (row.platform || 'unknown').toUpperCase() }}
                  </td>
                  <td class="max-w-[260px] px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div class="truncate" :title="modelPath(row)">{{ modelPath(row) }}</div>
                    <div v-if="row.model_mapping_chain || row.billing_tier" class="mt-1 truncate text-[11px] text-gray-400" :title="row.model_mapping_chain || row.billing_tier">
                      {{ row.model_mapping_chain || row.billing_tier }}
                    </div>
                  </td>
                  <td class="max-w-[280px] px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div class="truncate font-mono text-[11px]" :title="endpointPath(row)">{{ endpointPath(row) }}</div>
                    <div class="mt-1 text-[11px] text-gray-400">
                      {{ row.stream ? 'stream' : 'sync' }}{{ row.request_type ? ` / type ${row.request_type}` : '' }}
                    </div>
                  </td>
                  <td class="min-w-[170px] whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div>总耗时 {{ formatMs(row.duration_ms) }}</div>
                    <div class="mt-1 text-[11px] text-gray-400">
                      TTFT {{ formatMs(row.time_to_first_token_ms ?? row.first_token_ms) }}
                    </div>
                    <div class="mt-1 text-[11px] text-gray-400">
                      上游 {{ formatMs(row.upstream_latency_ms) }} / 响应 {{ formatMs(row.response_latency_ms) }}
                    </div>
                  </td>
                  <td class="min-w-[190px] px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                    <div class="truncate" :title="problemLabel(row)">{{ problemLabel(row) }}</div>
                    <div class="mt-1 text-[11px] text-gray-400">
                      状态 {{ row.status_code ?? '-' }} / 上游 {{ row.upstream_status_code ?? '-' }}
                    </div>
                    <div v-if="row.message" class="mt-1 max-w-[220px] truncate text-[11px] text-gray-400" :title="row.message">
                      {{ row.message }}
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <div v-if="row.request_id" class="flex items-center gap-2">
                      <span class="max-w-[220px] truncate font-mono text-[11px] text-gray-700 dark:text-gray-200" :title="row.request_id">
                        {{ row.request_id }}
                      </span>
                      <button
                        class="rounded-md bg-gray-100 px-2 py-1 text-[10px] font-bold text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
                        @click="handleCopyRequestId(row.request_id)"
                      >
                        {{ t('admin.ops.requestDetails.copy') }}
                      </button>
                    </div>
                    <span v-else class="text-xs text-gray-400">-</span>
                  </td>
                  <td class="whitespace-nowrap px-4 py-3 text-right">
                    <button
                      v-if="row.kind === 'error' && row.error_id"
                      class="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-bold text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-300 dark:hover:bg-red-900/30"
                      @click="openErrorDetail(row.error_id)"
                    >
                      {{ t('admin.ops.requestDetails.viewError') }}
                    </button>
                    <span v-else class="text-xs text-gray-400">-</span>
                  </td>
                </tr>
              </tbody>
            </table>
            </div>

            <Pagination
              :total="total"
              :page="page"
              :page-size="pageSize"
              @update:page="handlePageChange"
              @update:pageSize="handlePageSizeChange"
            />
          </div>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>
