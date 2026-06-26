<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">代理出口管理</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">授权目标的订阅、节点、策略、运行槽位和任务审计。</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadAll">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 md:grid-cols-5">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">启用订阅</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status.subscriptions_active }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">健康节点</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ status.healthy_nodes }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">策略</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ status.policies_total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">运行槽位</p>
          <p class="mt-2 text-2xl font-semibold text-sky-600 dark:text-sky-400">{{ status.slots_assigned }} / {{ status.slots_total }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">24h 错误</p>
          <p class="mt-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">{{ status.recent_errors }}</p>
        </div>
      </section>

      <nav class="flex gap-2 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium"
          :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
        </button>
      </nav>

      <section v-if="activeTab === 'subscriptions'" class="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="card p-5">
          <div class="flex items-center justify-between gap-3">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingSubscriptionID ? '编辑订阅' : '导入订阅' }}</h2>
            <button v-if="editingSubscriptionID" type="button" class="btn btn-secondary btn-sm" @click="resetSubscriptionForm">取消</button>
          </div>
          <div class="mt-4 space-y-4">
            <label class="block">
              <span class="input-label">名称</span>
              <input v-model.trim="subscriptionForm.name" class="input" placeholder="IEPL 节点池" />
            </label>
            <label class="block">
              <span class="input-label">类型</span>
              <select v-model="subscriptionForm.subscription_type" class="input">
                <option value="clash">Clash / Mihomo</option>
                <option value="shadowrocket">Shadowrocket</option>
                <option value="v2ray_ss">V2Ray + SS</option>
              </select>
            </label>
            <label class="block">
              <span class="input-label">订阅链接</span>
              <textarea v-model.trim="subscriptionForm.subscription_url" class="input min-h-[96px] font-mono text-xs" :placeholder="editingSubscriptionID ? '留空则不修改订阅链接' : ''" />
            </label>
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="subscriptionForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              启用订阅
            </label>
            <div class="grid grid-cols-2 gap-3">
              <label class="block">
                <span class="input-label">刷新秒数</span>
                <input v-model.number="subscriptionForm.refresh_interval_seconds" type="number" min="60" class="input" />
              </label>
              <label class="flex items-end gap-2 pb-2 text-sm text-gray-700 dark:text-dark-200">
                <input v-model="subscriptionForm.refresh_now" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                立即刷新
              </label>
            </div>
            <button type="button" class="btn btn-primary w-full" :disabled="savingSubscription" @click="saveSubscriptionNow">
              <Icon name="plus" size="sm" />
              {{ editingSubscriptionID ? '保存订阅' : '导入' }}
            </button>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">代理订阅</h2>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[960px] divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-800">
                <tr>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">名称</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">类型</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">状态</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">节点</th>
                  <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">刷新</th>
                  <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                <tr v-if="subscriptions.length === 0">
                  <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500">暂无订阅</td>
                </tr>
                <tr v-for="item in subscriptions" :key="item.id">
                  <td class="px-4 py-4">
                    <div class="font-medium text-gray-900 dark:text-white">{{ item.name }}</div>
                    <div class="mt-1 font-mono text-xs text-gray-500">{{ item.active_config_version || '-' }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ subscriptionTypeLabel(item.subscription_type) }}</td>
                  <td class="px-4 py-4">
                    <span class="badge" :class="refreshClass(item.last_refresh_status)">{{ refreshLabel(item.last_refresh_status) }}</span>
                    <div v-if="item.last_refresh_error" class="mt-1 max-w-[260px] truncate text-xs text-rose-500">{{ item.last_refresh_error }}</div>
                  </td>
                  <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ item.node_count }}</td>
                  <td class="px-4 py-4 text-xs text-gray-500">{{ formatDateTime(item.last_refreshed_at) }}</td>
                  <td class="px-4 py-4 text-right">
                    <div class="flex justify-end gap-2">
                      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="editSubscription(item)">编辑</button>
                      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="refreshSubscriptionNow(item.id)">
                        <Icon name="refresh" size="sm" />
                        刷新
                      </button>
                      <button type="button" class="btn btn-danger btn-sm" :disabled="loading" @click="deleteSubscriptionNow(item)">删除</button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section v-else-if="activeTab === 'nodes'" class="card overflow-hidden">
        <div class="grid gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 md:grid-cols-[1fr_180px_auto] md:items-end">
          <label class="block">
            <span class="input-label">关键字</span>
            <input v-model.trim="nodeFilters.q" class="input" placeholder="节点名 / 地区 / 协议" @keyup.enter="loadNodes" />
          </label>
          <label class="block">
            <span class="input-label">健康状态</span>
            <select v-model="nodeFilters.health_status" class="input" @change="loadNodes">
              <option value="">全部</option>
              <option value="healthy">健康</option>
              <option value="unknown">未知</option>
              <option value="degraded">降级</option>
              <option value="unhealthy">不可用</option>
              <option value="disabled">停用</option>
            </select>
          </label>
          <button type="button" class="btn btn-primary" :disabled="loading" @click="loadNodes">
            <Icon name="search" size="sm" />
            查询
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1120px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">节点</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">协议</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">地区</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">延迟</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">出口 IP</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="nodes.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500">暂无节点</td>
              </tr>
              <tr v-for="node in nodes" :key="node.id">
                <td class="px-4 py-4">
                  <div class="max-w-[360px] truncate font-medium text-gray-900 dark:text-white">{{ node.display_name }}</div>
                  <div class="mt-1 font-mono text-xs text-gray-500">{{ node.node_key.slice(0, 12) }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ node.protocol }}</td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ node.region || '-' }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="healthClass(node.health_status)">{{ healthLabel(node.health_status) }}</span>
                  <div v-if="node.last_error_message" class="mt-1 max-w-[260px] truncate text-xs text-rose-500">{{ node.last_error_message }}</div>
                </td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ node.last_latency_ms ?? '-' }}</td>
                <td class="px-4 py-4 font-mono text-xs text-gray-600 dark:text-dark-300">{{ node.last_egress_ip || '-' }}</td>
                <td class="px-4 py-4 text-right">
                  <div class="flex justify-end gap-2">
                    <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="checkNodeNow(node.id)">检测</button>
                    <button v-if="node.health_status === 'disabled'" type="button" class="btn btn-secondary btn-sm" @click="enableNodeNow(node.id)">启用</button>
                    <button v-else type="button" class="btn btn-secondary btn-sm" @click="disableNodeNow(node.id)">停用</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-else-if="activeTab === 'policies'" class="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="space-y-6">
          <div class="card p-5">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingPolicyID ? '编辑策略' : '代理策略' }}</h2>
              <button v-if="editingPolicyID" type="button" class="btn btn-secondary btn-sm" @click="resetPolicyForm">取消</button>
            </div>
            <div class="mt-4 space-y-4">
              <label class="block">
                <span class="input-label">名称</span>
                <input v-model.trim="policyForm.name" class="input" placeholder="授权采集默认策略" />
              </label>
              <label class="block">
                <span class="input-label">订阅</span>
                <select v-model="policyForm.subscription_id" class="input">
                  <option :value="0">请选择</option>
                  <option v-for="item in subscriptions" :key="item.id" :value="item.id">{{ item.name }}</option>
                </select>
              </label>
              <div class="grid grid-cols-2 gap-3">
                <label class="block">
                  <span class="input-label">并发槽位</span>
                  <input v-model.number="policyForm.max_concurrency" type="number" min="1" class="input" />
                </label>
                <label class="block">
                  <span class="input-label">切换预算</span>
                  <input v-model.number="policyForm.max_switches_per_task" type="number" min="0" class="input" />
                </label>
              </div>
              <label class="block">
                <span class="input-label">地区偏好</span>
                <input v-model.trim="policyForm.preferred_regions_text" class="input" placeholder="HK, JP, US" />
              </label>
              <label class="block">
                <span class="input-label">出口模式</span>
                <select v-model="policyForm.selection_mode" class="input">
                  <option value="auto">自动选择健康节点</option>
                  <option value="fixed">固定指定节点</option>
                </select>
              </label>
              <label v-if="policyForm.selection_mode === 'fixed'" class="block">
                <span class="input-label">固定出口节点</span>
                <select v-model.number="policyForm.fixed_node_id" class="input">
                  <option :value="0">请选择节点</option>
                  <option v-for="node in fixedNodeOptions" :key="node.id" :value="node.id">
                    {{ node.display_name }} · {{ node.region || '-' }} · {{ node.last_egress_ip || '未检测出口 IP' }}
                  </option>
                </select>
              </label>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                <input v-model="policyForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                启用策略
              </label>
              <button type="button" class="btn btn-primary w-full" :disabled="savingPolicy" @click="savePolicyNow">
                <Icon name="plus" size="sm" />
                {{ editingPolicyID ? '保存策略' : '创建策略' }}
              </button>
            </div>
          </div>

          <div class="card p-5">
            <div class="flex items-center justify-between gap-3">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingTargetID ? '编辑白名单' : '目标白名单' }}</h2>
              <button v-if="editingTargetID" type="button" class="btn btn-secondary btn-sm" @click="resetTargetForm">取消</button>
            </div>
            <div class="mt-4 space-y-4">
              <label class="block">
                <span class="input-label">策略</span>
                <select v-model="targetForm.policy_id" class="input" @change="loadTargets">
                  <option :value="0">请选择</option>
                  <option v-for="item in policies" :key="item.id" :value="item.id">{{ item.name }}</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">目标 Host</span>
                <input v-model.trim="targetForm.target_host" class="input font-mono" placeholder="example.com 或 *.example.com" />
              </label>
              <label class="block">
                <span class="input-label">用途</span>
                <select v-model="targetForm.purpose" class="input">
                  <option value="site_discovery">授权采集</option>
                  <option value="registration">授权注册</option>
                  <option value="supplier_probe">供应商探测</option>
                  <option value="manual_test">人工测试</option>
                </select>
              </label>
              <div class="grid grid-cols-2 gap-3">
                <label class="block">
                  <span class="input-label">允许方法</span>
                  <input v-model.trim="targetForm.allowed_methods_text" class="input" placeholder="GET, POST" />
                </label>
                <label class="block">
                  <span class="input-label">频率 / 分钟</span>
                  <input v-model.number="targetForm.rate_limit_per_minute" type="number" min="1" class="input" />
                </label>
              </div>
              <label class="block">
                <span class="input-label">授权备注</span>
                <input v-model.trim="targetForm.authorization_note" class="input" />
              </label>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                <input v-model="targetForm.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
                启用白名单
              </label>
              <button type="button" class="btn btn-primary w-full" :disabled="savingTarget" @click="saveTargetNow">
                <Icon name="plus" size="sm" />
                {{ editingTargetID ? '保存白名单' : '添加白名单' }}
              </button>
            </div>
          </div>
        </div>

        <div class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">策略列表</h2>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="policies.length === 0" class="px-5 py-8 text-center text-sm text-gray-500">暂无策略</div>
              <div v-for="policy in policies" :key="policy.id" class="px-5 py-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div class="font-medium text-gray-900 dark:text-white">{{ policy.name }}</div>
                    <div class="mt-1 text-xs text-gray-500">订阅 {{ policy.subscription_ids.join(', ') || '-' }} · 白名单 {{ policy.enabled_targets || 0 }}</div>
                    <div class="mt-1 text-xs text-gray-500">{{ policySelectionLabel(policy) }}</div>
                  </div>
                  <div class="flex items-center gap-2">
                    <span class="badge" :class="policy.enabled ? 'badge-success' : 'badge-gray'">{{ policy.enabled ? '启用' : '停用' }}</span>
                    <button type="button" class="btn btn-secondary btn-sm" @click="editPolicy(policy)">编辑</button>
                    <button type="button" class="btn btn-danger btn-sm" @click="deletePolicyNow(policy)">删除</button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="card overflow-hidden">
            <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">白名单</h2>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] divide-y divide-gray-200 dark:divide-dark-700">
                <thead class="bg-gray-50 dark:bg-dark-800">
                  <tr>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Host</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">用途</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">方法</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">频率</th>
                    <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">备注</th>
                    <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">操作</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
                  <tr v-if="targets.length === 0">
                    <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-500">暂无白名单</td>
                  </tr>
                  <tr v-for="target in targets" :key="target.id">
                    <td class="px-4 py-4 font-mono text-sm text-gray-900 dark:text-white">{{ target.target_host }}</td>
                    <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ purposeLabel(target.purpose) }}</td>
                    <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ target.allowed_methods.join(', ') }}</td>
                    <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ target.rate_limit_per_minute }}/min</td>
                    <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ target.authorization_note || '-' }}</td>
                    <td class="px-4 py-4 text-right">
                      <div class="flex justify-end gap-2">
                        <button type="button" class="btn btn-secondary btn-sm" @click="editTarget(target)">编辑</button>
                        <button type="button" class="btn btn-danger btn-sm" @click="deleteTargetNow(target)">删除</button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="activeTab === 'runtime'" class="grid gap-6 xl:grid-cols-2">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">运行槽位</h2>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="slots.length === 0" class="px-5 py-8 text-center text-sm text-gray-500">暂无槽位</div>
            <div v-for="slot in slots" :key="slot.id" class="px-5 py-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">{{ slot.slot_key }}</div>
                  <div class="mt-1 font-mono text-xs text-gray-500">mixed {{ slot.mixed_port }} · controller {{ slot.controller_port }}</div>
                  <div class="mt-1 text-xs text-gray-500">{{ slot.assigned_task_type || '-' }} / {{ slot.assigned_task_id || '-' }}</div>
                </div>
                <div class="flex items-center gap-2">
                  <span class="badge" :class="slotClass(slot.status)">{{ slotLabel(slot.status) }}</span>
                  <button type="button" class="btn btn-secondary btn-sm" @click="restartSlotNow(slot.id)">重启</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">任务绑定</h2>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="assignments.length === 0" class="px-5 py-8 text-center text-sm text-gray-500">暂无绑定</div>
            <div v-for="assignment in assignments" :key="assignment.id" class="px-5 py-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">{{ assignment.task_type }} #{{ assignment.task_id }}</div>
                  <div class="mt-1 font-mono text-xs text-gray-500">{{ assignment.target_host }} · slot {{ assignment.slot_id }} · node {{ assignment.node_id || '-' }}</div>
                  <div class="mt-1 text-xs text-gray-500">{{ assignment.egress_ip || '-' }} · switch {{ assignment.switch_count }}</div>
                </div>
                <div class="flex items-center gap-2">
                  <span class="badge" :class="assignmentClass(assignment.status)">{{ assignmentLabel(assignment.status) }}</span>
                  <select
                    v-if="assignment.status === 'active'"
                    v-model.number="assignmentSwitchNodeIDs[assignment.id]"
                    class="input h-9 w-64 py-1 text-sm"
                  >
                    <option :value="0">选择出口节点</option>
                    <option v-for="node in eligibleNodesForAssignment(assignment)" :key="node.id" :value="node.id">
                      {{ node.display_name }} · {{ node.last_egress_ip || node.region || '-' }}
                    </option>
                  </select>
                  <button v-if="assignment.status === 'active'" type="button" class="btn btn-primary btn-sm" @click="switchAssignmentNow(assignment)">切换出口</button>
                  <button v-if="assignment.status === 'active'" type="button" class="btn btn-secondary btn-sm" @click="releaseAssignmentNow(assignment.id)">释放</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-else class="card overflow-hidden">
        <div class="grid gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 md:grid-cols-[180px_1fr_auto] md:items-end">
          <label class="block">
            <span class="input-label">级别</span>
            <select v-model="auditFilters.level" class="input" @change="loadAudits">
              <option value="">全部</option>
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="error">Error</option>
            </select>
          </label>
          <label class="block">
            <span class="input-label">目标 Host</span>
            <input v-model.trim="auditFilters.target_host" class="input" @keyup.enter="loadAudits" />
          </label>
          <button type="button" class="btn btn-primary" @click="loadAudits">
            <Icon name="search" size="sm" />
            查询
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[1120px] divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">级别</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">事件</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">任务</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">目标</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">消息</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-if="audits.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500">暂无审计事件</td>
              </tr>
              <tr v-for="audit in audits" :key="audit.id">
                <td class="whitespace-nowrap px-4 py-4 text-sm text-gray-500">{{ formatDateTime(audit.created_at) }}</td>
                <td class="px-4 py-4">
                  <span class="badge" :class="auditClass(audit.level)">{{ audit.level }}</span>
                </td>
                <td class="px-4 py-4 font-mono text-xs text-gray-900 dark:text-white">{{ audit.event_type }}</td>
                <td class="px-4 py-4 text-sm text-gray-600 dark:text-dark-300">{{ audit.task_type || '-' }} {{ audit.task_id || '' }}</td>
                <td class="px-4 py-4 font-mono text-xs text-gray-600 dark:text-dark-300">{{ audit.target_host || '-' }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ audit.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  checkProxyNode,
  createProxyPolicy,
  createProxySubscription,
  createProxyTarget,
  deleteProxyPolicy,
  deleteProxySubscription,
  deleteProxyTarget,
  disableProxyNode,
  enableProxyNode,
  getProxyCenterStatus,
  listProxyAssignments,
  listProxyAuditEvents,
  listProxyNodes,
  listProxyPolicies,
  listProxyRuntimeSlots,
  listProxySubscriptions,
  listProxyTargets,
  refreshProxySubscription,
  releaseProxyAssignment,
  restartProxyRuntimeSlot,
  switchProxyAssignment,
  updateProxyPolicy,
  updateProxySubscription,
  updateProxyTarget,
  type ProxyAssignment,
  type ProxyAssignmentStatus,
  type ProxyAuditEvent,
  type ProxyAuditLevel,
  type ProxyCenterStatus,
  type ProxyNode,
  type ProxyNodeHealthStatus,
  type ProxyPolicy,
  type ProxyRefreshStatus,
  type ProxyRuntimeSlot,
  type ProxyRuntimeSlotStatus,
  type ProxySubscription,
  type ProxySubscriptionType,
  type ProxyTargetPolicy,
  type ProxyTaskPurpose
} from '@/api/admin/adminPlus'

