<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">续费提醒</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            维护服务器档案并提前提醒月付续费，避免服务器到期导致服务不可用。
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPage">
            <Icon name="refresh" size="sm" />
            刷新
          </button>
          <button type="button" class="btn btn-primary" :disabled="saving" @click="saveRenewal">
            <Icon name="check" size="sm" />
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
        </div>
      </section>

      <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">提醒开关</p>
          <p class="mt-2 text-2xl font-semibold" :class="form.enabled ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-700 dark:text-dark-200'">
            {{ form.enabled ? '已开启' : '已关闭' }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ form.server_name || '未命名服务器' }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">到期剩余</p>
          <p class="mt-2 text-2xl font-semibold" :class="stateTextClass">{{ daysLabel }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ statusLabel(status?.state) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">到期时间</p>
          <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ expiryDisplay }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ form.provider || '未知服务商' }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">下次提醒</p>
          <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ status?.next_reminder || '-' }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ reminderDaysText || '7,3,1' }} 天</p>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">服务器档案</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">记录 IDC 面板、主机、系统和 SSH 信息，续费时能快速定位服务器。</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="copySSHCommand">
              <Icon name="copy" size="sm" />
              复制 SSH
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="openPanelAction('打开面板')">
              <Icon name="externalLink" size="sm" />
              打开面板
            </button>
          </div>
        </div>

        <div class="grid gap-5 p-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(380px,0.9fr)]">
          <div class="rounded-md border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="flex flex-col gap-3 border-l-4 border-primary-500 pl-4 sm:flex-row sm:items-start sm:justify-between">
              <div class="flex items-start gap-3">
                <span class="flex h-10 w-10 items-center justify-center rounded-md bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
                  <Icon name="server" size="md" />
                </span>
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ form.server_name || '未命名服务器' }}</h3>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <span class="badge" :class="form.enabled ? 'badge-success' : 'badge-gray'">{{ form.enabled ? '提醒已开启' : '提醒已关闭' }}</span>
                    <span v-if="form.host_id" class="badge badge-gray">Host ID {{ form.host_id }}</span>
                    <span v-if="status?.state" class="badge" :class="statusBadgeClass(status.state)">{{ statusLabel(status.state) }}</span>
                  </div>
                </div>
              </div>
              <div class="text-left sm:text-right">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">到期</p>
                <p class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ expiryDisplay }}</p>
              </div>
            </div>

            <div class="mt-5 grid gap-3 md:grid-cols-2">
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">IP</p>
                <p class="mt-2 break-all font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ form.ip_address || '-' }}</p>
              </div>
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">系统</p>
                <p class="mt-2 break-all text-sm font-semibold text-gray-900 dark:text-white">{{ form.operating_system || '-' }}</p>
              </div>
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">用户名</p>
                <p class="mt-2 break-all font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ form.ssh_username || '-' }}</p>
              </div>
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">SSH 端口</p>
                <p class="mt-2 font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ form.ssh_port || 22 }}</p>
              </div>
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">SSH 密码</p>
                <p class="mt-2 text-sm font-semibold" :class="form.ssh_password_configured || form.ssh_password ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-500 dark:text-dark-400'">
                  {{ form.ssh_password_configured || form.ssh_password ? '已配置' : '未配置' }}
                </p>
              </div>
              <div class="rounded-md border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900/50">
                <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">服务商</p>
                <p class="mt-2 break-all text-sm font-semibold text-gray-900 dark:text-white">{{ form.provider || '-' }}</p>
              </div>
            </div>

            <div class="mt-5 flex flex-wrap gap-2">
              <button
                v-for="action in serverActions"
                :key="action.label"
                type="button"
                class="btn btn-sm"
                :class="serverActionClass(action.tone)"
                @click="openPanelAction(action.label)"
              >
                <Icon :name="action.icon" size="xs" />
                {{ action.label }}
              </button>
            </div>
          </div>

          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-1">
            <label class="block">
              <span class="input-label">服务器名称</span>
              <input v-model.trim="form.server_name" class="input" placeholder="例如：美国1区精品网 4H4G" />
            </label>
            <label class="block">
              <span class="input-label">服务商</span>
              <input v-model.trim="form.provider" class="input" placeholder="例如：七云 / Client" />
            </label>
            <label class="block">
              <span class="input-label">Host ID</span>
              <input v-model.trim="form.host_id" class="input" placeholder="服务器面板里的 Host ID" />
            </label>
            <label class="block">
              <span class="input-label">IP 地址</span>
              <input v-model.trim="form.ip_address" class="input font-mono" placeholder="服务器公网 IP" autocomplete="off" />
            </label>
            <label class="block md:col-span-2 xl:col-span-1">
              <span class="input-label">系统镜像</span>
              <input v-model.trim="form.operating_system" class="input" placeholder="例如：Ubuntu-22.04-x64" />
            </label>
            <label class="block md:col-span-2 xl:col-span-1">
              <span class="input-label">IDC 面板地址</span>
              <input v-model.trim="form.panel_url" class="input" placeholder="https://idc.example.com" autocomplete="off" />
            </label>
            <label class="block">
              <span class="input-label">SSH 用户名</span>
              <input v-model.trim="form.ssh_username" class="input font-mono" placeholder="root" autocomplete="username" />
            </label>
            <label class="block">
              <span class="input-label">SSH 端口</span>
              <input v-model.number="form.ssh_port" type="number" min="1" max="65535" class="input font-mono" placeholder="22" />
            </label>
            <label class="block md:col-span-2 xl:col-span-1">
              <span class="input-label">SSH 密码</span>
              <div class="relative">
                <input
                  v-model.trim="form.ssh_password"
                  class="input pr-10 font-mono"
                  :type="passwordVisible ? 'text' : 'password'"
                  autocomplete="new-password"
                  :placeholder="form.ssh_password_configured ? '已配置，留空表示不变' : '可填写后保存，页面不会回显明文'"
                />
                <button
                  type="button"
                  class="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-gray-400 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-100"
                  :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                  @click="passwordVisible = !passwordVisible"
                >
                  <Icon :name="passwordVisible ? 'eyeOff' : 'eye'" size="sm" />
                </button>
              </div>
            </label>
            <label class="block">
              <span class="input-label">到期日期</span>
              <input v-model="form.expires_at" type="date" class="input" />
            </label>
            <label class="block">
              <span class="input-label">到期时间</span>
              <input v-model="form.expires_at_time" type="time" step="1" class="input" />
            </label>
          </div>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">提醒配置</h2>
          </div>
          <div class="space-y-4 p-5">
            <div class="flex items-center justify-between gap-4 rounded-md border border-gray-200 px-4 py-3 dark:border-dark-700">
              <div>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">启用提醒</span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">开启后按提前提醒天数检查到期状态。</span>
              </div>
              <div class="flex items-center gap-3">
                <span class="text-sm font-medium" :class="form.enabled ? 'text-primary-700 dark:text-primary-300' : 'text-gray-500 dark:text-dark-400'">
                  {{ form.enabled ? '已开启' : '已关闭' }}
                </span>
                <Toggle v-model="form.enabled" class="scale-110" />
              </div>
            </div>

            <label class="block">
              <span class="input-label">提前提醒天数</span>
              <input v-model.trim="reminderDaysText" class="input" placeholder="7,3,1" />
            </label>
          </div>
        </div>

        <aside class="space-y-6">
          <div class="card p-5">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">提醒状态</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-gray-500 dark:text-dark-400">当前状态</dt>
                <dd><span class="badge" :class="statusBadgeClass(status?.state)">{{ statusLabel(status?.state) }}</span></dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-gray-500 dark:text-dark-400">剩余天数</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ daysLabel }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-gray-500 dark:text-dark-400">下次提醒</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.next_reminder || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-gray-500 dark:text-dark-400">上次通知</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ status?.last_notified_at || '-' }}</dd>
              </div>
            </dl>
          </div>

          <div class="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
            续费提醒只负责服务器档案和到期提醒，不参与数据库备份、恢复或对象存储配置。
          </div>
        </aside>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { adminPlusAPI, type ServerRenewalStatus } from '@/api/admin/adminPlus'

