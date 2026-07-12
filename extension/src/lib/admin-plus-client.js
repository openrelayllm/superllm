export class AdminPlusClient {
  constructor(config) {
    this.baseURL = trimTrailingSlash(config.baseURL || '')
    this.token = config.token || ''
  }

  ready() {
    return this.baseURL !== '' && this.token !== ''
  }

  async listSuppliers() {
    const page = await this.request('/api/v1/admin-plus/suppliers?page=1&page_size=500')
    return Array.isArray(page?.items) ? page.items : []
  }

  async createCaptureSessionTask(deviceID, supplierID, payload = {}) {
    return this.request('/api/v1/admin-plus/extension/session/capture-task', {
      method: 'POST',
      body: {
        device_id: deviceID,
        ...(supplierID ? { supplier_id: supplierID } : {}),
        lease_ttl_seconds: 300,
        url: payload.source_url || '',
        host: payload.source_host || '',
        origin: payload.source_origin || '',
        dashboard_url: payload.dashboard_url || '',
        api_base_url: payload.api_base_url || '',
        type: payload.supplier_type || payload.provider_type || '',
        supplier_type: payload.supplier_type || payload.provider_type || '',
        provider_type: payload.provider_type || payload.supplier_type || '',
        third_party_recharge_url: payload.third_party_recharge_url || '',
        local_recharge_url: payload.local_recharge_url || '',
        auto_create_supplier: payload.auto_create_supplier,
        page_context: payload.page_context || {},
        payload
      }
    })
  }

  async reportSupplierCandidate(payload) {
    return this.request('/api/v1/admin-plus/extension/suppliers/report-candidate', {
      method: 'POST',
      body: payload
    })
  }

  async claimTask(deviceID, types = []) {
    return this.request('/api/v1/admin-plus/extension/tasks/claim', {
      method: 'POST',
      body: {
        device_id: deviceID,
        types,
        lease_ttl_seconds: 300
      }
    })
  }

  async heartbeat(task, leaseTTLSeconds = 300) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/heartbeat`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        lease_ttl_seconds: leaseTTLSeconds
      }
    })
  }

  async browserCredential(task) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/browser-credential`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token
      }
    })
  }

  async registrationCredential(task) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/registration-credential`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token
      }
    })
  }

  async complete(task, result) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/complete`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        result
      }
    })
  }

  async fail(task, errorCode, errorMessage) {
    return this.request(`/api/v1/admin-plus/extension/tasks/${task.id}/fail`, {
      method: 'POST',
      body: {
        device_id: task.device_id,
        lease_token: task.lease_token,
        error_code: errorCode,
        error_message: errorMessage
      }
    })
  }

  async request(path, options = {}) {
    if (!this.ready()) {
      throw new Error('Admin Plus URL and token are required')
    }
    const response = await fetch(`${this.baseURL}${path}`, {
      method: options.method || 'GET',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.token}`
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    })
    const text = await response.text()
    const json = parseJSON(text)
    if (!response.ok || json.code !== 0) {
      const reason = json.reason || `HTTP_${response.status}`
      const message = json.message || text || 'Admin Plus request failed'
      const error = new Error(message)
      error.reason = reason
      throw error
    }
    return json.data
  }
}

export function trimTrailingSlash(value) {
  return String(value || '').replace(/\/+$/, '')
}

function parseJSON(text) {
  try {
    return JSON.parse(text)
  } catch {
    throw new Error('Admin Plus did not return JSON')
  }
}
