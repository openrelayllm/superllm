import { describe, expect, it } from 'vitest'
import type { SiteDiscoveryItem, SiteDiscoveryRegistrationTask } from '@/api/admin/adminPlus'
import {
  canImportDiscoveredSupplier,
  canQueueSiteRegistration,
  canRerunRegistration,
  isSiteDiscoveryProcessed,
  siteDiscoveryProcessedLabel,
  siteDiscoveryImportHint
} from '../siteDiscoveryPresentation'

describe('siteDiscoveryPresentation', () => {
  it('requires successful registration before supplier import', () => {
    expect(canImportDiscoveredSupplier(discoveryItem({ registration_status: '', supplier_id: 0 }))).toBe(false)
    expect(canImportDiscoveredSupplier(discoveryItem({ registration_status: 'queued', supplier_id: 0 }))).toBe(false)
    expect(canImportDiscoveredSupplier(discoveryItem({ registration_status: 'succeeded', supplier_id: 42 }))).toBe(true)
  })

  it('queues registration only for supported unregistered providers', () => {
    expect(canQueueSiteRegistration(discoveryItem({ registration_status: '', provider_type: 'new_api' }), true)).toBe(true)
    expect(canQueueSiteRegistration(discoveryItem({ registration_status: 'queued', provider_type: 'new_api' }), true)).toBe(false)
    expect(canQueueSiteRegistration(discoveryItem({ registration_status: '', provider_type: '' }), true)).toBe(false)
    expect(canQueueSiteRegistration(discoveryItem({ registration_status: '', provider_type: 'sub2api' }), false)).toBe(false)
  })

  it('allows rerun for active and failed registration tasks when backend marks retryable', () => {
    expect(canRerunRegistration(registrationTask({ status: 'queued', can_retry: true }))).toBe(true)
    expect(canRerunRegistration(registrationTask({ status: 'running', can_retry: true }))).toBe(true)
    expect(canRerunRegistration(registrationTask({ status: 'failed', can_retry: true }))).toBe(true)
    expect(canRerunRegistration(registrationTask({ status: 'waiting_manual_verification', can_retry: true }))).toBe(true)
    expect(canRerunRegistration(registrationTask({ status: 'succeeded', can_retry: true }))).toBe(false)
    expect(canRerunRegistration(registrationTask({ status: 'failed', can_retry: false }))).toBe(false)
  })

  it('keeps processing status separate from registration task status', () => {
    expect(siteDiscoveryProcessedLabel(discoveryItem({ registration_status: 'queued' }))).toBe('注册中')
    expect(isSiteDiscoveryProcessed(discoveryItem({ registration_status: 'queued' }))).toBe(false)
    expect(siteDiscoveryProcessedLabel(discoveryItem({ registration_status: 'succeeded', supplier_id: 42, import_status: 'imported' }))).toBe('已入库')
    expect(isSiteDiscoveryProcessed(discoveryItem({ registration_status: 'succeeded' }))).toBe(true)
  })

  it('explains why unregistered discoveries are not importable', () => {
    expect(siteDiscoveryImportHint(discoveryItem({ registration_status: '' }))).toBe('需先注册')
    expect(siteDiscoveryImportHint(discoveryItem({ registration_status: 'queued' }))).toBe('等待浏览器兜底完成后自动入库')
    expect(siteDiscoveryImportHint(discoveryItem({ registration_status: 'running' }))).toBe('注册完成后自动入库')
    expect(siteDiscoveryImportHint(discoveryItem({ registration_status: 'succeeded', supplier_id: 42 }))).toBe('已入库 #42')
  })
})

function discoveryItem(overrides: Partial<SiteDiscoveryItem> = {}): SiteDiscoveryItem {
  return {
    id: 1,
    run_id: 1,
    source_url: 'https://index.example.com',
    source_site_id: 'site-1',
    source_section: 'third-party',
    name: 'Example Relay',
    register_url: 'https://relay.example.com/register',
    dashboard_url: 'https://relay.example.com',
    api_base_url: 'https://relay.example.com',
    host: 'relay.example.com',
    provider_type: 'new_api',
    classification_status: 'supported',
    classification_confidence: 0.98,
    import_status: 'new',
    process_status: 'unprocessed',
    created_at: '2026-06-26T00:00:00Z',
    updated_at: '2026-06-26T00:00:00Z',
    ...overrides
  }
}

function registrationTask(overrides: Partial<SiteDiscoveryRegistrationTask> = {}): SiteDiscoveryRegistrationTask {
  return {
    id: 10,
    discovery_id: 1,
    task_id: 10,
    status: 'queued',
    can_retry: false,
    created_at: '2026-06-26T00:00:00Z',
    updated_at: '2026-06-26T00:00:00Z',
    discovery: discoveryItem(),
    ...overrides
  }
}