const appStore = useAppStore()

type ServerActionTone = 'primary' | 'neutral' | 'danger'
type ServerAction = {
  label: string
  icon: 'play' | 'refresh' | 'bolt' | 'key' | 'download'
  tone: ServerActionTone
}

const loading = ref(false)
const saving = ref(false)
const passwordVisible = ref(false)
const status = ref<ServerRenewalStatus | null>(null)
const reminderDaysText = ref('7,3,1')

const serverActions: ServerAction[] = [
  { label: '开机', icon: 'play', tone: 'primary' },
  { label: '关机', icon: 'bolt', tone: 'neutral' },
  { label: '重启', icon: 'refresh', tone: 'neutral' },
  { label: '硬关机', icon: 'bolt', tone: 'danger' },
  { label: '改密码', icon: 'key', tone: 'neutral' },
  { label: '重装', icon: 'download', tone: 'danger' }
]

const form = reactive<ServerRenewalStatus>({
  enabled: true,
  server_name: 'sub2api-admin-plus',
  provider: '',
  host_id: '',
  ip_address: '',
  operating_system: '',
  ssh_username: '',
  ssh_password: '',
  ssh_password_configured: false,
  ssh_port: 22,
  panel_url: '',
  expires_at: '',
  expires_at_time: '',
  reminder_days: [7, 3, 1],
  days_remaining: 0,
  state: 'unconfigured'
})

const daysLabel = computed(() => {
  const value = status.value
  if (!value?.expires_at) return '未配置'
  if (value.days_remaining < 0) return `逾期 ${Math.abs(value.days_remaining)} 天`
  if (value.days_remaining === 0) return '今天'
  return `${value.days_remaining} 天`
})

const expiryDisplay = computed(() => {
  if (!form.expires_at) return '未配置'
  return [form.expires_at, form.expires_at_time].filter(Boolean).join(' ')
})

