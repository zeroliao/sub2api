<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="grid gap-4 lg:grid-cols-3">
          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-dark-300">直连兜底</div>
            <div class="mt-3 flex items-center gap-3">
              <select v-model="settings.direct_fallback_mode" class="input max-w-48" @change="saveSettings">
                <option value="off">关闭</option>
                <option value="manual_only">仅手动</option>
                <option value="global">全局</option>
              </select>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                <input v-model="settings.auto_assign_enabled" type="checkbox" class="h-4 w-4" @change="saveSettings" />
                自动分配
              </label>
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-dark-300">导入预览</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ preview?.recommended || 0 }}</div>
            <div class="text-xs text-gray-500 dark:text-dark-400">推荐导入节点</div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-dark-300">安全策略</div>
            <div class="mt-2 text-sm text-gray-700 dark:text-gray-200">
              无可用代理时默认返回 503，不直接使用服务器 IP。
            </div>
          </div>
        </div>
      </template>

      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input
            v-model="filters.search"
            class="input w-full sm:w-64"
            placeholder="搜索账号"
            @input="loadRelationships"
          />
          <select v-model="filters.platform" class="input w-full sm:w-40" @change="loadRelationships">
            <option value="">全部平台</option>
            <option value="openai">OpenAI</option>
            <option value="anthropic">Claude</option>
            <option value="gemini">Gemini</option>
            <option value="antigravity">Antigravity</option>
          </select>
          <select v-model="filters.status" class="input w-full sm:w-40" @change="loadRelationships">
            <option value="">全部状态</option>
            <option value="active">active</option>
            <option value="error">error</option>
            <option value="disabled">disabled</option>
          </select>
          <button class="btn btn-secondary" :disabled="loading" @click="loadRelationships">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <button class="btn btn-primary" @click="showImport = true">
            <Icon name="plus" size="md" class="mr-2" />
            万能导入
          </button>
          <button class="btn btn-secondary" @click="showSubscriptions = true">
            订阅源
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="relationships" :loading="loading">
          <template #cell-account="{ row }">
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ row.account_name }}</div>
              <div class="text-xs text-gray-500">{{ row.platform }} / {{ row.account_type }}</div>
            </div>
          </template>

          <template #cell-proxy="{ row }">
            <div v-if="row.current_proxy">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.current_proxy.name }}</div>
              <code class="text-xs text-gray-500">{{ row.current_proxy.protocol }}://{{ row.current_proxy.host }}:{{ row.current_proxy.port }}</code>
            </div>
            <span v-else class="badge badge-danger">NO_AVAILABLE_PROXY</span>
          </template>

          <template #cell-source="{ row }">
            <span :class="['badge', row.proxy_source === 'manual' ? 'badge-primary' : 'badge-gray']">
              {{ row.proxy_source || '-' }}
            </span>
          </template>

          <template #cell-quality="{ row }">
            <div class="space-y-1 text-xs">
              <div>{{ row.current_proxy?.quality_status || '-' }}</div>
              <div class="text-gray-500">{{ row.current_proxy?.exit_ip || row.current_proxy?.region || '-' }}</div>
            </div>
          </template>

          <template #cell-load="{ row }">
            <div class="text-xs text-gray-700 dark:text-gray-200">
              <div>绑定 {{ row.bound_account_count }}</div>
              <div>活跃 {{ row.active_account_count }} / 并发 {{ row.current_concurrency }}</div>
            </div>
          </template>

          <template #cell-last_used="{ row }">
            <span class="text-xs text-gray-500">{{ formatDate(row.last_used_at) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-sm btn-secondary" @click="reassign(row.account_id)">重新分配</button>
              <button class="btn btn-sm btn-secondary" @click="restore(row.account_id)">恢复历史</button>
              <button class="btn btn-sm btn-secondary" @click="openHistory(row.account_id)">历史</button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="changePage"
          @update:pageSize="changePageSize"
        />
      </template>
    </TablePageLayout>

    <BaseDialog :show="showImport" title="万能导入代理" width="wide" @close="showImport = false">
      <div class="space-y-4">
        <textarea
          v-model="importContent"
          class="input min-h-[180px] font-mono text-xs"
          placeholder="粘贴 http/socks5、多行代理、Clash YAML、sing-box JSON、Clash/Mihomo 订阅 URL、ss/vmess/vless/trojan/hysteria2/tuic 节点"
        />
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" :disabled="importing" @click="previewImport">预览</button>
          <button class="btn btn-primary" :disabled="!preview || importing" @click="confirmImport">导入推荐节点</button>
        </div>
        <div v-if="preview" class="rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="border-b border-gray-200 p-3 text-sm dark:border-dark-700">
            共 {{ preview.total }}，可用 {{ preview.valid }}，重复 {{ preview.duplicates }}，需 sidecar {{ preview.sidecar_only }}
          </div>
          <div class="max-h-72 overflow-auto">
            <table class="w-full text-sm">
              <tbody>
                <tr v-for="item in preview.items" :key="item.key" class="border-b border-gray-100 dark:border-dark-800">
                  <td class="px-3 py-2">
                    <input v-model="item.selected" type="checkbox" :disabled="!item.valid || item.duplicate || item.sidecar_required" />
                  </td>
                  <td class="px-3 py-2">{{ item.name || item.host || item.protocol }}</td>
                  <td class="px-3 py-2">{{ item.protocol }}</td>
                  <td class="px-3 py-2">{{ item.host }}:{{ item.port || '-' }}</td>
                  <td class="px-3 py-2">
                    <span v-if="item.sidecar_required" class="badge badge-warning">sidecar</span>
                    <span v-else-if="item.duplicate" class="badge badge-gray">重复</span>
                    <span v-else-if="item.valid" class="badge badge-success">可导入</span>
                    <span v-else class="badge badge-danger">{{ item.error || '无效' }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showSubscriptions" title="代理订阅源" width="wide" @close="showSubscriptions = false">
      <div class="space-y-4">
        <div class="grid gap-3 md:grid-cols-4">
          <input v-model="subscriptionForm.name" class="input" placeholder="名称" />
          <input v-model="subscriptionForm.url" class="input md:col-span-2" placeholder="订阅 URL" />
          <button class="btn btn-primary" @click="createSubscription">新增订阅</button>
        </div>
        <div class="divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
          <div v-for="source in subscriptions" :key="source.id" class="flex items-center justify-between gap-3 p-3">
            <div class="min-w-0">
              <div class="font-medium text-gray-900 dark:text-white">{{ source.name }}</div>
              <div class="truncate text-xs text-gray-500">{{ source.url }}</div>
              <div v-if="source.last_error" class="text-xs text-red-500">{{ source.last_error }}</div>
            </div>
            <div class="flex gap-2">
              <button class="btn btn-sm btn-secondary" @click="syncSubscription(source.id)">同步预览</button>
              <button class="btn btn-sm btn-danger" @click="deleteSubscription(source.id)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showHistory" title="账号代理历史" width="wide" @close="showHistory = false">
      <div class="divide-y divide-gray-200 dark:divide-dark-700">
        <div v-for="item in history" :key="item.id" class="grid gap-2 py-3 text-sm md:grid-cols-5">
          <div>{{ item.proxy?.name || item.proxy_id }}</div>
          <div>{{ item.status }}</div>
          <div>{{ item.source }}</div>
          <div>{{ item.use_count }} 次</div>
          <div>{{ formatDate(item.last_used_at) }}</div>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AccountProxyBinding,
  ProxyDispatchSettings,
  ProxyImportPreview,
  ProxyRelationship,
  ProxySubscriptionSource
} from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const appStore = useAppStore()

const loading = ref(false)
const importing = ref(false)
const relationships = ref<ProxyRelationship[]>([])
const preview = ref<ProxyImportPreview | null>(null)
const subscriptions = ref<ProxySubscriptionSource[]>([])
const history = ref<AccountProxyBinding[]>([])
const showImport = ref(false)
const showSubscriptions = ref(false)
const showHistory = ref(false)
const importContent = ref('')
const settings = reactive<ProxyDispatchSettings>({
  direct_fallback_mode: 'off',
  auto_assign_enabled: true
})
const filters = reactive({ platform: '', status: '', search: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const subscriptionForm = reactive({ name: '', url: '', source_type: 'clash', sync_enabled: true, sync_interval_minutes: 1440, status: 'active' })

const columns = computed<Column[]>(() => [
  { key: 'account', label: '账号' },
  { key: 'account_status', label: '账号状态' },
  { key: 'proxy', label: '当前代理' },
  { key: 'source', label: '来源' },
  { key: 'quality', label: '代理状态' },
  { key: 'load', label: '负载' },
  { key: 'last_used', label: '最近使用' },
  { key: 'actions', label: '操作' }
])

async function loadRelationships() {
  loading.value = true
  try {
    const result = await adminAPI.proxies.listProxyRelationships(pagination.page, pagination.page_size, filters)
    relationships.value = result.items || []
    pagination.total = result.total || 0
  } catch (error: any) {
    appStore.showError(error?.message || '加载代理关系失败')
  } finally {
    loading.value = false
  }
}

async function loadSettings() {
  try {
    const result = await adminAPI.proxies.getProxyDispatchSettings()
    settings.direct_fallback_mode = result.direct_fallback_mode
    settings.auto_assign_enabled = result.auto_assign_enabled
  } catch {
    settings.direct_fallback_mode = 'off'
  }
}

async function saveSettings() {
  try {
    const result = await adminAPI.proxies.updateProxyDispatchSettings({ ...settings })
    settings.direct_fallback_mode = result.direct_fallback_mode
    settings.auto_assign_enabled = result.auto_assign_enabled
    appStore.showSuccess('分发策略已保存')
  } catch (error: any) {
    appStore.showError(error?.message || '保存分发策略失败')
  }
}

async function reassign(accountId: number) {
  try {
    await adminAPI.proxies.reassignAccountProxy(accountId)
    appStore.showSuccess('已重新分配代理')
    await loadRelationships()
  } catch (error: any) {
    appStore.showError(error?.message || '重新分配失败')
  }
}

async function restore(accountId: number) {
  try {
    await adminAPI.proxies.restoreAccountProxyHistory(accountId)
    appStore.showSuccess('已恢复历史代理')
    await loadRelationships()
  } catch (error: any) {
    appStore.showError(error?.message || '恢复历史代理失败')
  }
}

async function openHistory(accountId: number) {
  try {
    history.value = await adminAPI.proxies.getAccountProxyHistory(accountId)
    showHistory.value = true
  } catch (error: any) {
    appStore.showError(error?.message || '加载历史失败')
  }
}

async function previewImport() {
  importing.value = true
  try {
    preview.value = await adminAPI.proxies.previewImport({ content: importContent.value })
  } catch (error: any) {
    appStore.showError(error?.message || '导入预览失败')
  } finally {
    importing.value = false
  }
}

async function confirmImport() {
  if (!preview.value) return
  importing.value = true
  try {
    const result = await adminAPI.proxies.confirmImport(preview.value.items)
    appStore.showSuccess(`已导入 ${result.created} 个代理，跳过 ${result.skipped} 个`)
    showImport.value = false
    preview.value = null
  } catch (error: any) {
    appStore.showError(error?.message || '导入失败')
  } finally {
    importing.value = false
  }
}

async function loadSubscriptions() {
  subscriptions.value = await adminAPI.proxies.listProxySubscriptions()
}

async function createSubscription() {
  try {
    await adminAPI.proxies.createProxySubscription({ ...subscriptionForm })
    subscriptionForm.name = ''
    subscriptionForm.url = ''
    await loadSubscriptions()
    appStore.showSuccess('订阅源已新增')
  } catch (error: any) {
    appStore.showError(error?.message || '新增订阅源失败')
  }
}

async function syncSubscription(id: number) {
  try {
    preview.value = await adminAPI.proxies.syncProxySubscription(id)
    showSubscriptions.value = false
    showImport.value = true
  } catch (error: any) {
    appStore.showError(error?.message || '同步订阅失败')
  }
}

async function deleteSubscription(id: number) {
  try {
    await adminAPI.proxies.deleteProxySubscription(id)
    await loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error?.message || '删除订阅源失败')
  }
}

function changePage(page: number) {
  pagination.page = page
  loadRelationships()
}

function changePageSize(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadRelationships()
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '-'
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadRelationships(), loadSubscriptions()])
})
</script>
