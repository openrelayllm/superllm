/**
 * Minimal group API used by SuperLLM ops filters.
 */

import { apiClient } from '../client'
import type { AdminGroup, GroupPlatform } from '@/types'

export async function getAll(platform?: GroupPlatform): Promise<AdminGroup[]> {
  const { data } = await apiClient.get<AdminGroup[]>('/admin/groups/all', {
    params: platform ? { platform } : undefined
  })
  return data
}

export const groupsAPI = {
  getAll
}

export default groupsAPI
