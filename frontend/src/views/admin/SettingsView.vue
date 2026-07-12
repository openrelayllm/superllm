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
              SuperLLM 当前只保留后台运营系统所需的站点信息。
            </p>
          </div>

          <form class="space-y-5 p-6" @submit.prevent="saveSettings">
            <div class="grid gap-5 md:grid-cols-2">
              <label class="block">
                <span class="input-label">站点名称</span>
                <input v-model.trim="form.site_name" class="input" placeholder="SuperLLM" />
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
  getSettings,
  updateSettings,
  type SystemSettings
} from '@/api/admin/settings'

const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)

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

onMounted(async () => {
  loading.value = true
  try {
    await loadSettings()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || '加载设置失败')
  } finally {
    loading.value = false
  }
})
</script>
