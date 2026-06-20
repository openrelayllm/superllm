(() => {
  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== 'admin-plus:run-task') return false
    Promise.resolve(runTask(message.task, message.credential))
      .then((result) => sendResponse(result))
      .catch((error) => sendResponse({
        ok: false,
        error_code: error.reason || 'CONTENT_SCRIPT_ERROR',
        error_message: error.message || String(error)
      }))
    return true
  })

  async function runTask(task, credential) {
    const login = ensureLogin(credential)
    if (login) return login

    switch (task.type) {
      case 'fetch_rates':
      case 'fetch_balance':
      case 'fetch_promotions':
      case 'export_bills':
      case 'fetch_health':
        return window.AdminPlusSub2APIParser.collectByTask(task.type, pageSnapshot())
      default:
        return fail('UNSUPPORTED_TASK_TYPE', `unsupported task type: ${task.type}`)
    }
  }

  function ensureLogin(credential) {
    const passwordInput = document.querySelector('input[type="password"]')
    const loginLike = passwordInput || /login|signin|auth/i.test(location.pathname)
    if (!loginLike) return null

    if (credential.token) {
      window.localStorage.setItem('auth_token', credential.token)
      window.localStorage.setItem('token_expires_at', String(Date.now() + 24 * 60 * 60 * 1000))
      location.reload()
      return { ok: false, status: 'login_applied' }
    }

    if (!credential.username || !credential.password || !passwordInput) {
      return fail('LOGIN_CREDENTIAL_REQUIRED', 'supplier login page requires username and password')
    }

    const userInput = document.querySelector('input[type="email"], input[name*="email" i], input[name*="user" i], input[type="text"]')
    if (!userInput) {
      return fail('LOGIN_FORM_UNSUPPORTED', 'supplier login form username input was not found')
    }
    setInputValue(userInput, credential.username)
    setInputValue(passwordInput, credential.password)
    const submit = document.querySelector('button[type="submit"], input[type="submit"], button')
    if (!submit) {
      return fail('LOGIN_FORM_UNSUPPORTED', 'supplier login form submit button was not found')
    }
    submit.click()
    return { ok: false, status: 'login_submitted' }
  }

  function setInputValue(input, value) {
    const setter = Object.getOwnPropertyDescriptor(input.constructor.prototype, 'value')?.set
    setter?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
  }

  function ok(result) {
    return { ok: true, result }
  }

  function fail(errorCode, errorMessage) {
    return { ok: false, error_code: errorCode, error_message: errorMessage }
  }

  function pageSnapshot() {
    return {
      url: location.href,
      host: location.host,
      text: (document.body?.innerText || '').replace(/\r/g, '\n'),
      rows: Array.from(document.querySelectorAll('tr')).map((tr, index) => ({
        index,
        cells: Array.from(tr.querySelectorAll('th,td')).map((cell) => cell.textContent || '')
      }))
    }
  }
})()
