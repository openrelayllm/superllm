<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">邮箱验证码</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            Gmail 授权、凭据检查、new-api / sub2api 验证码读取。
          </p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading || exchanging || reading" @click="loadCredentials">
          <Icon name="refresh" size="sm" />
          刷新
        </button>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">邮箱凭据</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ credentials.length }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">已检查</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ checkedCredentialCount }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">当前模板</p>
          <p class="mt-2 truncate text-2xl font-semibold text-gray-900 dark:text-white">{{ supplierTypeLabel(readForm.supplier_type) }}</p>
        </div>
        <div class="card p-4">
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">读取状态</p>
          <p class="mt-2 text-2xl font-semibold" :class="readStatusClass">{{ readStatusLabel }}</p>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.75fr)]">
        <div class="space-y-6">
          <section class="card overflow-hidden">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Gmail OAuth</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ oauthStatusLabel }}</p>
              </div>
              <span class="badge" :class="oauthConfigStatusClass">{{ oauthConfigStatusLabel }}</span>
            </div>
            <div class="space-y-5 p-5">
              <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
                <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div>
                    <h3 class="text-sm font-semibold text-gray-900 dark:text-white">Google OAuth 配置</h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">在 Google Cloud 创建 Web application OAuth Client 后粘贴 Client ID 和 Secret。</p>
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <a
                      class="btn btn-secondary btn-sm"
                      href="https://console.cloud.google.com/apis/library/gmail.googleapis.com"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <Icon name="externalLink" size="xs" />
                      启用 Gmail API
                    </a>
                    <a
                      class="btn btn-secondary btn-sm"
                      href="https://console.cloud.google.com/apis/credentials"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <Icon name="externalLink" size="xs" />
                      Google 凭据页
                    </a>
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      :disabled="oauthConfigLoading || oauthSaving"
                      @click="loadOAuthSettings"
                    >
                      <Icon name="refresh" size="xs" />
                      刷新配置
                    </button>
                  </div>
                </div>
                <div class="mt-4 grid gap-4 md:grid-cols-2">
                  <label class="block">
                    <span class="input-label">Google Client ID</span>
                    <input v-model.trim="oauthConfigForm.client_id" class="input font-mono text-xs" placeholder="*.apps.googleusercontent.com" autocomplete="off" />
                  </label>
                  <label class="block">
                    <span class="input-label">Google Client Secret</span>
                    <input v-model.trim="oauthConfigForm.client_secret" type="password" class="input font-mono text-xs" :placeholder="oauthSettings?.client_secret_configured ? '已配置，留空保留' : '请输入 Client Secret'" autocomplete="new-password" />
                  </label>
                  <label class="block md:col-span-2">
                    <span class="input-label">授权重定向 URI</span>
                    <div class="flex gap-2">
                      <input :value="redirectURI" readonly class="input min-w-0 font-mono text-xs" />
                      <button type="button" class="btn btn-secondary" @click="copyCode(redirectURI)">
                        <Icon name="copy" size="sm" />
                        复制
                      </button>
                    </div>
                  </label>
                </div>
                <div class="mt-4 flex flex-wrap items-center gap-2">
                  <button type="button" class="btn btn-primary" :disabled="oauthSaving" @click="saveOAuthSettings">
                    <Icon name="key" size="sm" />
                    {{ oauthSaving ? '保存中...' : '保存配置' }}
                  </button>
                  <span class="text-xs text-gray-500 dark:text-dark-400">{{ oauthConfigHint }}</span>
                </div>
                <div class="mt-3 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                  OAuth Client 类型选择 Web application，并把上面的授权重定向 URI 加到 Authorized redirect URIs。
                </div>
              </div>

              <div class="grid gap-4 md:grid-cols-2">
                <label class="block">
                  <span class="input-label">凭据名称</span>
                  <input v-model.trim="oauthForm.name" class="input" placeholder="注册收信 Gmail" />
                </label>
                <label class="block">
                  <span class="input-label">登录提示邮箱</span>
                  <input v-model.trim="oauthForm.login_hint" class="input" placeholder="name@gmail.com" />
                </label>
                <label class="block md:col-span-2">
                  <span class="input-label">Redirect URI</span>
                  <input :value="redirectURI" readonly class="input font-mono text-xs" />
                </label>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <button type="button" class="btn btn-primary" :disabled="authorizing || exchanging || !oauthConfigured" @click="beginOAuth">
                  <Icon name="login" size="sm" />
                  {{ authorizing ? '生成中...' : '授权 Gmail' }}
                </button>
                <span class="text-xs text-gray-500 dark:text-dark-400">{{ gmailScope }}</span>
              </div>
            </div>
          </section>

          <section class="card overflow-hidden">
            <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">读取验证码</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ selectedCredentialLabel }}</p>
              </div>
              <span class="badge" :class="selectedCredential ? 'badge-success' : 'badge-warning'">
                {{ selectedCredential ? '已选择凭据' : '未选择凭据' }}
              </span>
            </div>
            <div class="space-y-4 p-5">
              <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                <label class="block xl:col-span-3">
                  <span class="input-label">邮箱凭据</span>
                  <select v-model.number="selectedCredentialId" class="input">
                    <option :value="0">请选择 Gmail 凭据</option>
                    <option v-for="credential in credentials" :key="credential.id" :value="credential.id">
                      {{ credentialOptionLabel(credential) }}
                    </option>
                  </select>
                </label>
                <label class="block">
                  <span class="input-label">供应商模板</span>
                  <select v-model="readForm.supplier_type" class="input">
                    <option v-for="option in supplierTypeOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </label>
                <label class="block">
                  <span class="input-label">验证码用途</span>
                  <select v-model="readForm.expected_purpose" class="input">
                    <option v-for="option in purposeOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>
                </label>
                <label class="block">
                  <span class="input-label">站点名称</span>
                  <input v-model.trim="readForm.site_name" class="input" placeholder="可选，留空则不按站点名过滤" />
                </label>
                <label class="block">
                  <span class="input-label">发件人</span>
                  <input v-model.trim="readForm.from" class="input" placeholder="no-reply@example.com" />
                </label>
                <label class="block">
                  <span class="input-label">触发时间</span>
                  <input v-model="readForm.triggered_at" type="datetime-local" class="input" />
                </label>
                <label class="block">
                  <span class="input-label">超时秒数</span>
                  <input v-model.number="readForm.timeout_seconds" type="number" min="2" max="120" class="input" />
                </label>
                <label class="block xl:col-span-3">
                  <span class="input-label">搜索关键词</span>
                  <input v-model.trim="readForm.keywords" class="input" placeholder="验证码, verification code, security code" />
                </label>
              </div>

              <div class="grid gap-4 sm:grid-cols-2">
                <label class="block">
                  <span class="input-label">轮询间隔秒数</span>
                  <input v-model.number="readForm.poll_interval_seconds" type="number" min="2" max="30" class="input" />
                </label>
                <label class="block">
                  <span class="input-label">候选邮件数</span>
                  <input v-model.number="readForm.max_results" type="number" min="1" max="20" class="input" />
                </label>
              </div>

              <div class="flex flex-wrap items-center gap-2">
                <button type="button" class="btn btn-secondary" :disabled="reading" @click="setTriggeredNow">
                  <Icon name="clock" size="sm" />
                  当前时间
                </button>
                <button type="button" class="btn btn-primary" :disabled="reading || !selectedCredential" @click="readCode">
                  <Icon name="mail" size="sm" />
                  {{ reading ? '读取中...' : '读取验证码' }}
                </button>
              </div>
            </div>
          </section>
        </div>

        <aside class="space-y-6">
          <section class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">真实测试</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">通过 SMTP 发到已授权 Gmail，再用 Gmail API 读回。</p>
              </div>
              <span class="badge" :class="testResult ? 'badge-success' : testingCode ? 'badge-warning' : 'badge-gray'">
                {{ testResultStatusLabel }}
              </span>
            </div>
            <div class="space-y-4 p-5">
              <div class="flex flex-wrap items-center gap-2">
                <button type="button" class="btn btn-primary" :disabled="testingCode || !selectedCredential" @click="sendRealTestCode">
                  <Icon name="beaker" size="sm" />
                  {{ testingCode ? '发送并读取中...' : '发送测试邮件并读取' }}
                </button>
                <span class="text-xs text-gray-500 dark:text-dark-400">{{ selectedCredential ? credentialEmail(selectedCredential) : '先选择一个 Gmail 凭据' }}</span>
              </div>
              <div v-if="testResult" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-200">
                <div class="flex items-center justify-between gap-3">
                  <span>读回验证码</span>
                  <span class="font-mono text-lg font-semibold">{{ testResult.code || '-' }}</span>
                </div>
                <div class="mt-2 grid gap-1 text-xs">
                  <div>模板：{{ testResult.template_family || '-' }}</div>
                  <div>置信度：{{ confidenceLabel(testResult.confidence) }}</div>
                  <div>用途：{{ purposeLabel(testResult.purpose || '') }}</div>
                  <div>收信时间：{{ formatDateTime(testResult.received_at) }}</div>
                </div>
              </div>
            </div>
          </section>

          <section class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">凭据列表</h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">只显示脱敏邮箱和检查状态。</p>
              </div>
              <span class="badge badge-gray">{{ loading ? '加载中' : `${credentials.length} 个` }}</span>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-if="credentials.length === 0" class="px-5 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                暂无 Gmail 凭据
              </div>
              <div
                v-for="credential in credentials"
                :key="credential.id"
                class="px-5 py-4"
                :class="credential.id === selectedCredentialId ? 'bg-primary-50/60 dark:bg-primary-950/20' : ''"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ credential.name || 'Gmail' }}</div>
                    <div class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ credentialEmail(credential) }}</div>
                  </div>
                  <span class="badge shrink-0" :class="credentialStatusClass(credential)">{{ credentialStatusLabel(credential) }}</span>
                </div>
                <dl class="mt-3 grid gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <div class="flex items-center justify-between gap-3">
                    <dt>过期时间</dt>
                    <dd class="font-mono">{{ formatDateTime(credential.expires_at) }}</dd>
                  </div>
                  <div class="flex items-center justify-between gap-3">
                    <dt>最近检查</dt>
                    <dd class="font-mono">{{ formatDateTime(credential.last_checked_at) }}</dd>
                  </div>
                </dl>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button type="button" class="btn btn-secondary btn-sm" @click="selectedCredentialId = credential.id">
                    使用
                  </button>
                  <button type="button" class="btn btn-secondary btn-sm" :disabled="checkingId === credential.id" @click="checkCredential(credential.id)">
                    <Icon name="shield" size="xs" />
                    {{ checkingId === credential.id ? '检查中' : '检查' }}
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section class="card overflow-hidden">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">读取结果</h2>
              <span class="badge" :class="lastResult ? 'badge-success' : reading ? 'badge-warning' : 'badge-gray'">
                {{ readStatusLabel }}
              </span>
            </div>
            <div class="p-5">
              <div v-if="lastResult" class="space-y-4">
                <div class="rounded-xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/60 dark:bg-emerald-950/20">
                  <div class="flex items-center justify-between gap-3">
                    <div class="font-mono text-3xl font-semibold tracking-wider text-emerald-700 dark:text-emerald-300">
                      {{ lastResult.code }}
                    </div>
                    <button type="button" class="btn btn-secondary btn-sm" @click="copyCode(lastResult.code)">
                      <Icon name="copy" size="xs" />
                      复制
                    </button>
                  </div>
                </div>
                <dl class="grid gap-3 text-sm">
                  <div class="flex items-center justify-between gap-3">
                    <dt class="text-gray-500 dark:text-dark-400">邮件 ID</dt>
                    <dd class="max-w-[190px] truncate font-mono text-xs text-gray-900 dark:text-white">{{ lastResult.message_id }}</dd>
                  </div>
                  <div class="flex items-center justify-between gap-3">
                    <dt class="text-gray-500 dark:text-dark-400">收信时间</dt>
                    <dd class="text-gray-900 dark:text-white">{{ formatDateTime(lastResult.received_at) }}</dd>
                  </div>
                  <div class="flex items-center justify-between gap-3">
                    <dt class="text-gray-500 dark:text-dark-400">模板</dt>
                    <dd class="text-gray-900 dark:text-white">{{ lastResult.template_family || '-' }}</dd>
                  </div>
                  <div class="flex items-center justify-between gap-3">
                    <dt class="text-gray-500 dark:text-dark-400">置信度</dt>
                    <dd class="text-gray-900 dark:text-white">{{ confidenceLabel(lastResult.confidence) }}</dd>
                  </div>
                  <div class="flex items-center justify-between gap-3">
                    <dt class="text-gray-500 dark:text-dark-400">用途</dt>
                    <dd class="text-gray-900 dark:text-white">{{ purposeLabel(lastResult.purpose || '') }}</dd>
                  </div>
                </dl>
              </div>
              <div v-else class="rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
                {{ reading ? '正在轮询 Gmail' : '暂无读取结果' }}
              </div>
            </div>
          </section>
        </aside>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import {
  checkMailCredential,
  createMailOAuthAuthorizeURL,
  exchangeMailOAuthCode,
  getMailOAuthSettings,
  listMailCredentials,
  readMailVerificationCode,
  sendTestMailVerificationCode,
  updateMailOAuthSettings,
  type MailOAuthSettings,
  type MailVerificationCodeResult,
  type MailVerificationCredential,
  type MailVerificationPurpose,
  type ReadMailVerificationCodePayload
} from '@/api/admin/adminPlus'
import { extractApiErrorCode, extractApiErrorMetadata } from '@/utils/apiError'

