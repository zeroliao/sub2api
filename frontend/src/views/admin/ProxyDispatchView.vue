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
                <option value="manual_only">仅手动代理</option>
                <option value="global">全局兜底</option>
              </select>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                <input v-model="settings.auto_assign_enabled" type="checkbox" class="h-4 w-4" @change="saveSettings" />
                自动分配
              </label>
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-dark-300">订阅扫描</div>
            <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ scanResult?.selected ?? preview?.recommended ?? 0 }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              最近一次推荐可用节点数
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <div class="text-sm text-gray-500 dark:text-dark-300">策略状态</div>
            <div class="mt-2 text-sm text-gray-700 dark:text-gray-200">
              sidecar 节点会先进入本地出口池，直连节点会直接导入代理池。
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
              <code class="text-xs text-gray-500">
                {{ row.current_proxy.protocol }}://{{ row.current_proxy.host }}:{{ row.current_proxy.port }}
              </code>
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
          placeholder="粘贴 http/socks5、多行代理、Clash YAML、sing-box JSON、订阅 URL、ss/vmess/vless/trojan/hysteria2/tuic/anytls 节点"
        />
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" :disabled="importing" @click="previewImport">预览</button>
          <button class="btn btn-primary" :disabled="!preview || importing || importablePreviewCount === 0" @click="confirmImport">导入推荐节点</button>
        </div>
        <div v-if="preview?.sidecar_only" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100">
          检测到 {{ preview.sidecar_only }} 个节点需要通过“代理订阅源”接入，先扫描订阅并生成本地 sidecar 出口后，才能参与代理池分发。
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
                    <input
                      v-model="item.selected"
                      type="checkbox"
                      :disabled="!item.valid || item.duplicate || item.sidecar_required"
                      :title="proxyImportItemDisabledReason(item) || undefined"
                    />
                  </td>
                  <td class="px-3 py-2">
                    <div>{{ item.name || item.host || item.protocol }}</div>
                    <div v-if="proxyImportItemDisabledReason(item)" class="mt-1 text-xs text-gray-500">
                      {{ proxyImportItemDisabledReason(item) }}
                    </div>
                  </td>
                  <td class="px-3 py-2">{{ item.protocol }}</td>
                  <td class="px-3 py-2">{{ item.host }}:{{ item.port || '-' }}</td>
                  <td class="px-3 py-2">
                    <span v-if="item.sidecar_required" class="badge badge-warning">sidecar</span>
                    <span v-else-if="item.duplicate" class="badge badge-gray">重复</span>
                    <span v-else-if="item.valid" class="badge badge-success">可导入</span>
                    <span v-else class="badge badge-danger">{{ item.error || '无效' }}</span>
                    <div v-if="item.sidecar_required && item.sidecar_hint" class="mt-1 text-xs text-gray-500">
                      {{ item.sidecar_hint }}
                    </div>
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

        <div v-if="activeScanningSourceID" class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-100">
          <div class="font-medium">订阅扫描进行中</div>
          <div class="mt-1">
            当前订阅：{{ activeScanningName }}，已运行 {{ formatScanElapsed(activeScanningSourceID) }}，预计 {{ formatActiveScanEstimate() }}
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <button
            class="flex w-full items-center justify-between gap-4 px-4 py-3 text-left"
            type="button"
            @click="subscriptionMetricsCollapsed = !subscriptionMetricsCollapsed"
          >
            <div class="min-w-0">
              <div class="text-sm font-medium text-gray-900 dark:text-white">订阅策略指标</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">
                扫描时长、国家分布、sidecar 出口数量、纯净度阈值等高级参数
              </div>
              <div class="mt-2 flex flex-wrap gap-2">
                <span
                  v-for="item in subscriptionMetricsSummary"
                  :key="item.label"
                  class="rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200"
                >
                  {{ item.label }} {{ item.value }}
                </span>
              </div>
            </div>
            <span class="text-sm text-gray-500 dark:text-dark-300">
              {{ subscriptionMetricsCollapsed ? '展开' : '收起' }}
            </span>
          </button>
        </div>

        <div v-if="!subscriptionMetricsCollapsed" class="grid gap-4 xl:grid-cols-2">
          <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">接入与检测</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">控制订阅源如何生成本地出口，以及是否做 IP 纯净度识别。</div>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">启用 sidecar</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">将订阅节点转换为本地 socks5 出口，供代理池和账号分发使用。</div>
              </div>
              <label class="inline-flex items-center justify-self-start gap-2 text-sm text-gray-700 dark:text-gray-200 sm:justify-self-end">
                <input v-model="subscriptionForm.sidecar_enabled" type="checkbox" class="h-4 w-4" />
                <span>开启</span>
              </label>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">运行时</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">负责实际拉起订阅节点并暴露本地代理端口的 sidecar 程序。</div>
              </div>
              <select v-model="subscriptionForm.runtime" class="input w-full sm:justify-self-end">
                <option value="sing-box">sing-box</option>
              </select>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">纯净度检测来源</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">用于识别出口 IP 风险分与国家信息，帮助筛掉不干净的节点。</div>
              </div>
              <select v-model="subscriptionForm.reputation_provider" class="input w-full sm:justify-self-end">
                <option value="none">不做纯净度检测</option>
                <option value="abuseipdb">AbuseIPDB</option>
              </select>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(220px,320px)]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">纯净度 API Key 引用</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">支持 `env:` 或 `keymd:` 形式，避免把密钥明文直接写进页面配置。</div>
              </div>
              <input v-model="subscriptionForm.reputation_api_key_ref" class="input w-full sm:justify-self-end" placeholder="env:ABUSEIPDB_API_KEY 或 keymd:AbuseIPDB API Key" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">端口起始</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">sidecar 本地出口分配的起始端口，首个活跃节点会从这里开始占用。</div>
              </div>
              <input v-model.number="subscriptionForm.port_start" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">端口结束</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">sidecar 可用端口范围上限，活跃出口数不能超过这个端口池容量。</div>
              </div>
              <input v-model.number="subscriptionForm.port_end" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">扫描周期分钟</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">后台自动重扫这个订阅源的频率，用于持续刷新节点状态与推荐结果。</div>
              </div>
              <input v-model.number="subscriptionForm.scan_interval_minutes" class="input w-full sm:justify-self-end" type="number" min="5" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">健康检查分钟</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">对已生成的本地出口做可用性检查的间隔，帮助尽快发现失效 sidecar。</div>
              </div>
              <input v-model.number="subscriptionForm.health_check_interval_minutes" class="input w-full sm:justify-self-end" type="number" min="5" />
            </div>
          </div>

          <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">筛选与配额</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">决定最终保留多少节点、如何做国家分布，以及节点必须达到的最低质量门槛。</div>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最大导入节点数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">扫描完成后最多保留多少个候选节点进入已选池。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.max_enabled_nodes" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">活跃出口数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">最终真正生成本地 sidecar 出口的节点数量，影响可供分发的本地代理总数。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.max_active_sidecar_nodes" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">备用节点数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">除已选节点外，额外预留的候补节点数量，便于主节点失效时接替。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.standby_nodes" class="input w-full sm:justify-self-end" type="number" min="0" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">每国家节点上限</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">同一个国家最多保留多少个节点，避免候选池过度集中到单一国家。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.max_nodes_per_country" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最少国家数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">优先保证至少覆盖这么多个国家，再按总分继续补齐节点。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.min_country_count" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最多国家数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">已覆盖国家超过这个数量后，不再继续引入新的国家维度。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.max_country_count" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最大延迟 ms</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">延迟高于这个阈值的节点会被降级或直接筛掉。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.max_latency_ms" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最低综合分</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">节点总评分必须达到这个分数，才有资格进入推荐候选池。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.min_quality_score" class="input w-full sm:justify-self-end" type="number" min="0" max="100" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最低纯净度</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">出口 IP 纯净度低于这个分数时，不再作为优先推荐节点。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.min_ip_clean_score" class="input w-full sm:justify-self-end" type="number" min="0" max="100" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(220px,320px)]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">优先国家</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">用逗号分隔国家代码，例如 `US,JP,SG`，这些国家会在评分时获得优先级。</div>
              </div>
              <input v-model="preferredCountriesText" class="input w-full sm:justify-self-end" placeholder="US,JP,SG" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(220px,320px)]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">屏蔽国家</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">用逗号分隔国家代码，例如 `RU,IR`，这些国家会在筛选阶段直接排除。</div>
              </div>
              <input v-model="blockedCountriesText" class="input w-full sm:justify-self-end" placeholder="RU,IR" />
            </div>
          </div>

          <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">扫描节奏</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">控制单轮扫描的批次、总时长预算与信誉缓存时间。</div>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">扫描批大小</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">每批处理多少个节点，批次之间会按预算插入短暂停顿。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.scan_batch_size" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">目标扫描时长 分钟</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">系统会尽量把整轮扫描控制在这个时长附近，用于平衡速度与资源占用。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.scan_budget_minutes" class="input w-full sm:justify-self-end" type="number" min="5" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最大扫描时长 分钟</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">单轮扫描的硬超时，超过后会停止继续处理，避免任务一直占用机器。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.scan_budget_max_minutes" class="input w-full sm:justify-self-end" type="number" min="5" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">纯净度缓存 小时</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">同一出口 IP 的信誉结果缓存多久，减少频繁调用外部信誉 API。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.reputation_cache_hours" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>
          </div>

          <div class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">资源保护与容错</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-300">避免扫描吃满机器资源，并在节点连续超时时自动休眠和替补。</div>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">资源自适应扫描</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">根据当前机器内存情况动态放慢或延后扫描，避免影响线上服务。</div>
              </div>
              <label class="inline-flex items-center justify-self-start gap-2 text-sm text-gray-700 dark:text-gray-200 sm:justify-self-end">
                <input v-model="subscriptionForm.strategy.resource_adaptive_scan" type="checkbox" class="h-4 w-4" />
                <span>开启</span>
              </label>
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">最低空闲内存 MB</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">资源自适应扫描的参考安全线，低于该值时会明显放慢扫描节奏。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.min_free_memory_mb" class="input w-full sm:justify-self-end" type="number" min="128" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">暂停扫描内存阈值 MB</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">可用内存低于这个值时，自动扫描会先延后，等机器恢复后再继续。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.pause_free_memory_mb" class="input w-full sm:justify-self-end" type="number" min="128" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">超时几次休眠</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">同一个节点连续超时达到这个次数后，会暂时停用，避免反复拖慢扫描。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.timeout_sleep_after" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">休眠分钟数</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">节点进入休眠后多久才允许重新参与扫描和候选选择。</div>
              </div>
              <input v-model.number="subscriptionForm.strategy.sleep_minutes" class="input w-full sm:justify-self-end" type="number" min="1" />
            </div>

            <div class="grid items-center gap-3 sm:grid-cols-[minmax(0,1fr)_220px]">
              <div>
                <div class="text-sm text-gray-900 dark:text-white">优先同国家替补</div>
                <div class="text-xs text-gray-500 dark:text-dark-300">已选节点失效后，优先从同一国家的备用节点里补位，减少出口地域波动。</div>
              </div>
              <label class="inline-flex items-center justify-self-start gap-2 text-sm text-gray-700 dark:text-gray-200 sm:justify-self-end">
                <input v-model="subscriptionForm.strategy.replace_same_country_first" type="checkbox" class="h-4 w-4" />
                <span>开启</span>
              </label>
            </div>
          </div>
        </div>

        <div class="divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
          <div v-for="source in subscriptions" :key="source.id" class="flex flex-col gap-3 p-3 lg:flex-row lg:items-center lg:justify-between">
            <div class="min-w-0 space-y-1">
              <div class="font-medium text-gray-900 dark:text-white">{{ source.name }}</div>
              <div class="truncate text-xs text-gray-500">{{ source.url }}</div>
              <div class="flex flex-wrap gap-2 text-xs text-gray-500">
                <span>已选 {{ readScanNumber(source.last_scan_result, 'selected') ?? '-' }}</span>
                <span>sidecar {{ source.sidecar_enabled ? 'on' : 'off' }}</span>
                <span>纯净度 {{ source.reputation_provider || 'none' }}</span>
                <span v-if="isScanning(source.id)">扫描中 {{ formatScanElapsed(source.id) }}</span>
                <span v-else>最近扫描 {{ formatDate(source.last_scan_at) }}</span>
              </div>
              <div v-if="isScanning(source.id)" class="flex flex-wrap gap-2 text-xs">
                <span class="badge badge-warning">扫描中</span>
                <span class="text-blue-600 dark:text-blue-300">预计 {{ formatSourceScanEstimate(source) }}</span>
              </div>
              <div v-if="source.last_error" class="text-xs text-red-500">{{ source.last_error }}</div>
            </div>
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-sm btn-secondary" @click="syncSubscription(source.id)">同步预览</button>
              <button class="btn btn-sm btn-secondary" :disabled="isScanning(source.id)" @click="scanSubscription(source.id)">
                <Icon name="refresh" size="sm" :class="isScanning(source.id) ? 'animate-spin mr-1' : 'mr-1'" />
                {{ isScanning(source.id) ? '扫描中' : '扫描' }}
              </button>
              <button class="btn btn-sm btn-secondary" @click="openSubscriptionNodes(source)">节点</button>
              <button class="btn btn-sm btn-danger" @click="deleteSubscription(source.id)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="showNodesDialog" :title="nodesDialogTitle" width="full" @close="closeSubscriptionNodes">
      <div class="space-y-4">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedNodeSource?.name || '订阅节点' }}</div>
            <div class="truncate text-xs text-gray-500">{{ selectedNodeSource?.url || '扫描后会在这里展示订阅源下的全部节点' }}</div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-sm btn-secondary" @click="closeSubscriptionNodes">
              <Icon name="arrowLeft" size="sm" class="mr-1" />
              返回订阅源
            </button>
            <button class="btn btn-sm btn-secondary" :disabled="!selectedNodeSource || savingNodeStrategy" @click="saveNodeStrategy(false)">
              保存策略
            </button>
            <button class="btn btn-sm btn-primary" :disabled="!selectedNodeSource || savingNodeStrategy || isScanning(selectedNodeSource.id)" @click="saveNodeStrategy(true)">
              <Icon name="refresh" size="sm" :class="selectedNodeSource && isScanning(selectedNodeSource.id) ? 'animate-spin mr-1' : 'mr-1'" />
              保存并扫描
            </button>
          </div>
        </div>

        <div class="grid gap-3 md:grid-cols-6">
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">全部节点</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ nodePreviewSummary.total }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">预览选中</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ nodePreviewSummary.selected }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">备用</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ nodePreviewSummary.standby }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">不可选</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ nodePreviewSummary.excluded }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">国家数</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ nodePreviewSummary.countryCount }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">上次实际选中</div>
            <div class="mt-1 font-semibold text-gray-900 dark:text-white">{{ selectedNodeSource ? readScanNumber(selectedNodeSource.last_scan_result, 'selected') ?? '-' : '-' }}</div>
          </div>
        </div>

        <div class="grid gap-3 rounded-lg border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40 lg:grid-cols-4">
          <div class="lg:col-span-4">
            <div class="flex flex-wrap gap-2">
              <button
                v-for="preset in nodeStrategyPresets"
                :key="preset.key"
                class="btn btn-sm btn-secondary"
                type="button"
                @click="applyNodeStrategyPreset(preset.key)"
              >
                {{ preset.label }}
              </button>
            </div>
          </div>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">最大导入</span>
            <input v-model.number="nodeStrategyDraft.max_enabled_nodes" class="input w-full" type="number" min="1" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">每国家上限</span>
            <input v-model.number="nodeStrategyDraft.max_nodes_per_country" class="input w-full" type="number" min="1" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">最多国家数</span>
            <input v-model.number="nodeStrategyDraft.max_country_count" class="input w-full" type="number" min="1" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">备用节点</span>
            <input v-model.number="nodeStrategyDraft.standby_nodes" class="input w-full" type="number" min="0" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">最高延迟 ms</span>
            <input v-model.number="nodeStrategyDraft.max_latency_ms" class="input w-full" type="number" min="1" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">最低纯净度</span>
            <input v-model.number="nodeStrategyDraft.min_ip_clean_score" class="input w-full" type="number" min="0" max="100" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">最低综合分</span>
            <input v-model.number="nodeStrategyDraft.min_quality_score" class="input w-full" type="number" min="0" max="100" />
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">状态筛选</span>
            <select v-model="nodeStatusFilter" class="input w-full">
              <option value="">全部</option>
              <option value="selected">预览选中</option>
              <option value="standby">备用</option>
              <option value="candidate">候选</option>
              <option value="excluded">不可选</option>
              <option value="sleeping">休眠</option>
              <option value="missing">缺失</option>
            </select>
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">不可选原因</span>
            <select v-model="nodeReasonFilter" class="input w-full">
              <option value="">全部原因</option>
              <option v-for="reason in nodeReasonOptions" :key="reason" :value="reason">{{ reason }}</option>
            </select>
          </label>
          <label class="space-y-1 text-sm">
            <span class="text-xs text-gray-500">排序</span>
            <select v-model="nodeSortKey" class="input w-full">
              <option value="preview">预览状态</option>
              <option value="score">分数高到低</option>
              <option value="latency">延迟低到高</option>
              <option value="clean">纯净度高到低</option>
              <option value="country">国家</option>
              <option value="name">节点名</option>
            </select>
          </label>
          <label class="space-y-1 text-sm lg:col-span-2">
            <span class="text-xs text-gray-500">优先国家</span>
            <input v-model="nodePreferredCountriesText" class="input w-full" placeholder="US,JP,SG" />
          </label>
          <label class="space-y-1 text-sm lg:col-span-2">
            <span class="text-xs text-gray-500">屏蔽国家</span>
            <input v-model="nodeBlockedCountriesText" class="input w-full" placeholder="RU,IR" />
          </label>
        </div>

        <div class="grid gap-3 lg:grid-cols-3">
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">策略差异</div>
            <div class="mt-2 flex flex-wrap gap-2">
              <span class="badge badge-success">新增 {{ nodeDiffSummary.added }}</span>
              <span class="badge badge-gray">移除 {{ nodeDiffSummary.removed }}</span>
              <span class="badge badge-warning">保持 {{ nodeDiffSummary.unchanged }}</span>
            </div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">最近扫描</div>
            <div class="mt-1 text-gray-900 dark:text-white">{{ selectedNodeSource ? formatScanResultSummary(selectedNodeSource.last_scan_result) : '-' }}</div>
            <div class="mt-1 text-xs text-gray-500">{{ formatDate(selectedNodeSource?.last_scan_at) }}</div>
          </div>
          <div class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700">
            <div class="text-xs text-gray-500">纯净度配置</div>
            <div class="mt-1 text-gray-900 dark:text-white">{{ reputationConfigStatus }}</div>
            <div class="mt-1 text-xs text-gray-500">{{ selectedNodeSource?.reputation_api_key_ref || '未设置 API Key 引用' }}</div>
          </div>
        </div>

        <div class="max-h-[560px] overflow-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-900 dark:text-dark-300">
              <tr>
                <th class="px-3 py-2">预览</th>
                <th class="px-3 py-2">节点</th>
                <th class="px-3 py-2">协议</th>
                <th class="px-3 py-2">出口</th>
                <th class="px-3 py-2">延迟</th>
                <th class="px-3 py-2">纯净度</th>
                <th class="px-3 py-2">分数</th>
                <th class="px-3 py-2">运行指标</th>
                <th class="px-3 py-2">状态与原因</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredNodePreviewRows" :key="row.node.id" :class="['border-t border-gray-100 dark:border-dark-800', row.preview_status === 'excluded' || row.preview_status === 'missing' ? 'bg-gray-50/60 text-gray-500 dark:bg-dark-900/30' : '']">
                <td class="px-3 py-2">
                  <input type="checkbox" :checked="row.preview_status === 'selected'" disabled class="h-4 w-4" :aria-label="`${row.node.name || row.node.server} 是否按当前策略选中`" />
                </td>
                <td class="px-3 py-2">
                  <div class="font-medium text-gray-900 dark:text-white">{{ row.node.name || row.node.server }}</div>
                  <div class="text-xs text-gray-500">{{ row.node.server }}:{{ row.node.port }}</div>
                  <div class="mt-1 max-w-[240px] truncate text-xs text-gray-400" :title="row.node.node_key">{{ row.node.node_key }}</div>
                </td>
                <td class="px-3 py-2">
                  <div>{{ row.node.protocol }}</div>
                  <div class="text-xs text-gray-500">{{ row.node.sidecar_required ? 'sidecar' : 'direct' }}</div>
                </td>
                <td class="px-3 py-2">
                  <div>{{ row.country || '-' }}</div>
                  <div class="text-xs text-gray-500">{{ row.node.exit_ip || '-' }}</div>
                  <div class="text-xs text-gray-500">{{ row.node.exit_country || row.node.exit_country_code || '' }}</div>
                </td>
                <td class="px-3 py-2">{{ row.node.latency_ms != null ? `${row.node.latency_ms} ms` : '-' }}</td>
                <td class="px-3 py-2">
                  <div>{{ row.node.ip_clean_score ?? '-' }}</div>
                  <div class="text-xs text-gray-500">{{ row.node.reputation_provider || '-' }}</div>
                  <div class="text-xs text-gray-500">{{ formatDate(row.node.reputation_checked_at) }}</div>
                </td>
                <td class="px-3 py-2">{{ row.node.score }}</td>
                <td class="px-3 py-2">
                  <div>失败 {{ row.node.failure_count || 0 }}</div>
                  <div class="text-xs text-amber-600 dark:text-amber-300">超时 {{ row.node.timeout_count || 0 }}</div>
                  <div class="text-xs text-gray-500">扫描 {{ formatDate(row.node.last_scanned_at) }}</div>
                  <div v-if="row.node.sleep_until" class="text-xs text-gray-500">休眠 {{ formatDate(row.node.sleep_until) }}</div>
                </td>
                <td class="px-3 py-2">
                  <div class="flex flex-wrap gap-1">
                    <span :class="['badge', nodePreviewBadgeClass(row.preview_status)]">{{ nodePreviewStatusLabel(row.preview_status) }}</span>
                    <span :class="['badge', row.node.selected ? 'badge-success' : 'badge-gray']">上次 {{ row.node.status }}</span>
                  </div>
                  <div class="mt-1 text-xs text-gray-500">{{ row.reason }}</div>
                  <div v-if="row.node.last_error" class="mt-1 text-xs text-red-500">{{ row.node.last_error }}</div>
                </td>
              </tr>
              <tr v-if="!filteredNodePreviewRows.length">
                <td colspan="9" class="px-3 py-8 text-center text-sm text-gray-500">暂无匹配节点</td>
              </tr>
            </tbody>
          </table>
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
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AccountProxyBinding,
  ProxyDispatchSettings,
  ProxyImportPreview,
  ProxyImportPreviewItem,
  ProxyRelationship,
  ProxySubscriptionNode,
  ProxySubscriptionScanResult,
  ProxySubscriptionScanStatus,
  ProxySubscriptionSource,
  ProxySubscriptionStrategy
} from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()

