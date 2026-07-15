const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { webcrypto } = require('node:crypto')

const backgroundSource = fs.readFileSync(path.join(__dirname, 'src/background-app.js'), 'utf8')
const config = {
  baseURL: 'https://admin.example.com',
  token: 'admin-token',
  deviceID: 'chrome-test-device',
  connectedAt: '2026-07-15T00:00:00Z'
}
const activeTab = {
  id: 7,
  title: 'Supplier Console',
  url: 'https://supplier.example.com/dashboard'
}
let supplierRequests = 0
let lastContentMessage = null

const chrome = {
  storage: {
    local: {
      async get(key) {
        if (key === 'adminPlusOperatorConfig') return { adminPlusOperatorConfig: config }
        return {}
      },
      async set() {}
    }
  },
  runtime: {
    getURL(value) {
      return `chrome-extension://test/${value}`
    },
    onMessage: {
      addListener() {}
    }
  },
  tabs: {
    async query() {
      return [activeTab]
    },
    async sendMessage(_tabID, message) {
      lastContentMessage = message
      return {
        status: 'identified',
        provider_type: 'new_api',
        confidence: 0.9,
        evidence: ['test'],
        page: {
          title: activeTab.title,
          url: activeTab.url,
          origin: 'https://supplier.example.com',
          host: 'supplier.example.com'
        },
        credential: {},
        defaults: {
          name: activeTab.title,
          dashboard_url: activeTab.url,
          api_base_url: 'https://supplier.example.com'
        }
      }
    }
  }
}

async function fetchMock(url) {
  const value = String(url)
  if (value.includes('/api/v1/admin-plus/suppliers')) {
    supplierRequests += 1
    return {
      ok: true,
      status: 200,
      async text() {
        return JSON.stringify({ code: 0, data: { items: [] } })
      }
    }
  }
  return { ok: false, status: 404 }
}

async function main() {
  const context = {
    chrome,
    fetch: fetchMock,
    crypto: globalThis.crypto || webcrypto,
    URL,
    AbortController,
    setTimeout,
    clearTimeout,
    console,
    atob: globalThis.atob,
    btoa: globalThis.btoa
  }
  vm.runInNewContext(backgroundSource, context)
  assert.equal(typeof context.adminPlusHandleMessage, 'function')

  const state = await context.adminPlusHandleMessage({ type: 'state:get' })
  assert.equal(state.connection.status, 'connected')
  await context.adminPlusHandleMessage({ type: 'site:identify' })
  assert.equal(supplierRequests, 1, 'state and site identification should reuse one supplier request')

  await context.adminPlusHandleMessage({
    type: 'site:collect-candidate',
    includeSensitive: true,
    skipAPIProbe: true,
    knownProviderType: 'new_api'
  })
  assert.equal(lastContentMessage.skip_api_probe, true)

  console.log('extension background app tests passed')
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