type MailSupplierType = '' | 'new_api' | 'sub2api'

interface OAuthCache {
  redirectURI: string
  name: string
}

const GMAIL_SCOPE = 'https://www.googleapis.com/auth/gmail.readonly'
const GMAIL_API_LIBRARY_URL = 'https://console.cloud.google.com/apis/library/gmail.googleapis.com'
const GOOGLE_CLOUD_CREDENTIALS_URL = 'https://console.cloud.google.com/apis/credentials'
const OAUTH_STATE_PREFIX = 'admin-plus-mail-oauth:'
const OAUTH_QUERY_KEYS = new Set(['code', 'state', 'scope', 'authuser', 'prompt', 'error', 'error_description'])

const appStore = useAppStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const oauthConfigLoading = ref(false)
const oauthSaving = ref(false)
const authorizing = ref(false)
const exchanging = ref(false)
const reading = ref(false)
const testingCode = ref(false)
const checkingId = ref<number | null>(null)
const credentials = ref<MailVerificationCredential[]>([])
const selectedCredentialId = ref(0)
const lastResult = ref<MailVerificationCodeResult | null>(null)
const testResult = ref<MailVerificationCodeResult | null>(null)
const oauthSettings = ref<MailOAuthSettings | null>(null)

const oauthConfigForm = reactive({
  client_id: '',
  client_secret: ''
})