function defaultStrategy(): ProxySubscriptionStrategy {
  return {
    max_parsed_nodes: 200,
    max_enabled_nodes: 20,
    max_active_sidecar_nodes: 5,
    max_probe_concurrency: 3,
    scan_batch_size: 5,
    standby_nodes: 2,
    min_country_count: 1,
    max_country_count: 6,
    max_nodes_per_country: 3,
    preferred_countries: [],
    blocked_countries: [],
    max_latency_ms: 2500,
    min_ip_clean_score: 40,
    min_quality_score: 60,
    selection_mode: 'balanced',
    reputation_cache_hours: 24,
    scan_budget_minutes: 30,
    scan_budget_max_minutes: 40,
    resource_adaptive_scan: true,
    min_free_memory_mb: 800,
    pause_free_memory_mb: 500,
    timeout_sleep_after: 3,
    sleep_minutes: 30,
    replace_same_country_first: true
  }
}

function createSubscriptionForm() {
  return {
    name: '',
    url: '',
    source_type: 'clash',
    sync_enabled: true,
    sync_interval_minutes: 1440,
    status: 'active',
    sidecar_enabled: true,
    runtime: 'sing-box',
    port_start: 31000,
    port_end: 31999,
    scan_enabled: true,
    scan_interval_minutes: 60,
    health_check_interval_minutes: 20,
    reputation_provider: 'abuseipdb',
    reputation_api_key_ref: 'keymd:AbuseIPDB API Key',
    strategy: defaultStrategy()
  }
}

