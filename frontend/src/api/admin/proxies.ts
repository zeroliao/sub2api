/**
 * Admin Proxies API endpoints
 * Handles proxy server management for administrators
 */

import { apiClient } from '../client'
import type {
  Proxy,
  ProxyAccountSummary,
  ProxyDispatchSettings,
  ProxyImportPreview,
  ProxyImportPreviewItem,
  ProxyRelationship,
  ProxyQualityCheckResult,
  ProxySubscriptionSource,
  CreateProxyRequest,
  UpdateProxyRequest,
  PaginatedResponse,
  AdminDataPayload,
  AdminDataImportResult
} from '@/types'

/**
 * List all proxies with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters
 * @returns Paginated list of proxies
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    protocol?: string
    status?: 'active' | 'inactive'
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<Proxy>> {
  const { data } = await apiClient.get<PaginatedResponse<Proxy>>('/admin/proxies', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

/**
 * Get all active proxies (without pagination)
 * @returns List of all active proxies
 */
export async function getAll(): Promise<Proxy[]> {
  const { data } = await apiClient.get<Proxy[]>('/admin/proxies/all')
  return data
}

/**
 * Get all active proxies with account count (sorted by creation time desc)
 * @returns List of all active proxies with account count
 */
export async function getAllWithCount(): Promise<Proxy[]> {
  const { data } = await apiClient.get<Proxy[]>('/admin/proxies/all', {
    params: { with_count: 'true' }
  })
  return data
}

/**
 * Get proxy by ID
 * @param id - Proxy ID
 * @returns Proxy details
 */
export async function getById(id: number): Promise<Proxy> {
  const { data } = await apiClient.get<Proxy>(`/admin/proxies/${id}`)
  return data
}

/**
 * Create new proxy
 * @param proxyData - Proxy data
 * @returns Created proxy
 */
export async function create(proxyData: CreateProxyRequest): Promise<Proxy> {
  const { data } = await apiClient.post<Proxy>('/admin/proxies', proxyData)
  return data
}

/**
 * Update proxy
 * @param id - Proxy ID
 * @param updates - Fields to update
 * @returns Updated proxy
 */
export async function update(id: number, updates: UpdateProxyRequest): Promise<Proxy> {
  const { data } = await apiClient.put<Proxy>(`/admin/proxies/${id}`, updates)
  return data
}

/**
 * Delete proxy
 * @param id - Proxy ID
 * @returns Success confirmation
 */