const oauthForm = reactive({
  name: 'Gmail 验证码',
  login_hint: ''
})

const readForm = reactive({
  supplier_type: '' as MailSupplierType,
  expected_purpose: 'email_verification' as MailVerificationPurpose,
  site_name: '',
  from: '',
  triggered_at: '',
  timeout_seconds: 90,
  poll_interval_seconds: 5,
  max_results: 10,
  keywords: '验证码, verification code, security code, login code'
})

const supplierTypeOptions: Array<{ value: MailSupplierType; label: string }> = [
  { value: 'sub2api', label: 'sub2api' },
  { value: 'new_api', label: 'new-api' },
  { value: '', label: '通用验证码' }
]

const purposeOptions: Array<{ value: MailVerificationPurpose; label: string }> = [
  { value: 'email_verification', label: '邮箱验证' },
  { value: 'notification_email_verification', label: '通知邮箱验证' },
  { value: '', label: '自动识别' }
]

const redirectURI = computed(() => currentRedirectURI())
const gmailScope = computed(() => GMAIL_SCOPE)
const selectedCredential = computed(() => credentials.value.find((item) => item.id === selectedCredentialId.value) || null)
const checkedCredentialCount = computed(() => credentials.value.filter((item) => item.last_checked_at && !item.last_error_code).length)
const selectedCredentialLabel = computed(() => selectedCredential.value ? credentialOptionLabel(selectedCredential.value) : '选择一个 Gmail 凭据后读取验证码')
const oauthConfigured = computed(() => Boolean(oauthSettings.value?.enabled && oauthSettings.value.client_id && oauthSettings.value.client_secret_configured))
const oauthConfigStatusLabel = computed(() => {
  if (oauthConfigLoading.value) return '读取配置'
  if (oauthConfigured.value) return '已配置'
  if (oauthSettings.value?.client_id || oauthSettings.value?.client_secret_configured) return '配置不完整'
  return '未配置'
})
const oauthConfigStatusClass = computed(() => {
  if (oauthConfigured.value) return 'badge-success'
  if (oauthConfigLoading.value) return 'badge-gray'
  return 'badge-warning'
})
const oauthConfigHint = computed(() => {
  if (oauthSettings.value?.client_secret_configured) return 'Secret 已配置；只修改 Client ID 时 Secret 可以留空。'
  return '保存后才能生成 Gmail 授权链接。'
})
const oauthStatusLabel = computed(() => {
  if (exchanging.value) return '正在保存 Gmail 授权'
  if (authorizing.value) return '正在生成授权链接'
  if (!oauthConfigured.value) return '先配置 Google OAuth Client ID 和 Client Secret'
  return '授权成功后会保存为后台邮箱凭据'
})
const readStatusLabel = computed(() => {
  if (reading.value) return '轮询中'
  if (lastResult.value) return '已读取'
  return '待操作'
})
const readStatusClass = computed(() => {
  if (reading.value) return 'text-amber-600 dark:text-amber-400'
  if (lastResult.value) return 'text-emerald-600 dark:text-emerald-400'
  return 'text-gray-900 dark:text-white'
})
const testResultStatusLabel = computed(() => {
  if (testingCode.value) return '测试中'
  if (!testResult.value) return '未测试'
  return '已读回'
})