const sshCommand = computed(() => {
  const host = form.ip_address?.trim()
  if (!host) return ''
  const user = form.ssh_username?.trim() || 'root'
  const port = Number(form.ssh_port) || 22
  return port === 22 ? `ssh ${user}@${host}` : `ssh -p ${port} ${user}@${host}`
})

const stateTextClass = computed(() => statusTextClass(status.value?.state))

onMounted(() => {
  void loadPage()
})

async function loadPage() {
  loading.value = true
  try {
    const nextStatus = await adminPlusAPI.getServerRenewal()
    applyStatus(nextStatus)
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || '加载续费提醒失败')
  } finally {
    loading.value = false
  }
}

async function saveRenewal() {
  saving.value = true
  try {
    const nextStatus = await adminPlusAPI.updateServerRenewal({
      enabled: form.enabled,
      server_name: form.server_name.trim() || 'sub2api-admin-plus',
      provider: form.provider?.trim() || '',
      host_id: form.host_id?.trim() || '',
      ip_address: form.ip_address?.trim() || '',
      operating_system: form.operating_system?.trim() || '',
      ssh_username: form.ssh_username?.trim() || '',
      ssh_password: form.ssh_password?.trim() || '',
      ssh_port: Number(form.ssh_port) || 22,
      panel_url: form.panel_url?.trim() || '',
      expires_at: form.expires_at.trim(),
      expires_at_time: form.expires_at_time?.trim() || '',
      reminder_days: parseReminderDays(reminderDaysText.value)
    })
    applyStatus(nextStatus)
    appStore.showSuccess('续费提醒已保存')
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || '保存续费提醒失败')
  } finally {
    saving.value = false
  }
}

function applyStatus(nextStatus: ServerRenewalStatus) {
  status.value = nextStatus
  Object.assign(form, {
    enabled: Boolean(nextStatus.enabled),
    server_name: nextStatus.server_name || 'sub2api-admin-plus',
    provider: nextStatus.provider || '',
    host_id: nextStatus.host_id || '',
    ip_address: nextStatus.ip_address || '',
    operating_system: nextStatus.operating_system || '',
    ssh_username: nextStatus.ssh_username || '',
    ssh_password: '',
    ssh_password_configured: Boolean(nextStatus.ssh_password_configured),
    ssh_port: nextStatus.ssh_port || 22,
    panel_url: nextStatus.panel_url || '',
    expires_at: nextStatus.expires_at || '',
    expires_at_time: nextStatus.expires_at_time || '',
    reminder_days: nextStatus.reminder_days?.length ? nextStatus.reminder_days : [7, 3, 1],
    days_remaining: nextStatus.days_remaining || 0,
    state: nextStatus.state || 'unconfigured',
    next_reminder: nextStatus.next_reminder || '',
    last_notified_at: nextStatus.last_notified_at || ''
  })
  reminderDaysText.value = form.reminder_days.join(',')
}

function parseReminderDays(value: string): number[] {
  const days = value
    .split(/[,\s]+/)
    .map((item) => Number.parseInt(item, 10))
    .filter((item) => Number.isFinite(item) && item >= 0 && item <= 365)
  const unique = Array.from(new Set(days)).sort((a, b) => b - a)
  return unique.length ? unique : [7, 3, 1]
}

async function copySSHCommand() {
  if (!sshCommand.value) {
    appStore.showError('请先填写服务器 IP')
    return
  }
  try {
    await navigator.clipboard.writeText(sshCommand.value)
    appStore.showSuccess('SSH 命令已复制')
  } catch {
    appStore.showError('复制 SSH 命令失败')
  }
}

function openPanelAction(label: string) {
  const url = normalizePanelURL(form.panel_url)
  if (!url) {
    appStore.showError('请先填写 IDC 面板地址')
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
  if (label !== '打开面板') {
    appStore.showSuccess(`已打开面板，请在 IDC 面板执行${label}`)
  }
}

function normalizePanelURL(value?: string): string {
  const trimmed = value?.trim() || ''
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

function serverActionClass(tone: ServerActionTone): string {
  if (tone === 'primary') return 'btn-primary'
  if (tone === 'danger') return 'btn-danger'
  return 'btn-secondary'
}

function statusLabel(value?: string): string {
  if (value === 'active') return '正常'
  if (value === 'reminder_due') return '待提醒'
  if (value === 'due_today') return '今日到期'
  if (value === 'expired') return '已到期'
  return '未配置'
}

function statusBadgeClass(value?: string): string {
  if (value === 'active') return 'badge-success'
  if (value === 'reminder_due' || value === 'due_today') return 'badge-warning'
  if (value === 'expired') return 'badge-danger'
  return 'badge-gray'
}

function statusTextClass(value?: string): string {
  if (value === 'active') return 'text-emerald-700 dark:text-emerald-400'
  if (value === 'reminder_due' || value === 'due_today') return 'text-amber-600 dark:text-amber-400'
  if (value === 'expired') return 'text-rose-600 dark:text-rose-400'
  return 'text-gray-700 dark:text-dark-200'
}
</script>
