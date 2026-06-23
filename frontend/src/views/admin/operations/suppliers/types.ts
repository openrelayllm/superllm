import type { SupplierRuntimeStatus, SupplierHealthStatus, SupplierChannelProbeStatus, SupplierType } from '@/api/admin/adminPlus'

export type ChannelStatusWindow = 'pulse' | '7d' | '15d' | '30d'
export type ChannelScheduleStepStatus = 'done' | 'pending' | 'warning'
export type ChannelScheduleStepIcon = 'beaker' | 'key' | 'link' | 'play' | 'check' | 'clock' | 'exclamationTriangle'
export type ScheduleListStatusFilter = '' | 'scheduled' | 'paused' | 'risky' | 'untested'
export type ScheduleListLocalGroupFilter = '' | '__ungrouped__' | string
export type ChannelProtocol = 'openai' | 'claude' | 'gemini' | 'other'

export interface ChannelScheduleStep {
  key: string
  label: string
  description: string
  status: ChannelScheduleStepStatus
  icon: ChannelScheduleStepIcon
}

export interface ScheduleListRow {
  key: string
  name: string
  supplier_id: number
  supplier_name: string
  supplier_type: SupplierType
  binding_id: number
  supplier_group_id?: number
  local_account_id: number
  local_account_name: string
  local_account_status: string
  local_group_ids: number[]
  local_group_names: string[]
  runtime_status: SupplierRuntimeStatus
  health_status: SupplierHealthStatus
  schedulable: boolean
  group_name: string
  provider_family: string
  external_group_id: string
  supplier_group_rate: number
  effective_rate_multiplier: number
  probe_status: SupplierChannelProbeStatus
  first_token_ms: number
  duration_ms: number
  error_message: string
  captured_at: string
}

export interface LoadBestChannelChecksOptions {
  replace?: boolean
}

export interface QuickProvisionBestChannelOptions {
  actionKey?: string
  localGroupIDs?: number[]
  submittedMessage?: (jobID: number) => string
  completedMessage?: string
}