onMounted(() => {
  void initializePage()
})

async function initializePage() {
  await handleOAuthCallback()
  await Promise.all([loadOAuthSettings(), loadCredentials()])
}

async function loadOAuthSettings() {
  oauthConfigLoading.value = true
  try {
    const settings = await getMailOAuthSettings('gmail')
    oauthSettings.value = settings
    oauthConfigForm.client_id = settings.client_id || ''
    oauthConfigForm.client_secret = ''
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    oauthConfigLoading.value = false
  }
}

async function saveOAuthSettings() {
  const clientID = oauthConfigForm.client_id.trim()
  const clientSecret = oauthConfigForm.client_secret.trim()
  if (!clientID) {
    appStore.showWarning('请输入 Google Client ID')
    return
  }
  if (!clientSecret && !oauthSettings.value?.client_secret_configured) {
    appStore.showWarning('请输入 Google Client Secret')
    return
  }
  oauthSaving.value = true
  try {
    const settings = await updateMailOAuthSettings({
      provider: 'gmail',
      client_id: clientID,
      client_secret: clientSecret || undefined,
      redirect_uri: currentRedirectURI(),
      frontend_redirect_uri: '/admin/mails'
    })
    oauthSettings.value = settings
    oauthConfigForm.client_id = settings.client_id || ''
    oauthConfigForm.client_secret = ''
    appStore.showSuccess('Google OAuth 配置已保存')
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    oauthSaving.value = false
  }
}