const loading = ref(false)
const importing = ref(false)
const relationships = ref<ProxyRelationship[]>([])
const preview = ref<ProxyImportPreview | null>(null)
const subscriptions = ref<ProxySubscriptionSource[]>([])
const subscriptionNodes = ref<ProxySubscriptionNode[]>([])
const scanResult = ref<ProxySubscriptionScanResult | null>(null)
const history = ref<AccountProxyBinding[]>([])
const showImport = ref(false)
const showSubscriptions = ref(false)
const showHistory = ref(false)
const showNodesDialog = ref(false)
const nodesDialogTitle = ref('订阅节点')
const selectedNodeSource = ref<ProxySubscriptionSource | null>(null)
const savingNodeStrategy = ref(false)
const nodeStatusFilter = ref('')
const nodeReasonFilter = ref('')
const nodeSortKey = ref<'preview' | 'score' | 'latency' | 'clean' | 'country' | 'name'>('preview')
const importContent = ref('')
const preferredCountriesText = ref('')
const blockedCountriesText = ref('')
const nodePreferredCountriesText = ref('')
const nodeBlockedCountriesText = ref('')
const scanningSubscriptionIds = ref<number[]>([])
const subscriptionMetricsCollapsed = ref(true)
const scanStartedAtMap = reactive<Record<number, number>>({})
const scanNow = ref(Date.now())
const serverScanStatus = ref<ProxySubscriptionScanStatus | null>(null)
let scanTicker: ReturnType<typeof setInterval> | null = null
let scanStatusPoller: ReturnType<typeof setInterval> | null = null
let openingNodesRoute = false

