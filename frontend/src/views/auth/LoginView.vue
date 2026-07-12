<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">
          管理员登录
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          使用 Sub2API 管理员账号继续
        </p>
      </div>

      <form class="space-y-5" @submit.prevent="handleLogin">
        <div>
          <label for="email" class="input-label">邮箱</label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
              <Icon name="mail" size="md" class="text-gray-400 dark:text-dark-500" />
            </div>
            <input
              id="email"
              v-model.trim="formData.email"
              type="email"
              required
              autofocus
              autocomplete="email"
              :disabled="isLoading || !publicSettingsLoaded"
              class="input pl-11"
              :class="{ 'input-error': errors.email }"
              placeholder="admin@example.com"
            />
          </div>
        </div>

        <div>
          <label for="password" class="input-label">密码</label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
              <Icon name="lock" size="md" class="text-gray-400 dark:text-dark-500" />
            </div>
            <input
              id="password"
              v-model="formData.password"
              :type="showPassword ? 'text' : 'password'"
              required
              autocomplete="current-password"
              :disabled="isLoading || !publicSettingsLoaded"
              class="input pl-11 pr-11"
              :class="{ 'input-error': errors.password }"
              placeholder="输入管理员密码"
            />
            <button
              type="button"
              class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-dark-300"
              :disabled="isLoading || !publicSettingsLoaded"
              @click="showPassword = !showPassword"
            >
              <Icon v-if="showPassword" name="eyeOff" size="md" />
              <Icon v-else name="eye" size="md" />
            </button>
          </div>
        </div>

        <div v-if="turnstileEnabled && turnstileSiteKey">
          <TurnstileWidget
            ref="turnstileRef"
            :site-key="turnstileSiteKey"
            @verify="onTurnstileVerify"
            @expire="onTurnstileExpire"
            @error="onTurnstileError"
          />
        </div>

        <button
          type="submit"
          class="btn btn-primary w-full"
          :disabled="isLoading || !publicSettingsLoaded || (turnstileEnabled && !turnstileToken)"
        >
          <svg
            v-if="isLoading"
            class="-ml-1 mr-2 h-4 w-4 animate-spin text-white"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          <Icon v-else name="login" size="md" class="mr-2" />
          {{ isLoading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </AuthLayout>

  <TotpLoginModal
    v-if="show2FAModal"
    ref="totpModalRef"
    :temp-token="totpTempToken"
    :user-email-masked="totpUserEmailMasked"
    @verify="handle2FAVerify"
    @cancel="handle2FACancel"
  />
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { AuthLayout } from '@/components/layout'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { getPublicSettings, isTotp2FARequired } from '@/api/auth'
import type { TotpLoginResponse } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const isLoading = ref(false)
const showPassword = ref(false)
const publicSettingsLoaded = ref(false)

const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const turnstileToken = ref('')
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)

const show2FAModal = ref(false)
const totpTempToken = ref('')
const totpUserEmailMasked = ref('')
const totpModalRef = ref<InstanceType<typeof TotpLoginModal> | null>(null)

const formData = reactive({
  email: '',
  password: ''
})

const errors = reactive({
  email: '',
  password: '',
  turnstile: ''
})

function validateForm(): boolean {
  errors.email = ''
  errors.password = ''
  errors.turnstile = ''

  if (!formData.email) {
    errors.email = '请输入邮箱'
    appStore.showError(errors.email)
    return false
  }

  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
    errors.email = '邮箱格式不正确'
    appStore.showError(errors.email)
    return false
  }

  if (!formData.password || formData.password.length < 6) {
    errors.password = '请输入至少 6 位密码'
    appStore.showError(errors.password)
    return false
  }

  if (turnstileEnabled.value && !turnstileToken.value) {
    errors.turnstile = '请完成人机验证'
    appStore.showError(errors.turnstile)
    return false
  }

  return true
}

function resolvePostLoginRedirect(): string {
  const redirect = router.currentRoute.value.query.redirect
  if (authStore.isAdmin && typeof redirect === 'string' && redirect.startsWith('/admin')) {
    return redirect
  }
  return '/admin/dashboard'
}

async function ensureAdminSession(): Promise<boolean> {
  if (authStore.isAdmin) return true
  await authStore.logout().catch(() => undefined)
  appStore.showError('当前账号不是管理员，不能进入 SuperLLM')
  return false
}

async function handleLogin(): Promise<void> {
  if (!validateForm()) return

  isLoading.value = true
  try {
    const response = await authStore.login({
      email: formData.email,
      password: formData.password,
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined
    })

    if (isTotp2FARequired(response)) {
      const totpResponse = response as TotpLoginResponse
      totpTempToken.value = totpResponse.temp_token || ''
      totpUserEmailMasked.value = totpResponse.user_email_masked || ''
      show2FAModal.value = true
      return
    }

    if (!(await ensureAdminSession())) return

    appStore.showSuccess('登录成功')
    await router.push(resolvePostLoginRedirect())
  } catch (error: unknown) {
    turnstileRef.value?.reset()
    turnstileToken.value = ''
    appStore.showError(extractApiErrorMessage(error, '登录失败'))
  } finally {
    isLoading.value = false
  }
}

async function handle2FAVerify(code: string): Promise<void> {
  totpModalRef.value?.setVerifying(true)

  try {
    await authStore.login2FA(totpTempToken.value, code)
    if (!(await ensureAdminSession())) {
      show2FAModal.value = false
      return
    }

    show2FAModal.value = false
    appStore.showSuccess('登录成功')
    await router.push(resolvePostLoginRedirect())
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { message?: string } } }
    const message = err.response?.data?.message || err.message || '二次验证失败'
    totpModalRef.value?.setError(message)
    totpModalRef.value?.setVerifying(false)
  }
}

function handle2FACancel(): void {
  show2FAModal.value = false
  totpTempToken.value = ''
  totpUserEmailMasked.value = ''
}

function onTurnstileVerify(token: string): void {
  turnstileToken.value = token
  errors.turnstile = ''
}

function onTurnstileExpire(): void {
  turnstileToken.value = ''
  errors.turnstile = '验证已过期'
}

function onTurnstileError(): void {
  turnstileToken.value = ''
  errors.turnstile = '验证失败'
}

onMounted(async () => {
  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
  } catch (error) {
    console.error('Failed to load public settings:', error)
  } finally {
    publicSettingsLoaded.value = true
  }
})
</script>