type TabValue = 'subscriptions' | 'nodes' | 'policies' | 'runtime' | 'audits'
type PolicySelectionMode = 'auto' | 'fixed'

const appStore = useAppStore()
const loading = ref(false)
const savingSubscription = ref(false)
const savingPolicy = ref(false)
const savingTarget = ref(false)
const editingSubscriptionID = ref(0)
const editingPolicyID = ref(0)
const editingTargetID = ref(0)
const activeTab = ref<TabValue>('subscriptions')
const assignmentSwitchNodeIDs = reactive<Record<number, number>>({})

const tabs: Array<{ value: TabValue; label: string }> = [
  { value: 'subscriptions', label: '订阅' },
  { value: 'nodes', label: '节点池' },
  { value: 'policies', label: '策略' },
  { value: 'runtime', label: '运行槽位' },
  { value: 'audits', label: '审计' }
]

const status = reactive<ProxyCenterStatus>({
  subscriptions_total: 0,
  subscriptions_active: 0,
  nodes_total: 0,
  healthy_nodes: 0,
  policies_total: 0,
  targets_total: 0,
  slots_total: 0,
  slots_assigned: 0,
  assignments_active: 0,
  recent_errors: 0
})

const subscriptions = ref<ProxySubscription[]>([])
const nodes = ref<ProxyNode[]>([])
const policies = ref<ProxyPolicy[]>([])
const targets = ref<ProxyTargetPolicy[]>([])
const slots = ref<ProxyRuntimeSlot[]>([])
const assignments = ref<ProxyAssignment[]>([])
const audits = ref<ProxyAuditEvent[]>([])