const settings = reactive<ProxyDispatchSettings>({
  direct_fallback_mode: 'off',
  auto_assign_enabled: true
})
const filters = reactive({ platform: '', status: '', search: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const subscriptionForm = reactive(createSubscriptionForm())
const nodeStrategyDraft = reactive(defaultStrategy())
const importablePreviewCount = computed(() => {
  if (!preview.value) return 0
  return preview.value.items.filter(item => item.valid && !item.duplicate && !item.sidecar_required && item.selected).length
})
const activeScanningSource = computed(() => subscriptions.value.find(source => isScanning(source.id)) || null)
const activeScanningSourceID = computed(() => scanningSubscriptionIds.value[0] || serverScanStatus.value?.source_id || 0)
const activeScanningName = computed(() => {
  const source = activeScanningSource.value
  return source?.name || serverScanStatus.value?.source_name || `订阅源 #${serverScanStatus.value?.source_id || ''}`
})
const subscriptionMetricsSummary = computed(() => [
  { label: '活跃出口', value: String(subscriptionForm.strategy.max_active_sidecar_nodes) },
  { label: '最大导入', value: String(subscriptionForm.strategy.max_enabled_nodes) },
  {
    label: '扫描预算',
    value: `${subscriptionForm.strategy.scan_budget_minutes}-${subscriptionForm.strategy.scan_budget_max_minutes} 分钟`
  },
  { label: '最低纯净度', value: String(subscriptionForm.strategy.min_ip_clean_score) }
])
type NodePreviewStatus = 'selected' | 'standby' | 'candidate' | 'excluded' | 'sleeping' | 'missing'
type NodePreviewRow = ProxySubscriptionNode & {
  preview_status: NodePreviewStatus
  preview_reason: string
  preview_rank: number
  preview_country: string
}
const nodeStrategyPresets = [
  { key: 'stable', label: '稳定优先' },
  { key: 'speed', label: '速度优先' },
  { key: 'coverage', label: '国家覆盖' },
  { key: 'conservative', label: '资源保守' }
] as const
type NodeStrategyPreset = typeof nodeStrategyPresets[number]['key']
const nodePreviewRows = computed<NodePreviewRow[]>(() => previewSubscriptionNodes(subscriptionNodes.value, nodeStrategyDraft, selectedNodeSource.value))
const filteredNodePreviewRows = computed(() => {
  const status = nodeStatusFilter.value
  const reason = nodeReasonFilter.value
  return sortNodeRows(nodePreviewRows.value.filter(row => {
    if (status && row.preview_status !== status) return false
    if (reason && reasonCategory(row.preview_reason) !== reason) return false
    return true
  })).map(row => ({
    node: row,
    preview_status: row.preview_status,
    reason: row.preview_reason,
    country: row.preview_country
  }))
})
const nodePreviewSummary = computed(() => {
  const rows = nodePreviewRows.value
  const selectedRows = rows.filter(row => row.preview_status === 'selected')
  return {
    total: rows.length,
    selected: selectedRows.length,
    standby: rows.filter(row => row.preview_status === 'standby').length,
    excluded: rows.filter(row => row.preview_status === 'excluded' || row.preview_status === 'missing' || row.preview_status === 'sleeping').length,
    countryCount: new Set(selectedRows.map(row => row.preview_country || 'unknown')).size
  }
})
const nodeReasonOptions = computed(() => {
  return Array.from(new Set(nodePreviewRows.value.map(row => reasonCategory(row.preview_reason)).filter(Boolean))).sort()
})
const nodeDiffSummary = computed(() => {
  const previewSelected = new Set(nodePreviewRows.value.filter(row => row.preview_status === 'selected').map(row => row.node_key))
  const actualSelected = new Set(subscriptionNodes.value.filter(node => node.selected).map(node => node.node_key))
  let added = 0
  let removed = 0
  let unchanged = 0
  previewSelected.forEach(key => {
    if (actualSelected.has(key)) unchanged++
    else added++
  })
  actualSelected.forEach(key => {
    if (!previewSelected.has(key)) removed++
  })
  return { added, removed, unchanged }
})
const reputationConfigStatus = computed(() => {
  const source = selectedNodeSource.value
  if (!source || source.reputation_provider === 'none') return '未启用纯净度检测'
  if (!source.reputation_api_key_ref) return '已启用，但缺少 API Key 引用'
  if (source.reputation_api_key_ref.startsWith('env:')) return '使用服务器环境变量'
  if (source.reputation_api_key_ref.startsWith('keymd:')) return '使用 key.md 标签引用'
  return '使用自定义引用'
})

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

function parseCountryList(raw: string): string[] {
  return raw
    .split(',')
    .map(item => item.trim().toUpperCase())
    .filter(Boolean)
}

function cloneStrategy(strategy?: ProxySubscriptionStrategy): ProxySubscriptionStrategy {
  const base = strategy || defaultStrategy()
  return {
    ...defaultStrategy(),
    ...base,
    preferred_countries: [...(base.preferred_countries || [])],
    blocked_countries: [...(base.blocked_countries || [])]
  }
}

function applyNodeStrategyDraft(strategy?: ProxySubscriptionStrategy) {
  const next = cloneStrategy(strategy)
  Object.assign(nodeStrategyDraft, next)
  nodePreferredCountriesText.value = next.preferred_countries.join(',')
  nodeBlockedCountriesText.value = next.blocked_countries.join(',')
}

function resetSubscriptionForm() {
  Object.assign(subscriptionForm, createSubscriptionForm())
  preferredCountriesText.value = ''
  blockedCountriesText.value = ''
}

function buildNodeStrategyPayload(): ProxySubscriptionStrategy {
  return {
    ...nodeStrategyDraft,
    preferred_countries: parseCountryList(nodePreferredCountriesText.value),
    blocked_countries: parseCountryList(nodeBlockedCountriesText.value)
  }
}

function buildSubscriptionPayload() {
  return {
    ...subscriptionForm,
    strategy: {
      ...subscriptionForm.strategy,
      preferred_countries: parseCountryList(preferredCountriesText.value),
      blocked_countries: parseCountryList(blockedCountriesText.value)
    }
  }
}

function buildSubscriptionUpdatePayload(source: ProxySubscriptionSource, strategy = source.strategy) {
  return {
    name: source.name,
    url: source.url,
    source_type: source.source_type,
    provider: source.provider,
    sync_enabled: source.sync_enabled,
    sync_interval_minutes: source.sync_interval_minutes,
    strategy,
    sidecar_enabled: source.sidecar_enabled,
    runtime: source.runtime,
    port_start: source.port_start,
    port_end: source.port_end,
    scan_enabled: source.scan_enabled,
    scan_interval_minutes: source.scan_interval_minutes,
    health_check_interval_minutes: source.health_check_interval_minutes,
    reputation_provider: source.reputation_provider,
    reputation_api_key_ref: source.reputation_api_key_ref,
    status: source.status
  }
}

function reasonCategory(reason: string) {
  if (!reason) return ''
  if (reason.includes('纯净度')) return '纯净度不足'
  if (reason.includes('延迟')) return '延迟过高'
  if (reason.includes('国家') || reason.includes('屏蔽')) return '国家策略'
  if (reason.includes('最大导入')) return '数量上限'
  if (reason.includes('休眠')) return '休眠中'
  if (reason.includes('缺失')) return '订阅缺失'
  if (reason.includes('sidecar')) return 'sidecar 配置'
  if (reason.includes('备用')) return '备用保留'
  if (reason.includes('选中')) return '已选中'
  return '其它'
}

function sortNodeRows(rows: NodePreviewRow[]) {
  const copy = [...rows]
  if (nodeSortKey.value === 'score') {
    return copy.sort((a, b) => b.score - a.score || a.node_key.localeCompare(b.node_key))
  }
  if (nodeSortKey.value === 'latency') {
    return copy.sort((a, b) => (a.latency_ms ?? Number.MAX_SAFE_INTEGER) - (b.latency_ms ?? Number.MAX_SAFE_INTEGER) || b.score - a.score)
  }
  if (nodeSortKey.value === 'clean') {
    return copy.sort((a, b) => (b.ip_clean_score ?? -1) - (a.ip_clean_score ?? -1) || b.score - a.score)
  }
  if (nodeSortKey.value === 'country') {
    return copy.sort((a, b) => a.preview_country.localeCompare(b.preview_country) || b.score - a.score)
  }
  if (nodeSortKey.value === 'name') {
    return copy.sort((a, b) => (a.name || a.server).localeCompare(b.name || b.server))
  }
  return copy
}

function nodeCountry(node: ProxySubscriptionNode) {
  return (node.exit_country_code || node.country_hint || node.exit_country || '').trim().toUpperCase()
}

function nodeIsSleeping(node: ProxySubscriptionNode) {
  return Boolean(node.sleep_until && new Date(node.sleep_until).getTime() > Date.now())
}

function countryListHas(country: string, list: string[]) {
  return list.map(item => item.trim().toUpperCase()).includes(country.trim().toUpperCase())
}

function nodeBaseExclusionReason(node: ProxySubscriptionNode, strategy: ProxySubscriptionStrategy, source: ProxySubscriptionSource | null) {
  const country = nodeCountry(node)
  if (node.status === 'missing') return '订阅源最新扫描中已缺失'
  if (source && node.sidecar_required && !source.sidecar_enabled) return '订阅源未启用 sidecar'
  if (nodeIsSleeping(node)) return `节点休眠到 ${formatDate(node.sleep_until)}`
  if (country && countryListHas(country, strategy.blocked_countries || [])) return `国家 ${country} 已屏蔽`
  if (strategy.min_ip_clean_score > 0 && (node.ip_clean_score == null || node.ip_clean_score < strategy.min_ip_clean_score)) {
    return node.ip_clean_score == null ? '缺少纯净度结果' : `纯净度 ${node.ip_clean_score} 低于 ${strategy.min_ip_clean_score}`
  }
  if (strategy.min_quality_score > 0 && node.score < strategy.min_quality_score) return `综合分 ${node.score} 低于 ${strategy.min_quality_score}`
  if (strategy.max_latency_ms > 0 && node.latency_ms != null && node.latency_ms > strategy.max_latency_ms) {
    return `延迟 ${node.latency_ms}ms 高于 ${strategy.max_latency_ms}ms`
  }
  return ''
}

function previewSubscriptionNodes(nodes: ProxySubscriptionNode[], rawStrategy: ProxySubscriptionStrategy, source: ProxySubscriptionSource | null): NodePreviewRow[] {
  const strategy = {
    ...cloneStrategy(rawStrategy),
    preferred_countries: parseCountryList(nodePreferredCountriesText.value),
    blocked_countries: parseCountryList(nodeBlockedCountriesText.value)
  }
  const baseRows = nodes.map((node, index) => ({
    ...node,
    preview_status: 'candidate' as NodePreviewStatus,
    preview_reason: '符合基础阈值，未进入选中池',
    preview_rank: index + 1,
    preview_country: nodeCountry(node) || 'unknown'
  }))
  const eligible = baseRows.filter(row => {
    const reason = nodeBaseExclusionReason(row, strategy, source)
    if (reason) {
      row.preview_status = row.status === 'missing' ? 'missing' : nodeIsSleeping(row) ? 'sleeping' : 'excluded'
      row.preview_reason = reason
      return false
    }
    return true
  })
  eligible.sort((a, b) => {
    if (a.score !== b.score) return b.score - a.score
    return a.node_key.localeCompare(b.node_key)
  })
  const selectedKeys = new Set<string>()
  const perCountry = new Map<string, number>()
  let countryCount = 0
  const normalizeCountry = (country: string) => country || 'unknown'
  const canSelect = (row: NodePreviewRow) => {
    const country = normalizeCountry(row.preview_country)
    if (strategy.max_nodes_per_country > 0 && (perCountry.get(country) || 0) >= strategy.max_nodes_per_country) {
      row.preview_reason = `国家 ${country} 已达到每国 ${strategy.max_nodes_per_country} 个上限`
      return false
    }
    if ((perCountry.get(country) || 0) === 0 && strategy.max_country_count > 0 && countryCount >= strategy.max_country_count) {
      row.preview_reason = `已达到最多 ${strategy.max_country_count} 个国家`
      return false
    }
    return true
  }
  const selectRow = (row: NodePreviewRow, reason: string) => {
    if (selectedKeys.size >= strategy.max_enabled_nodes) {
      row.preview_reason = `已达到最大导入 ${strategy.max_enabled_nodes} 个`
      return false
    }
    if (!canSelect(row)) return false
    const country = normalizeCountry(row.preview_country)
    selectedKeys.add(row.node_key)
    if ((perCountry.get(country) || 0) === 0) countryCount++
    perCountry.set(country, (perCountry.get(country) || 0) + 1)
    row.preview_status = 'selected'
    row.preview_reason = reason
    return true
  }
  if (strategy.min_country_count > 1) {
    for (const row of eligible) {
      if (selectedKeys.size >= strategy.min_country_count) break
      if ((perCountry.get(normalizeCountry(row.preview_country)) || 0) > 0) continue
      selectRow(row, '为满足最低国家覆盖优先选中')
    }
  }
  for (const row of eligible) {
    if (selectedKeys.size >= strategy.max_enabled_nodes) break
    if (selectedKeys.has(row.node_key)) continue
    selectRow(row, '按分数和国家配额选中')
  }
  let standbyCount = 0
  const markStandby = (row: NodePreviewRow) => {
    if (standbyCount >= strategy.standby_nodes || selectedKeys.has(row.node_key) || row.preview_status !== 'candidate') return
    row.preview_status = 'standby'
    row.preview_reason = '作为替换候补保留'
    standbyCount++
  }
  if (strategy.replace_same_country_first) {
    for (const row of eligible) {
      if ((perCountry.get(normalizeCountry(row.preview_country)) || 0) > 0) markStandby(row)
      if (standbyCount >= strategy.standby_nodes) break
    }
  }
  for (const row of eligible) {
    markStandby(row)
    if (standbyCount >= strategy.standby_nodes) break
  }
  eligible.forEach((row, index) => {
    row.preview_rank = index + 1
  })
  return baseRows.sort((a, b) => {
    const statusOrder: Record<NodePreviewStatus, number> = { selected: 1, standby: 2, candidate: 3, sleeping: 4, excluded: 5, missing: 6 }
    if (statusOrder[a.preview_status] !== statusOrder[b.preview_status]) return statusOrder[a.preview_status] - statusOrder[b.preview_status]
    if (a.score !== b.score) return b.score - a.score
    return a.node_key.localeCompare(b.node_key)
  })
}

function nodePreviewStatusLabel(status: NodePreviewStatus) {
  const labels: Record<NodePreviewStatus, string> = {
    selected: '可选中',
    standby: '备用',
    candidate: '候选',
    excluded: '不可选',
    sleeping: '休眠',
    missing: '缺失'
  }
  return labels[status]
}

function nodePreviewBadgeClass(status: NodePreviewStatus) {
  if (status === 'selected') return 'badge-success'
  if (status === 'standby') return 'badge-warning'
  return 'badge-gray'
}

function applyNodeStrategyPreset(preset: NodeStrategyPreset) {
  if (preset === 'stable') {
    nodeStrategyDraft.max_latency_ms = 1800
    nodeStrategyDraft.min_quality_score = 70
    nodeStrategyDraft.min_ip_clean_score = 70
    nodeStrategyDraft.timeout_sleep_after = 2
    nodeStrategyDraft.standby_nodes = Math.max(nodeStrategyDraft.standby_nodes, 3)
    return
  }
  if (preset === 'speed') {
    nodeStrategyDraft.max_latency_ms = 800
    nodeStrategyDraft.min_quality_score = 65
    nodeStrategyDraft.min_ip_clean_score = 40
    nodeStrategyDraft.max_nodes_per_country = Math.max(nodeStrategyDraft.max_nodes_per_country, 5)
    return
  }
  if (preset === 'coverage') {
    nodeStrategyDraft.min_country_count = 4
    nodeStrategyDraft.max_country_count = 10
    nodeStrategyDraft.max_nodes_per_country = 2
    nodeStrategyDraft.standby_nodes = Math.max(nodeStrategyDraft.standby_nodes, 4)
    return
  }
  nodeStrategyDraft.scan_batch_size = 3
  nodeStrategyDraft.max_probe_concurrency = 2
  nodeStrategyDraft.scan_budget_minutes = 45
  nodeStrategyDraft.scan_budget_max_minutes = 55
  nodeStrategyDraft.resource_adaptive_scan = true
}

function formatScanResultSummary(result: Record<string, unknown> | undefined) {
  if (!result) return '-'
  const parsed = readScanNumber(result, 'parsed')
  const saved = readScanNumber(result, 'saved')
  const selected = readScanNumber(result, 'selected')
  const errors = readScanNumber(result, 'errors')
  return `解析 ${parsed ?? '-'} / 入库 ${saved ?? '-'} / 选中 ${selected ?? '-'} / 错误 ${errors ?? 0}`
}

function isScanning(id: number) {
  return scanningSubscriptionIds.value.includes(id)
}

function replaceScanningSource(id: number | null, startedAt?: string | null) {
  scanningSubscriptionIds.value = id ? [id] : []
  for (const key of Object.keys(scanStartedAtMap)) {
    delete scanStartedAtMap[Number(key)]
  }
  if (id) {
    scanStartedAtMap[id] = startedAt ? new Date(startedAt).getTime() : Date.now()
    scanNow.value = Date.now()
    startScanTicker()
  } else {
    stopScanTickerIfIdle()
  }
}

function startScanTicker() {
  if (scanTicker) return
  scanTicker = setInterval(() => {
    scanNow.value = Date.now()
  }, 1000)
}

function stopScanTickerIfIdle() {
  if (scanningSubscriptionIds.value.length > 0 || !scanTicker) return
  clearInterval(scanTicker)
  scanTicker = null
}

function markScanning(id: number, active: boolean) {
  if (active) {
    if (!isScanning(id)) {
      scanningSubscriptionIds.value = [...scanningSubscriptionIds.value, id]
    }
    if (!scanStartedAtMap[id]) {
      scanStartedAtMap[id] = Date.now()
    }
    scanNow.value = Date.now()
    startScanTicker()
    return
  }
  scanningSubscriptionIds.value = scanningSubscriptionIds.value.filter(item => item !== id)
  delete scanStartedAtMap[id]
  stopScanTickerIfIdle()
}

async function syncScanStatus() {
  try {
    const status = await adminAPI.proxies.getProxySubscriptionScanStatus()
    serverScanStatus.value = status
    if (status.active && status.source_id) {
      replaceScanningSource(status.source_id, status.started_at)
    } else {
      replaceScanningSource(null)
    }
  } catch {
    // Keep the local in-flight state if polling fails once.
  }
}

function startScanStatusPolling() {
  if (scanStatusPoller) return
  scanStatusPoller = setInterval(() => {
    void syncScanStatus()
  }, 3000)
}

function stopScanStatusPolling() {
  if (!scanStatusPoller) return
  clearInterval(scanStatusPoller)
  scanStatusPoller = null
}

function readScanNumber(result: Record<string, unknown> | undefined, key: string) {
  if (!result) return null
  const value = result[key]
  return typeof value === 'number' ? value : null
}

function proxyImportItemDisabledReason(item: ProxyImportPreviewItem) {
  if (!item.valid) return item.error || '节点格式无效，不能导入'
  if (item.duplicate) return '该节点已存在于代理池中，已跳过'
  if (item.sidecar_required) return '该节点需通过代理订阅源接入，不能直接导入普通代理池'
  return ''
}

function formatDurationMs(durationMs: number) {
  const totalSeconds = Math.max(0, Math.floor(durationMs / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes <= 0) return `${seconds} 秒`
  if (minutes < 60) return `${minutes} 分 ${seconds.toString().padStart(2, '0')} 秒`
  const hours = Math.floor(minutes / 60)
  const remainMinutes = minutes % 60
  return `${hours} 小时 ${remainMinutes} 分`
}

function formatScanElapsed(id: number) {
  const startedAt = scanStartedAtMap[id]
  if (!startedAt) return '刚开始'
  return formatDurationMs(scanNow.value - startedAt)
}

function formatSourceScanEstimate(source: ProxySubscriptionSource) {
  const minMinutes = source.strategy?.scan_budget_minutes || 30
  const maxMinutes = source.strategy?.scan_budget_max_minutes || Math.max(40, minMinutes)
  if (maxMinutes <= minMinutes) return `${minMinutes} 分钟内`
  return `${minMinutes}-${maxMinutes} 分钟`
}

function formatActiveScanEstimate() {
  const source = activeScanningSource.value
  if (source) return formatSourceScanEstimate(source)
  const status = serverScanStatus.value
  if (!status?.active) return '-'
  const minMinutes = status.scan_budget_minutes || 30
  const maxMinutes = status.scan_budget_max_minutes || Math.max(40, minMinutes)
  if (maxMinutes <= minMinutes) return `${minMinutes} 分钟内`
  return `${minMinutes}-${maxMinutes} 分钟`
}

function isBusyScanError(error: any) {
  const reason = error?.response?.data?.reason || error?.response?.data?.error?.reason || ''
  const detail = error?.response?.data?.detail || error?.message || ''
  return reason === 'PROXY_SUBSCRIPTION_SCAN_BUSY' || String(detail).includes('PROXY_SUBSCRIPTION_SCAN_BUSY')
}

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
    const sidecarOnly = preview.value.sidecar_only || 0
    const messageParts = [`已导入 ${result.created} 个代理`]
    if (result.skipped > 0) {
      messageParts.push(`跳过 ${result.skipped} 个`)
    }
    if (sidecarOnly > 0) {
      messageParts.push(`${sidecarOnly} 个 sidecar 节点请通过代理订阅源接入`)
    }
    appStore.showSuccess(messageParts.join('，'))
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

async function loadNodeViewFromRoute() {
  const id = Number(route.params.subscriptionId)
  if (!id) return
  const source = subscriptions.value.find(item => item.id === id)
  if (!source) {
    await loadSubscriptions()
  }
  const resolved = subscriptions.value.find(item => item.id === id)
  if (resolved) {
    await openSubscriptionNodes(resolved, false)
  } else {
    appStore.showError('订阅源不存在或已删除')
    await router.replace('/admin/proxy-dispatch')
  }
}

async function createSubscription() {
  try {
    await adminAPI.proxies.createProxySubscription(buildSubscriptionPayload())
    resetSubscriptionForm()
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

async function scanSubscription(id: number) {
  if (isScanning(id)) {
    appStore.showWarning?.('该订阅源正在扫描中，请等待当前任务完成后再试')
    return
  }
  markScanning(id, true)
  try {
    scanResult.value = await adminAPI.proxies.scanProxySubscription(id)
    await loadSubscriptions()
    if (selectedNodeSource.value?.id === id) {
      const refreshed = subscriptions.value.find(source => source.id === id)
      if (refreshed) selectedNodeSource.value = refreshed
      subscriptionNodes.value = await adminAPI.proxies.listProxySubscriptionNodes(id)
    }
    appStore.showSuccess(`扫描完成，已选中 ${scanResult.value.selected} 个节点`)
  } catch (error: any) {
    if (isBusyScanError(error)) {
      await syncScanStatus()
      const scanningSource = activeScanningSource.value
      const detail = scanningSource
        ? `当前正在扫描“${scanningSource.name}”，已运行 ${formatScanElapsed(scanningSource.id)}，请稍后再试`
        : '当前已有订阅扫描任务在运行，请等待其完成后再试'
      appStore.showWarning?.(detail)
    } else {
      appStore.showError(error?.message || '扫描订阅失败')
    }
  } finally {
    markScanning(id, false)
    await syncScanStatus()
  }
}

async function openSubscriptionNodes(source: ProxySubscriptionSource, pushRoute = true) {
  try {
    if (pushRoute) {
      openingNodesRoute = true
      try {
        await router.push(`/admin/proxy-dispatch/subscriptions/${source.id}/nodes`)
      } finally {
        openingNodesRoute = false
      }
    }
    nodesDialogTitle.value = `${source.name} 节点`
    selectedNodeSource.value = source
    applyNodeStrategyDraft(source.strategy)
    nodeStatusFilter.value = ''
    subscriptionNodes.value = await adminAPI.proxies.listProxySubscriptionNodes(source.id)
    showNodesDialog.value = true
  } catch (error: any) {
    appStore.showError(error?.message || '加载节点失败')
  }
}

async function closeSubscriptionNodes() {
  showNodesDialog.value = false
  selectedNodeSource.value = null
  subscriptionNodes.value = []
  nodeStatusFilter.value = ''
  nodeReasonFilter.value = ''
  if (route.name === 'AdminProxyDispatchSubscriptionNodes') {
    await router.push('/admin/proxy-dispatch')
  }
}

async function saveNodeStrategy(scanAfterSave: boolean) {
  if (!selectedNodeSource.value) return
  const source = selectedNodeSource.value
  savingNodeStrategy.value = true
  try {
    const payload = buildSubscriptionUpdatePayload(source, buildNodeStrategyPayload())
    const updated = await adminAPI.proxies.updateProxySubscription(source.id, payload)
    selectedNodeSource.value = updated
    const index = subscriptions.value.findIndex(item => item.id === updated.id)
    if (index >= 0) {
      subscriptions.value.splice(index, 1, updated)
    }
    appStore.showSuccess(scanAfterSave ? '策略已保存，开始扫描' : '策略已保存')
    if (scanAfterSave) {
      await scanSubscription(updated.id)
    }
  } catch (error: any) {
    appStore.showError(error?.message || '保存节点策略失败')
  } finally {
    savingNodeStrategy.value = false
  }
}

async function deleteSubscription(id: number) {
  try {
    await adminAPI.proxies.deleteProxySubscription(id)
    await loadSubscriptions()
    appStore.showSuccess('订阅源已删除')
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
  await loadNodeViewFromRoute()
  await syncScanStatus()
  startScanStatusPolling()
})

watch(
  () => route.params.subscriptionId,
  async (id) => {
    if (openingNodesRoute) return
    if (id) {
      await loadNodeViewFromRoute()
    } else if (showNodesDialog.value) {
      showNodesDialog.value = false
      selectedNodeSource.value = null
      subscriptionNodes.value = []
    }
  }
)

onUnmounted(() => {
  stopScanStatusPolling()
  if (scanTicker) {
    clearInterval(scanTicker)
    scanTicker = null
  }
})
</script>
