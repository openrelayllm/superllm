<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else>
        <section class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">基础设置</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              Admin Plus 当前只保留后台运营系统所需的站点信息。
            </p>
          </div>

          <form class="space-y-5 p-6" @submit.prevent="saveSettings">
            <div class="grid gap-5 md:grid-cols-2">
              <label class="block">
                <span class="input-label">站点名称</span>
                <input v-model.trim="form.site_name" class="input" placeholder="Sub2API Admin Plus" />
              </label>

              <label class="block">
                <span class="input-label">副标题</span>
                <input v-model.trim="form.site_subtitle" class="input" placeholder="Operations Automation Console" />
              </label>

              <label class="block md:col-span-2">
                <span class="input-label">Logo URL</span>
                <input v-model.trim="form.site_logo" class="input" placeholder="/logo.svg" />
              </label>

              <label class="block md:col-span-2">
                <span class="input-label">API Base URL</span>
                <input v-model.trim="form.api_base_url" class="input" placeholder="https://api.example.com" />
              </label>

              <label class="block">
                <span class="input-label">联系方式</span>
                <input v-model.trim="form.contact_info" class="input" placeholder="support@example.com" />
              </label>

              <label class="block">
                <span class="input-label">文档地址</span>
                <input v-model.trim="form.doc_url" class="input" placeholder="https://docs.example.com" />
              </label>
            </div>

            <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/60">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h3 class="text-sm font-semibold text-gray-900 dark:text-white">纯度检测验证码</h3>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                    仅控制 proxyai.best 公开检测页；Developer API 继续使用平台 API Key 鉴权。
                  </p>
                </div>
                <label class="inline-flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-dark-200">
                  <input v-model="form.proxyai_purity_turnstile_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                  启用
                </label>
              </div>

              <div class="mt-4 grid gap-5 md:grid-cols-2">
                <label class="block">
                  <span class="input-label">Turnstile Site Key</span>
                  <input v-model.trim="form.proxyai_purity_turnstile_site_key" class="input" placeholder="0x4AAAA..." />
                </label>

                <label class="block">
                  <span class="input-label">Turnstile Secret Key</span>
                  <input
                    v-model.trim="form.proxyai_purity_turnstile_secret_key"
                    class="input"
                    type="password"
                    :placeholder="form.proxyai_purity_turnstile_secret_key_configured ? '已配置，留空则不修改' : '请输入 Secret Key'"
                    autocomplete="new-password"
                  />
                </label>
              </div>
            </div>

            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary" :disabled="saving">
                {{ saving ? '保存中...' : '保存设置' }}
              </button>
            </div>
          </form>
        </section>

        <section class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Admin API Key</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              供外部自动化运营系统调用 Admin Plus 管理接口。密钥只在重新生成时展示一次。
            </p>
          </div>

          <div class="space-y-5 p-6">
            <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
              该密钥拥有管理员能力，请仅保存在受控的服务端环境中，不要写入浏览器插件或前端代码。
            </div>

            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  当前状态
                </div>
                <div class="mt-1 font-mono text-sm text-gray-500 dark:text-dark-400">
                  {{ apiKeyStatus.exists ? apiKeyStatus.masked_key : '未配置' }}
                </div>
              </div>

              <div class="flex gap-2">
                <button type="button" class="btn btn-secondary" :disabled="apiKeyOperating" @click="loadAdminApiKey">
                  刷新
                </button>
                <button type="button" class="btn btn-primary" :disabled="apiKeyOperating" @click="regenerateKey">
                  重新生成
                </button>
                <button
                  v-if="apiKeyStatus.exists"
                  type="button"
                  class="btn btn-danger"
                  :disabled="apiKeyOperating"
                  @click="removeKey"
                >
                  删除
                </button>
              </div>
            </div>

            <div v-if="generatedKey" class="rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20">
              <div class="text-sm font-medium text-green-800 dark:text-green-200">
                新密钥已生成，请立即保存
              </div>
              <div class="mt-2 flex flex-col gap-2 sm:flex-row">
                <code class="min-w-0 flex-1 break-all rounded bg-white px-3 py-2 text-sm text-green-900 dark:bg-dark-900 dark:text-green-100">
                  {{ generatedKey }}
                </code>
                <button type="button" class="btn btn-secondary" @click="copyGeneratedKey">复制</button>
              </div>
            </div>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores/app'
import {
  deleteAdminApiKey,
  getAdminApiKey,
  getSettings,
  regenerateAdminApiKey,
  updateSettings,
  type AdminApiKeyStatus,
  type SystemSettings
} from '@/api/admin/settings'

const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)
const apiKeyOperating = ref(false)
const generatedKey = ref('')

const form = reactive<SystemSettings>({
  site_name: '',
  site_logo: '',
  site_subtitle: '',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  proxyai_purity_turnstile_enabled: false,
  proxyai_purity_turnstile_site_key: '',
  proxyai_purity_turnstile_secret_key: '',
  proxyai_purity_turnstile_secret_key_configured: false
})

const apiKeyStatus = reactive<AdminApiKeyStatus>({
  exists: false,
  masked_key: ''
})

function applySettings(settings: SystemSettings) {
  form.site_name = settings.site_name || ''
  form.site_logo = settings.site_logo || ''
  form.site_subtitle = settings.site_subtitle || ''
  form.api_base_url = settings.api_base_url || ''
  form.contact_info = settings.contact_info || ''
  form.doc_url = settings.doc_url || ''
  form.proxyai_purity_turnstile_enabled = Boolean(settings.proxyai_purity_turnstile_enabled)
  form.proxyai_purity_turnstile_site_key = settings.proxyai_purity_turnstile_site_key || ''
  form.proxyai_purity_turnstile_secret_key = ''
  form.proxyai_purity_turnstile_secret_key_configured = Boolean(settings.proxyai_purity_turnstile_secret_key_configured)
}

async function loadSettings() {
  const settings = await getSettings()
  applySettings(settings)
}

async function loadAdminApiKey() {
  apiKeyOperating.value = true
  try {
    const status = await getAdminApiKey()
    apiKeyStatus.exists = status.exists
    apiKeyStatus.masked_key = status.masked_key || ''
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载 Admin API Key 失败')
  } finally {
    apiKeyOperating.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const settings = await updateSettings({ ...form })
    applySettings(settings)
    appStore.clearPublicSettingsCache()
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess('设置已保存')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '保存设置失败')
  } finally {
    saving.value = false
  }
}

async function regenerateKey() {
  apiKeyOperating.value = true
  generatedKey.value = ''
  try {
    const result = await regenerateAdminApiKey()
    generatedKey.value = result.key
    await loadAdminApiKey()
    appStore.showSuccess('Admin API Key 已重新生成')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '生成 Admin API Key 失败')
  } finally {
    apiKeyOperating.value = false
  }
}

async function removeKey() {
  apiKeyOperating.value = true
  generatedKey.value = ''
  try {
    await deleteAdminApiKey()
    apiKeyStatus.exists = false
    apiKeyStatus.masked_key = ''
    appStore.showSuccess('Admin API Key 已删除')
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '删除 Admin API Key 失败')
  } finally {
    apiKeyOperating.value = false
  }
}

async function copyGeneratedKey() {
  if (!generatedKey.value) return
  await navigator.clipboard.writeText(generatedKey.value)
  appStore.showSuccess('已复制')
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([loadSettings(), loadAdminApiKey()])
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载设置失败')
  } finally {
    loading.value = false
  }
})
</script>
