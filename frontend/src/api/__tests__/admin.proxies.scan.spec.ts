import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { post },
}))

import {
  PROXY_SUBSCRIPTION_SCAN_TIMEOUT_MS,
  scanProxySubscription,
} from '@/api/admin/proxies'

describe('admin proxy subscription scan API', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { source_id: 7, selected: 1 } })
  })

  it('uses a long-lived request timeout for scans', async () => {
    await expect(scanProxySubscription(7)).resolves.toMatchObject({ source_id: 7 })

    expect(post).toHaveBeenCalledWith(
      '/admin/proxy-subscriptions/7/scan',
      undefined,
      { timeout: PROXY_SUBSCRIPTION_SCAN_TIMEOUT_MS },
    )
    expect(PROXY_SUBSCRIPTION_SCAN_TIMEOUT_MS).toBe(60 * 60 * 1000)
  })
})