export async function deleteProxy(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/proxies/${id}`)
  return data
}

/**
 * Toggle proxy status
 * @param id - Proxy ID
 * @param status - New status
 * @returns Updated proxy
 */
export async function toggleStatus(id: number, status: 'active' | 'inactive'): Promise<Proxy> {
  return update(id, { status })
}

/**
 * Test proxy connectivity
 * @param id - Proxy ID
 * @returns Test result with IP info
 */
export async function testProxy(id: number): Promise<{
  success: boolean
  message: string
  latency_ms?: number
  ip_address?: string
  city?: string
  region?: string
  country?: string
  country_code?: string
}> {
  const { data } = await apiClient.post<{
    success: boolean
    message: string
    latency_ms?: number
    ip_address?: string
    city?: string
    region?: string
    country?: string
    country_code?: string
  }>(`/admin/proxies/${id}/test`)
  return data
}

/**
 * Check proxy quality across common AI targets
 * @param id - Proxy ID
 * @returns Quality check result
 */
export async function checkProxyQuality(id: number): Promise<ProxyQualityCheckResult> {
  const { data } = await apiClient.post<ProxyQualityCheckResult>(`/admin/proxies/${id}/quality-check`)
  return data
}

/**
 * Get proxy usage statistics
 * @param id - Proxy ID
 * @returns Proxy usage statistics
 */
export async function getStats(id: number): Promise<{
  total_accounts: number
  active_accounts: number
  total_requests: number
  success_rate: number
  average_latency: number
}> {
  const { data } = await apiClient.get<{
    total_accounts: number
    active_accounts: number
    total_requests: number
    success_rate: number
    average_latency: number
  }>(`/admin/proxies/${id}/stats`)
  return data
}

/**
 * Get accounts using a proxy
 * @param id - Proxy ID
 * @returns List of accounts using the proxy
 */
export async function getProxyAccounts(id: number): Promise<ProxyAccountSummary[]> {
  const { data } = await apiClient.get<ProxyAccountSummary[]>(`/admin/proxies/${id}/accounts`)
  return data
}

/**
 * Batch create proxies
 * @param proxies - Array of proxy data to create
 * @returns Creation result with count of created and skipped
 */
export async function batchCreate(
  proxies: Array<{
    protocol: string
    host: string
    port: number
    username?: string
    password?: string
  }>
): Promise<{
  created: number
  skipped: number
}> {
  const { data } = await apiClient.post<{
    created: number
    skipped: number
  }>('/admin/proxies/batch', { proxies })
  return data
}

export async function batchDelete(ids: number[]): Promise<{
  deleted_ids: number[]
  skipped: Array<{ id: number; reason: string }>
}> {
  const { data } = await apiClient.post<{
    deleted_ids: number[]
    skipped: Array<{ id: number; reason: string }>
  }>('/admin/proxies/batch-delete', { ids })
  return data
}

export async function exportData(options?: {
  ids?: number[]
  filters?: {
    protocol?: string
    status?: 'active' | 'inactive'
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  }
}): Promise<AdminDataPayload> {
  const params: Record<string, string> = {}
  if (options?.ids && options.ids.length > 0) {
    params.ids = options.ids.join(',')
  } else if (options?.filters) {
    const { protocol, status, search, sort_by, sort_order } = options.filters
    if (protocol) params.protocol = protocol
    if (status) params.status = status
    if (search) params.search = search
    if (sort_by) params.sort_by = sort_by
    if (sort_order) params.sort_order = sort_order
  }
  const { data } = await apiClient.get<AdminDataPayload>('/admin/proxies/data', { params })
  return data
}

export async function importData(payload: {
  data: AdminDataPayload
}): Promise<AdminDataImportResult> {
  const { data } = await apiClient.post<AdminDataImportResult>('/admin/proxies/data', payload)
  return data
}

export async function previewImport(payload: {
  content?: string
  url?: string
  provider?: string
}): Promise<ProxyImportPreview> {
  const { data } = await apiClient.post<ProxyImportPreview>('/admin/proxies/import/preview', payload)
  return data
}

export async function confirmImport(items: ProxyImportPreviewItem[]): Promise<{
  created: number
  skipped: number
  failed: number
  proxy_ids: number[]
  errors?: string[]
}> {
  const { data } = await apiClient.post<{
    created: number
    skipped: number
    failed: number
    proxy_ids: number[]
    errors?: string[]
  }>('/admin/proxies/import/confirm', { items })
  return data
}

export async function batchHealthCheck(ids: number[] = []): Promise<Array<{
  success: boolean
  message: string
  latency_ms?: number
  ip_address?: string
}>> {
  const { data } = await apiClient.post<Array<{
    success: boolean
    message: string
    latency_ms?: number
    ip_address?: string
  }>>('/admin/proxies/batch-health-check', { ids })
  return data
}

export async function listProxyRelationships(
  page = 1,
  pageSize = 20,
  filters?: { platform?: string; status?: string; search?: string }
): Promise<PaginatedResponse<ProxyRelationship>> {
  const { data } = await apiClient.get<PaginatedResponse<ProxyRelationship>>('/admin/proxy-dispatch/relationships', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function reassignAccountProxy(accountId: number): Promise<ProxyRelationship> {
  const { data } = await apiClient.post<ProxyRelationship>(`/admin/proxy-dispatch/accounts/${accountId}/reassign`)
  return data
}

export async function restoreAccountProxyHistory(accountId: number): Promise<ProxyRelationship> {
  const { data } = await apiClient.post<ProxyRelationship>(`/admin/proxy-dispatch/accounts/${accountId}/restore-history`)
  return data
}

export async function getAccountProxyHistory(accountId: number) {
  const { data } = await apiClient.get(`/admin/proxy-dispatch/accounts/${accountId}/history`)
  return data
}

export async function getProxyDispatchSettings(): Promise<ProxyDispatchSettings> {
  const { data } = await apiClient.get<ProxyDispatchSettings>('/admin/proxy-dispatch/settings')
  return data
}

export async function updateProxyDispatchSettings(settings: ProxyDispatchSettings): Promise<ProxyDispatchSettings> {
  const { data } = await apiClient.put<ProxyDispatchSettings>('/admin/proxy-dispatch/settings', settings)
  return data
}

export async function listProxySubscriptions(): Promise<ProxySubscriptionSource[]> {
  const { data } = await apiClient.get<ProxySubscriptionSource[]>('/admin/proxy-subscriptions')
  return data
}

export async function createProxySubscription(payload: Partial<ProxySubscriptionSource>): Promise<ProxySubscriptionSource> {
  const { data } = await apiClient.post<ProxySubscriptionSource>('/admin/proxy-subscriptions', payload)
  return data
}

export async function updateProxySubscription(id: number, payload: Partial<ProxySubscriptionSource>): Promise<ProxySubscriptionSource> {
  const { data } = await apiClient.put<ProxySubscriptionSource>(`/admin/proxy-subscriptions/${id}`, payload)
  return data
}

export async function deleteProxySubscription(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/proxy-subscriptions/${id}`)
  return data
}

export async function syncProxySubscription(id: number): Promise<ProxyImportPreview> {
  const { data } = await apiClient.post<ProxyImportPreview>(`/admin/proxy-subscriptions/${id}/sync`)
  return data
}

export const proxiesAPI = {
  list,
  getAll,
  getAllWithCount,
  getById,
  create,
  update,
  delete: deleteProxy,
  toggleStatus,
  testProxy,
  checkProxyQuality,
  getStats,
  getProxyAccounts,
  batchCreate,
  batchDelete,
  exportData,
  importData,
  previewImport,
  confirmImport,
  batchHealthCheck,
  listProxyRelationships,
  reassignAccountProxy,
  restoreAccountProxyHistory,
  getAccountProxyHistory,
  getProxyDispatchSettings,
  updateProxyDispatchSettings,
  listProxySubscriptions,
  createProxySubscription,
  updateProxySubscription,
  deleteProxySubscription,
  syncProxySubscription
}

export default proxiesAPI