async function loadCredentials() {
  loading.value = true
  try {
    const items = await listMailCredentials({ provider: 'gmail' })
    credentials.value = items
    if (!selectedCredentialId.value && items.length > 0) {
      selectedCredentialId.value = items[0].id
    }
    if (selectedCredentialId.value && !items.some((item) => item.id === selectedCredentialId.value)) {
      selectedCredentialId.value = items[0]?.id || 0
    }
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function beginOAuth() {
  if (!oauthConfigured.value) {
    appStore.showWarning('请先保存 Google OAuth Client ID 和 Client Secret')
    return
  }
  authorizing.value = true
  try {
    const state = createOAuthState()
    const redirect = currentRedirectURI()
    sessionStorage.setItem(`${OAUTH_STATE_PREFIX}${state}`, JSON.stringify({
      redirectURI: redirect,
      name: oauthForm.name.trim()
    } satisfies OAuthCache))
    const result = await createMailOAuthAuthorizeURL({
      provider: 'gmail',
      redirect_uri: redirect,
      state,
      login_hint: normalizeEmpty(oauthForm.login_hint)
    })
    window.location.assign(result.authorize_url)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    authorizing.value = false
  }
}

async function handleOAuthCallback() {
  const oauthError = queryString(route.query.error)
  const code = queryString(route.query.code)
  if (oauthError) {
    appStore.showError(queryString(route.query.error_description) || oauthError)
    await cleanOAuthQuery()
    return
  }
  if (!code) return

  const state = queryString(route.query.state)
  const cache = readOAuthCache(state)
  if (!state || !cache) {
    appStore.showError('Gmail OAuth 状态已过期')
    await cleanOAuthQuery()
    return
  }

  exchanging.value = true
  try {
    const credential = await exchangeMailOAuthCode({
      provider: 'gmail',
      code,
      redirect_uri: cache.redirectURI,
      name: cache.name || oauthForm.name
    })
    selectedCredentialId.value = credential.id
    sessionStorage.removeItem(`${OAUTH_STATE_PREFIX}${state}`)
    appStore.showSuccess('Gmail 授权已保存')
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    exchanging.value = false
    await cleanOAuthQuery()
  }
}

async function checkCredential(id: number) {
  checkingId.value = id
  try {
    const updated = await checkMailCredential(id)
    upsertCredential(updated)
    appStore.showSuccess('邮箱凭据检查通过')
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    checkingId.value = null
  }
}

async function readCode() {
  if (!selectedCredential.value) {
    appStore.showWarning('请选择 Gmail 凭据')
    return
  }
  reading.value = true
  lastResult.value = null
  try {
    const payload: ReadMailVerificationCodePayload = {
      provider: 'gmail',
      credential_id: selectedCredential.value.id,
      from: normalizeEmpty(readForm.from),
      keywords: parseKeywords(readForm.keywords),
      supplier_type: readForm.supplier_type,
      expected_purpose: readForm.expected_purpose,
      site_name: normalizeEmpty(readForm.site_name),
      triggered_at: dateTimeLocalToISOString(readForm.triggered_at),
      timeout_seconds: readForm.timeout_seconds,
      poll_interval_seconds: readForm.poll_interval_seconds,
      max_results: readForm.max_results
    }
    lastResult.value = await readMailVerificationCode(payload)
    appStore.showSuccess('验证码已读取')
  } catch (error) {
    appStore.showError(errorMessage(error), 7000)
  } finally {
    reading.value = false
  }
}

async function sendRealTestCode() {
  if (!selectedCredential.value) {
    appStore.showWarning('请选择 Gmail 凭据')
    return
  }
  testingCode.value = true
  testResult.value = null
  try {
    testResult.value = await sendTestMailVerificationCode({
      credential_id: selectedCredential.value.id,
      supplier_type: readForm.supplier_type,
      expected_purpose: readForm.expected_purpose,
      site_name: normalizeEmpty(readForm.site_name) || 'Lime',
      timeout_seconds: readForm.timeout_seconds,
      poll_interval_seconds: readForm.poll_interval_seconds
    })
    appStore.showSuccess(`真实测试读回验证码：${testResult.value.code}`)
  } catch (error) {
    appStore.showError(errorMessage(error), 8000)
  } finally {
    testingCode.value = false
  }
}

async function copyCode(code: string) {
  try {
    await navigator.clipboard.writeText(code)
    appStore.showSuccess('验证码已复制')
  } catch {
    appStore.showError('复制失败')
  }
}

function upsertCredential(updated: MailVerificationCredential) {
  const index = credentials.value.findIndex((item) => item.id === updated.id)
  if (index === -1) {
    credentials.value.unshift(updated)
    return
  }
  credentials.value.splice(index, 1, updated)
}

function credentialOptionLabel(credential: MailVerificationCredential): string {
  return `${credential.name || 'Gmail'} · ${credentialEmail(credential)}`
}

function credentialEmail(credential: MailVerificationCredential): string {
  return credential.email_masked || credential.email || '-'
}

function credentialStatusLabel(credential: MailVerificationCredential): string {
  if (credential.last_error_code) return '异常'
  if (credential.last_checked_at) return '正常'
  return '未检查'
}

function credentialStatusClass(credential: MailVerificationCredential): string {
  if (credential.last_error_code) return 'badge-danger'
  if (credential.last_checked_at) return 'badge-success'
  return hasGmailReadonlyScope(credential) ? 'badge-primary' : 'badge-warning'
}

function hasGmailReadonlyScope(credential: MailVerificationCredential): boolean {
  return (credential.scopes || []).includes(GMAIL_SCOPE)
}

function supplierTypeLabel(value?: string): string {
  if (value === 'new_api') return 'new-api'
  if (value === 'sub2api') return 'sub2api'
  return '通用'
}

function purposeLabel(value: string): string {
  if (value === 'email_verification') return '邮箱验证'
  if (value === 'notification_email_verification') return '通知邮箱验证'
  return value || '自动识别'
}

function confidenceLabel(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${Math.round(value * 100)}%`
}

function setTriggeredNow() {
  readForm.triggered_at = dateToDateTimeLocal(new Date())
}

function currentRedirectURI(): string {
  return new URL('/admin/mails', window.location.origin).toString()
}

function createOAuthState(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function readOAuthCache(state: string): OAuthCache | null {
  if (!state) return null
  const raw = sessionStorage.getItem(`${OAUTH_STATE_PREFIX}${state}`)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<OAuthCache>
    if (!parsed.redirectURI) return null
    return {
      redirectURI: parsed.redirectURI,
      name: parsed.name || ''
    }
  } catch {
    return null
  }
}

async function cleanOAuthQuery() {
  const query: Record<string, string | string[]> = {}
  for (const [key, value] of Object.entries(route.query)) {
    if (OAUTH_QUERY_KEYS.has(key)) continue
    if (Array.isArray(value)) {
      const items = value.filter((item): item is string => typeof item === 'string')
      if (items.length > 0) query[key] = items
      continue
    }
    if (typeof value === 'string') query[key] = value
  }
  await router.replace({ path: route.path, query })
}

function queryString(value: unknown): string {
  if (Array.isArray(value)) return typeof value[0] === 'string' ? value[0] : ''
  return typeof value === 'string' ? value : ''
}

function parseKeywords(value: string): string[] {
  return value
    .split(/[\n,，]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function normalizeEmpty(value: string): string | undefined {
  return value.trim() || undefined
}

function dateToDateTimeLocal(value: Date): string {
  const offsetMs = value.getTimezoneOffset() * 60 * 1000
  return new Date(value.getTime() - offsetMs).toISOString().slice(0, 16)
}

function dateTimeLocalToISOString(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

function errorMessage(error: unknown): string {
  const code = extractApiErrorCode(error)
  if (code === 'MAIL_OAUTH_CONFIG_INVALID') {
    return `请先配置 Google OAuth Client ID 和 Client Secret，并在 Google Cloud Console 授权重定向 URI 中加入：${currentRedirectURI()}`
  }
  if (code === 'MAIL_OAUTH_CLIENT_SECRET_REQUIRED') return '请输入 Google OAuth Client Secret'
  if (code === 'MAIL_OAUTH_CLIENT_ID_REQUIRED') return '请输入 Google OAuth Client ID'
  if (code === 'MAIL_GMAIL_SCOPE_REQUIRED') return `当前 Gmail 凭据缺少读取权限，请删除这个凭据后重新授权，并确认授权 scope 包含 ${GMAIL_SCOPE}`
  if (code === 'MAIL_GMAIL_UNAUTHENTICATED') return 'Gmail 授权已失效，请重新授权这个 Gmail 凭据'
  if (code === 'MAIL_GMAIL_RATE_LIMITED') return 'Gmail API 当前限流，请稍后重试，或降低轮询频率'
  if (code === 'MAIL_GMAIL_PERMISSION_DENIED') return gmailPermissionDeniedMessage(error)
  if (code === 'MAIL_VERIFICATION_CODE_NOT_FOUND') return '超时未找到验证码邮件'
  if (errorStatus(error) === 404 && !code) {
    return '当前后端还没有加载邮箱验证码测试接口，请重启或重新部署到包含 /api/v1/admin-plus/mails/verification-code/send-test 的后端版本后再试'
  }
  if (error instanceof Error) return error.message
  if (error && typeof error === 'object') {
    const data = error as { message?: unknown; reason?: unknown }
    const message = typeof data.message === 'string' ? data.message.trim() : ''
    const reason = typeof data.reason === 'string' ? data.reason.trim() : ''
    if (message && reason) return `${message}（${reason}）`
    return message || reason || '操作失败'
  }
  return '操作失败'
}

function gmailPermissionDeniedMessage(error: unknown): string {
  const metadata = extractApiErrorMetadata(error)
  const reason = metadataValue(metadata, 'gmail_reason')
  const message = metadataValue(metadata, 'gmail_message')
  const activationUrl = metadataValue(metadata, 'gmail_activation_url') || metadataValue(metadata, 'gmail_help_url') || GMAIL_API_LIBRARY_URL
  const normalized = `${reason} ${message}`.toLowerCase()

  let action = `先打开 ${activationUrl} 启用 Gmail API，等待 1-3 分钟后重试`
  if (reason === 'ACCESS_TOKEN_SCOPE_INSUFFICIENT' || normalized.includes('insufficient authentication scopes')) {
    action = `当前授权 scope 不足，请删除当前 Gmail 凭据后重新授权，并确认 scope 包含 ${GMAIL_SCOPE}`
  } else if (reason === 'SERVICE_DISABLED' || normalized.includes('disabled') || normalized.includes('has not been used')) {
    action = `当前 Google Cloud 项目未启用 Gmail API，请打开 ${activationUrl} 启用 Gmail API，等待 1-3 分钟后重试`
  } else if (normalized.includes('admin') || normalized.includes('policy') || normalized.includes('blocked')) {
    action = '可能被 Google Workspace 管理员策略限制，请让管理员允许该 OAuth 应用和 Gmail readonly scope'
  }

  const original = message ? `Google 原始信息：${message}` : ''
  return [
    `Gmail API 权限被 Google 拒绝${reason ? `（${reason}）` : ''}`,
    action,
    `如果 OAuth 同意屏幕仍是 Testing，请在 Google Cloud OAuth consent screen 把当前 Gmail 加入 Test users；也可以到 ${GOOGLE_CLOUD_CREDENTIALS_URL} 检查 OAuth Client 配置`,
    original,
  ].filter(Boolean).join('。')
}

function metadataValue(metadata: Record<string, unknown> | undefined, key: string): string {
  const value = metadata?.[key]
  return typeof value === 'string' ? value.trim() : ''
}

function errorStatus(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') return undefined
  const status = (error as { status?: unknown }).status
  return typeof status === 'number' ? status : undefined
}
</script>
