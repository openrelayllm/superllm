import { apiClient } from '../client'

export interface SystemSettings {
  site_name: string
  site_logo: string
  site_subtitle: string
  api_base_url: string
  contact_info: string
  doc_url: string
  proxyai_purity_turnstile_enabled: boolean
  proxyai_purity_turnstile_site_key: string
  proxyai_purity_turnstile_secret_key?: string
  proxyai_purity_turnstile_secret_key_configured?: boolean
}

export type UpdateSettingsRequest = Partial<SystemSettings>

export interface AdminApiKeyStatus {
  exists: boolean
  masked_key: string
}

export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>('/admin/settings')
  return data
}

export async function updateSettings(settings: UpdateSettingsRequest): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>('/admin/settings', settings)
  return data
}

export async function getAdminApiKey(): Promise<AdminApiKeyStatus> {
  const { data } = await apiClient.get<AdminApiKeyStatus>('/admin/settings/admin-api-key')
  return data
}

export async function regenerateAdminApiKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>('/admin/settings/admin-api-key/regenerate')
  return data
}

export async function deleteAdminApiKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/admin/settings/admin-api-key')
  return data
}

export default {
  getSettings,
  updateSettings,
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey
}
