const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')

const probeSource = fs.readFileSync(path.join(__dirname, 'src/content/site-probe.js'), 'utf8')

function createInput(overrides = {}) {
  return {
    type: 'text',
    name: '',
    id: '',
    autocomplete: '',
    placeholder: '',
    value: '',
    disabled: false,
    readOnly: false,
    getAttribute() { return '' },
    getBoundingClientRect() { return { width: 120, height: 32 } },
    ...overrides
  }
}

async function collectCandidate(inputs, options = {}) {
  let listener = null
  let fetchCalls = 0
  const storage = {
    length: 0,
    key() { return null },
    getItem() { return null }
  }
  const document = {
    title: 'Supplier Console',
    body: { innerText: '' },
    querySelectorAll(selector) {
      return selector === 'input' ? inputs : []
    }
  }
  const window = {
    localStorage: storage,
    sessionStorage: storage,
    getComputedStyle() {
      return { display: 'block', visibility: 'visible' }
    }
  }
  window.window = window
  const context = {
    window,
    document,
    location: {
      href: 'https://supplier.example.com/login',
      origin: 'https://supplier.example.com',
      host: 'supplier.example.com',
      pathname: '/login'
    },
    chrome: {
      runtime: {
        onMessage: {
          addListener(handler) {
            listener = handler
          }
        }
      }
    },
    fetch: async () => {
      fetchCalls += 1
      return { ok: false }
    },
    AbortController,
    URL,
    setTimeout,
    clearTimeout
  }

  vm.runInNewContext(probeSource, context)
  assert.equal(typeof listener, 'function')
  const result = await new Promise((resolve) => {
    listener({
      type: 'admin-plus:collect-candidate:v2',
      include_sensitive: true,
      skip_api_probe: options.skipAPIProbe === true
    }, null, resolve)
  })
  return { result, fetchCalls }
}

async function main() {
  const emailOnlyProbe = await collectCandidate([
    createInput({ type: 'email', name: 'email', autocomplete: 'username', value: 'ops@example.com' })
  ], { skipAPIProbe: true })
  const emailOnly = emailOnlyProbe.result.credential
  assert.equal(emailOnly.username, 'ops@example.com')
  assert.equal(emailOnly.password, '')
  assert.equal(emailOnly.password_present, false)
  assert.equal(emailOnlyProbe.fetchCalls, 0)

  const revealedPasswordProbe = await collectCandidate([
    createInput({ type: 'email', name: 'email', autocomplete: 'username', value: 'ops@example.com' }),
    createInput({ type: 'text', name: 'login_password', autocomplete: 'current-password', value: 'secret-value' })
  ])
  const revealedPassword = revealedPasswordProbe.result.credential
  assert.equal(revealedPassword.username, 'ops@example.com')
  assert.equal(revealedPassword.password, 'secret-value')
  assert.equal(revealedPassword.password_present, true)

  const standardPasswordProbe = await collectCandidate([
    createInput({ type: 'email', name: 'email', value: 'ops@example.com' }),
    createInput({ type: 'password', name: 'password', value: 'standard-secret' })
  ])
  const standardPassword = standardPasswordProbe.result.credential
  assert.equal(standardPassword.password, 'standard-secret')
  assert.equal(standardPassword.password_present, true)

  console.log('extension site probe tests passed')
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
