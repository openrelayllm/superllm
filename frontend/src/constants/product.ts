export const DEFAULT_SITE_NAME = 'SuperLLM'
export const DEFAULT_SITE_SUBTITLE = 'Operations Automation Console'

const LEGACY_SITE_NAMES = new Set(['sub2api', 'sub2api admin', 'sub2api admin plus'])

export function normalizeProductSiteName(value?: string): string {
  const trimmed = value?.trim() || ''
  const normalized = trimmed.toLowerCase().replace(/[-_]+/g, ' ').replace(/\s+/g, ' ')
  return LEGACY_SITE_NAMES.has(normalized) || !normalized ? DEFAULT_SITE_NAME : trimmed
}