const subscriptionForm = reactive({
  name: '',
  subscription_type: 'clash' as ProxySubscriptionType,
  subscription_url: '',
  enabled: true,
  refresh_interval_seconds: 3600,
  refresh_now: true
})

const nodeFilters = reactive({
  q: '',
  health_status: '' as ProxyNodeHealthStatus | ''
})

const policyForm = reactive({
  name: '',
  subscription_id: 0,
  preferred_regions_text: 'HK, JP, US',
  max_concurrency: 1,
  max_switches_per_task: 2,
  enabled: true,
  selection_mode: 'auto' as PolicySelectionMode,
  fixed_node_id: 0
})

const targetForm = reactive({
  policy_id: 0,
  target_host: '',
  purpose: 'site_discovery' as ProxyTaskPurpose,
  allowed_methods_text: 'GET, POST',
  rate_limit_per_minute: 60,
  authorization_note: '',
  enabled: true
})

const auditFilters = reactive({
  level: '' as ProxyAuditLevel | '',
  target_host: ''
})

const selectedPolicyID = computed(() => targetForm.policy_id || policies.value[0]?.id || 0)
const fixedNodeOptions = computed(() => {
  const subscriptionID = Number(policyForm.subscription_id || 0)
  return nodes.value.filter((node) => {
    if (subscriptionID > 0 && node.subscription_id !== subscriptionID) return false
    return node.health_status !== 'disabled' && node.health_status !== 'unhealthy' && node.health_status !== 'suspect'
  })
})

onMounted(() => {
  void loadAll()
})

watch(activeTab, (tab) => {
  if (tab === 'nodes') void loadNodes()
  if (tab === 'policies') void loadPoliciesAndTargets()
  if (tab === 'runtime') void loadRuntime()
  if (tab === 'audits') void loadAudits()
})

watch(() => policyForm.selection_mode, (mode) => {
  if (mode !== 'fixed') policyForm.fixed_node_id = 0
})

watch(() => policyForm.subscription_id, () => {
  if (policyForm.fixed_node_id && !fixedNodeOptions.value.some((node) => node.id === policyForm.fixed_node_id)) {
    policyForm.fixed_node_id = 0
  }
})

async function loadAll() {
  loading.value = true
  try {
    await Promise.all([
      loadStatus(),
      loadSubscriptions(),
      loadNodes(),
      loadPoliciesAndTargets(),
      loadRuntime(),
      loadAudits()
    ])
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadStatus() {
  Object.assign(status, await getProxyCenterStatus())
}

async function loadSubscriptions() {
  const result = await listProxySubscriptions({ page: 1, page_size: 200 })
  subscriptions.value = result.items || []
}

async function loadNodes() {
  const result = await listProxyNodes({
    page: 1,
    page_size: 300,
    q: nodeFilters.q || undefined,
    health_status: nodeFilters.health_status || undefined,
    include_disabled: true
  })
  nodes.value = result.items || []
}

async function loadPoliciesAndTargets() {
  const result = await listProxyPolicies({ page: 1, page_size: 200 })
  policies.value = result.items || []
  if (!targetForm.policy_id && policies.value[0]) {
    targetForm.policy_id = policies.value[0].id
  }
  await loadTargets()
}

async function loadTargets() {
  if (!selectedPolicyID.value) {
    targets.value = []
    return
  }
  const result = await listProxyTargets(selectedPolicyID.value, { page: 1, page_size: 300 })
  targets.value = result.items || []
}

async function loadRuntime() {
  const [slotResult, assignmentResult] = await Promise.all([
    listProxyRuntimeSlots({ page: 1, page_size: 100 }),
    listProxyAssignments({ page: 1, page_size: 100 })
  ])
  slots.value = slotResult.items || []
  assignments.value = assignmentResult.items || []
  assignments.value.forEach((assignment) => {
    assignmentSwitchNodeIDs[assignment.id] = assignment.node_id || assignmentSwitchNodeIDs[assignment.id] || 0
  })
}

async function loadAudits() {
  const result = await listProxyAuditEvents({
    page: 1,
    page_size: 100,
    level: auditFilters.level || undefined,
    target_host: auditFilters.target_host || undefined
  })
  audits.value = result.items || []
}

async function createSubscriptionNow() {
  if (!subscriptionForm.name || !subscriptionForm.subscription_url) {
    appStore.showError('请填写订阅名称和链接')
    return
  }
  savingSubscription.value = true
  try {
    await createProxySubscription({
      name: subscriptionForm.name,
      subscription_type: subscriptionForm.subscription_type,
      subscription_url: subscriptionForm.subscription_url,
      enabled: subscriptionForm.enabled,
      refresh_interval_seconds: subscriptionForm.refresh_interval_seconds,
      refresh_now: subscriptionForm.refresh_now
    })
    resetSubscriptionForm()
    appStore.showSuccess('订阅已导入')
    await loadAll()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingSubscription.value = false
  }
}

async function saveSubscriptionNow() {
  if (!editingSubscriptionID.value) {
    await createSubscriptionNow()
    return
  }
  if (!subscriptionForm.name) {
    appStore.showError('请填写订阅名称')
    return
  }
  savingSubscription.value = true
  try {
    await updateProxySubscription(editingSubscriptionID.value, {
      name: subscriptionForm.name,
      subscription_type: subscriptionForm.subscription_type,
      subscription_url: subscriptionForm.subscription_url || undefined,
      enabled: subscriptionForm.enabled,
      refresh_interval_seconds: subscriptionForm.refresh_interval_seconds
    })
    const id = editingSubscriptionID.value
    const shouldRefresh = subscriptionForm.refresh_now
    resetSubscriptionForm()
    if (shouldRefresh) {
      await refreshProxySubscription(id)
    }
    appStore.showSuccess('订阅已保存')
    await loadAll()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingSubscription.value = false
  }
}

function editSubscription(item: ProxySubscription) {
  editingSubscriptionID.value = item.id
  subscriptionForm.name = item.name
  subscriptionForm.subscription_type = item.subscription_type
  subscriptionForm.subscription_url = ''
  subscriptionForm.enabled = item.enabled
  subscriptionForm.refresh_interval_seconds = item.refresh_interval_seconds
  subscriptionForm.refresh_now = false
}

function resetSubscriptionForm() {
  editingSubscriptionID.value = 0
  subscriptionForm.name = ''
  subscriptionForm.subscription_type = 'clash'
  subscriptionForm.subscription_url = ''
  subscriptionForm.enabled = true
  subscriptionForm.refresh_interval_seconds = 3600
  subscriptionForm.refresh_now = true
}

async function deleteSubscriptionNow(item: ProxySubscription) {
  if (!window.confirm(`确认删除代理订阅「${item.name}」？`)) return
  try {
    await deleteProxySubscription(item.id)
    appStore.showSuccess('订阅已删除')
    if (editingSubscriptionID.value === item.id) resetSubscriptionForm()
    await loadAll()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function refreshSubscriptionNow(id: number) {
  try {
    await refreshProxySubscription(id)
    appStore.showSuccess('订阅刷新完成')
    await loadAll()
  } catch (error) {
    appStore.showError(errorMessage(error))
    await loadSubscriptions()
  }
}

async function checkNodeNow(id: number) {
  try {
    await checkProxyNode(id)
    appStore.showSuccess('节点检测完成')
    await loadNodes()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function disableNodeNow(id: number) {
  try {
    await disableProxyNode(id, 'manual')
    appStore.showSuccess('节点已停用')
    await loadNodes()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function enableNodeNow(id: number) {
  try {
    await enableProxyNode(id)
    appStore.showSuccess('节点已启用')
    await loadNodes()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function createPolicyNow() {
  if (!policyForm.name || !policyForm.subscription_id) {
    appStore.showError('请填写策略名称并选择订阅')
    return
  }
  if (policyForm.selection_mode === 'fixed' && !policyForm.fixed_node_id) {
    appStore.showError('固定出口模式需要选择一个节点')
    return
  }
  savingPolicy.value = true
  try {
    await createProxyPolicy({
      name: policyForm.name,
      enabled: policyForm.enabled,
      subscription_ids: [policyForm.subscription_id],
      preferred_regions: splitCSV(policyForm.preferred_regions_text),
      max_concurrency: policyForm.max_concurrency,
      max_switches_per_task: policyForm.max_switches_per_task,
      config: policyConfigFromForm()
    })
    resetPolicyForm()
    appStore.showSuccess('策略已创建')
    await loadPoliciesAndTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingPolicy.value = false
  }
}

async function savePolicyNow() {
  if (!editingPolicyID.value) {
    await createPolicyNow()
    return
  }
  if (!policyForm.name || !policyForm.subscription_id) {
    appStore.showError('请填写策略名称并选择订阅')
    return
  }
  if (policyForm.selection_mode === 'fixed' && !policyForm.fixed_node_id) {
    appStore.showError('固定出口模式需要选择一个节点')
    return
  }
  savingPolicy.value = true
  try {
    const currentPolicy = policies.value.find((policy) => policy.id === editingPolicyID.value)
    await updateProxyPolicy(editingPolicyID.value, {
      name: policyForm.name,
      enabled: policyForm.enabled,
      subscription_ids: [policyForm.subscription_id],
      preferred_regions: splitCSV(policyForm.preferred_regions_text),
      max_concurrency: policyForm.max_concurrency,
      max_switches_per_task: policyForm.max_switches_per_task,
      config: policyConfigFromForm(currentPolicy)
    })
    appStore.showSuccess('策略已保存')
    resetPolicyForm()
    await loadPoliciesAndTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingPolicy.value = false
  }
}

function editPolicy(policy: ProxyPolicy) {
  editingPolicyID.value = policy.id
  policyForm.name = policy.name
  policyForm.subscription_id = policy.subscription_ids[0] || 0
  policyForm.preferred_regions_text = policy.preferred_regions.join(', ')
  policyForm.max_concurrency = policy.max_concurrency
  policyForm.max_switches_per_task = policy.max_switches_per_task
  policyForm.enabled = policy.enabled
  policyForm.selection_mode = policySelectionMode(policy)
  policyForm.fixed_node_id = fixedNodeID(policy)
}

function resetPolicyForm() {
  editingPolicyID.value = 0
  policyForm.name = ''
  policyForm.subscription_id = 0
  policyForm.preferred_regions_text = 'HK, JP, US'
  policyForm.max_concurrency = 1
  policyForm.max_switches_per_task = 2
  policyForm.enabled = true
  policyForm.selection_mode = 'auto'
  policyForm.fixed_node_id = 0
}

async function deletePolicyNow(policy: ProxyPolicy) {
  if (!window.confirm(`确认删除代理策略「${policy.name}」？`)) return
  try {
    await deleteProxyPolicy(policy.id)
    appStore.showSuccess('策略已删除')
    if (editingPolicyID.value === policy.id) resetPolicyForm()
    if (targetForm.policy_id === policy.id) targetForm.policy_id = 0
    await loadPoliciesAndTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function createTargetNow() {
  if (!targetForm.policy_id || !targetForm.target_host) {
    appStore.showError('请选择策略并填写目标 Host')
    return
  }
  savingTarget.value = true
  try {
    await createProxyTarget(targetForm.policy_id, {
      target_host: targetForm.target_host,
      purpose: targetForm.purpose,
      allowed_methods: splitCSV(targetForm.allowed_methods_text).map((method) => method.toUpperCase()),
      rate_limit_per_minute: targetForm.rate_limit_per_minute,
      enabled: targetForm.enabled,
      authorization_note: targetForm.authorization_note
    })
    resetTargetForm()
    appStore.showSuccess('白名单已添加')
    await loadTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingTarget.value = false
  }
}

async function saveTargetNow() {
  if (!editingTargetID.value) {
    await createTargetNow()
    return
  }
  if (!targetForm.policy_id || !targetForm.target_host) {
    appStore.showError('请选择策略并填写目标 Host')
    return
  }
  savingTarget.value = true
  try {
    await updateProxyTarget(targetForm.policy_id, editingTargetID.value, {
      target_host: targetForm.target_host,
      purpose: targetForm.purpose,
      allowed_methods: splitCSV(targetForm.allowed_methods_text).map((method) => method.toUpperCase()),
      rate_limit_per_minute: targetForm.rate_limit_per_minute,
      enabled: targetForm.enabled,
      authorization_note: targetForm.authorization_note
    })
    appStore.showSuccess('白名单已保存')
    resetTargetForm()
    await loadTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    savingTarget.value = false
  }
}

function editTarget(target: ProxyTargetPolicy) {
  editingTargetID.value = target.id
  targetForm.policy_id = target.policy_id
  targetForm.target_host = target.target_host
  targetForm.purpose = target.purpose
  targetForm.allowed_methods_text = target.allowed_methods.join(', ')
  targetForm.rate_limit_per_minute = target.rate_limit_per_minute
  targetForm.authorization_note = target.authorization_note || ''
  targetForm.enabled = target.enabled
}

function resetTargetForm() {
  editingTargetID.value = 0
  targetForm.policy_id = selectedPolicyID.value
  targetForm.target_host = ''
  targetForm.purpose = 'site_discovery'
  targetForm.allowed_methods_text = 'GET, POST'
  targetForm.rate_limit_per_minute = 60
  targetForm.authorization_note = ''
  targetForm.enabled = true
}

async function deleteTargetNow(target: ProxyTargetPolicy) {
  if (!window.confirm(`确认删除目标白名单「${target.target_host}」？`)) return
  try {
    await deleteProxyTarget(target.policy_id, target.id)
    appStore.showSuccess('白名单已删除')
    if (editingTargetID.value === target.id) resetTargetForm()
    await loadTargets()
    await loadStatus()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function restartSlotNow(id: number) {
  try {
    await restartProxyRuntimeSlot(id)
    appStore.showSuccess('槽位已重启')
    await loadRuntime()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function releaseAssignmentNow(id: number) {
  try {
    await releaseProxyAssignment(id)
    appStore.showSuccess('绑定已释放')
    await loadRuntime()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function switchAssignmentNow(assignment: ProxyAssignment) {
  const nodeID = Number(assignmentSwitchNodeIDs[assignment.id] || 0)
  if (!nodeID) {
    appStore.showError('请选择要切换的出口节点')
    return
  }
  try {
    await switchProxyAssignment(assignment.id, {
      node_id: nodeID,
      error_code: 'MANUAL_SWITCH',
      error_message: 'manual switch from proxy manager'
    })
    appStore.showSuccess('出口节点已切换')
    await loadRuntime()
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

function eligibleNodesForAssignment(assignment: ProxyAssignment): ProxyNode[] {
  const policy = policies.value.find((item) => item.id === assignment.policy_id)
  return nodes.value.filter((node) => {
    if (policy && policy.subscription_ids.length > 0 && !policy.subscription_ids.includes(node.subscription_id)) return false
    return node.health_status !== 'disabled' && node.health_status !== 'unhealthy' && node.health_status !== 'suspect'
  })
}

function splitCSV(value: string): string[] {
  return value.split(',').map((item) => item.trim()).filter(Boolean)
}

function policyConfigFromForm(policy?: ProxyPolicy): Record<string, unknown> {
  const config: Record<string, unknown> = { ...(policy?.config || {}) }
  config.selection_mode = policyForm.selection_mode
  if (policyForm.selection_mode === 'fixed') {
    config.fixed_node_id = policyForm.fixed_node_id
  } else {
    delete config.fixed_node_id
  }
  return config
}

function policySelectionMode(policy: ProxyPolicy): PolicySelectionMode {
  return String(policy.config?.selection_mode || 'auto') === 'fixed' ? 'fixed' : 'auto'
}

function fixedNodeID(policy: ProxyPolicy): number {
  const value = policy.config?.fixed_node_id
  if (typeof value === 'number') return value
  if (typeof value === 'string') return Number(value) || 0
  return 0
}

function nodeName(id: number): string {
  return nodes.value.find((node) => node.id === id)?.display_name || `节点 ${id}`
}

function policySelectionLabel(policy: ProxyPolicy): string {
  if (policySelectionMode(policy) === 'fixed') {
    const id = fixedNodeID(policy)
    return id ? `固定出口：${nodeName(id)}` : '固定出口：未选择'
  }
  return '自动选择健康节点'
}

function subscriptionTypeLabel(value: ProxySubscriptionType): string {
  if (value === 'shadowrocket') return 'Shadowrocket'
  if (value === 'v2ray_ss') return 'V2Ray + SS'
  return 'Clash'
}

function refreshLabel(value: ProxyRefreshStatus): string {
  const labels: Record<ProxyRefreshStatus, string> = {
    never: '未刷新',
    succeeded: '成功',
    failed: '失败',
    invalid: '无效'
  }
  return labels[value] || value
}

function refreshClass(value: ProxyRefreshStatus): string {
  if (value === 'succeeded') return 'badge-success'
  if (value === 'failed' || value === 'invalid') return 'badge-danger'
  return 'badge-gray'
}

function healthLabel(value: ProxyNodeHealthStatus): string {
  const labels: Record<ProxyNodeHealthStatus, string> = {
    unknown: '未知',
    healthy: '健康',
    degraded: '降级',
    suspect: '可疑',
    unhealthy: '不可用',
    disabled: '停用'
  }
  return labels[value] || value
}

function healthClass(value: ProxyNodeHealthStatus): string {
  if (value === 'healthy') return 'badge-success'
  if (value === 'degraded' || value === 'suspect') return 'badge-warning'
  if (value === 'unhealthy' || value === 'disabled') return 'badge-danger'
  return 'badge-gray'
}

function purposeLabel(value: ProxyTaskPurpose): string {
  const labels: Record<ProxyTaskPurpose, string> = {
    site_discovery: '授权采集',
    registration: '授权注册',
    supplier_probe: '供应商探测',
    manual_test: '人工测试'
  }
  return labels[value] || value
}

function slotLabel(value: ProxyRuntimeSlotStatus): string {
  const labels: Record<ProxyRuntimeSlotStatus, string> = {
    idle: '空闲',
    assigned: '已绑定',
    draining: '释放中',
    unhealthy: '异常',
    stopped: '停止'
  }
  return labels[value] || value
}

function slotClass(value: ProxyRuntimeSlotStatus): string {
  if (value === 'assigned') return 'badge-warning'
  if (value === 'idle') return 'badge-success'
  if (value === 'unhealthy') return 'badge-danger'
  return 'badge-gray'
}

function assignmentLabel(value: ProxyAssignmentStatus): string {
  const labels: Record<ProxyAssignmentStatus, string> = {
    active: '运行中',
    released: '已释放',
    failed: '失败'
  }
  return labels[value] || value
}

function assignmentClass(value: ProxyAssignmentStatus): string {
  if (value === 'active') return 'badge-warning'
  if (value === 'released') return 'badge-success'
  return 'badge-danger'
}

function auditClass(value: ProxyAuditLevel): string {
  if (value === 'error') return 'badge-danger'
  if (value === 'warning') return 'badge-warning'
  return 'badge-gray'
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function errorMessage(error: unknown): string {
  return (error as { message?: string })?.message || '操作失败'
}
</script>
