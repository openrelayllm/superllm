(function attachAdminPlusParser(global) {
  function collectRates(snapshot) {
    const entries = []
    for (const row of snapshot.rows || []) {
      const cells = normalizeCells(row.cells)
      if (cells.length < 2) continue
      const model = findModel(cells)
      const price = findMoneyOrNumber(cells)
      if (!model || price === null) continue
      entries.push({
        model,
        billing_mode: inferBillingMode(cells),
        price_item: inferPriceItem(cells),
        unit: inferUnit(cells),
        currency: inferCurrency(cells),
        price_micros: Math.round(price * 1000000),
        raw_payload: { cells, url: snapshot.url }
      })
    }
    if (entries.length === 0) {
      return fail('RATE_TABLE_NOT_FOUND', 'no supported rate table rows were found')
    }
    return ok({
      source: 'chrome',
      captured_at: nowISO(),
      threshold_percent: 1,
      entries
    })
  }

  function collectBalance(snapshot) {
    const balance = findLabeledAmount(snapshot.text || '', ['balance', '余额', '剩余', '可用'])
    if (!balance) {
      return fail('BALANCE_NOT_FOUND', 'no supported balance value was found')
    }
    return ok({
      source: 'chrome',
      captured_at: nowISO(),
      runtime_status: 'monitor_only',
      balance_cents: Math.round(balance.amount * 100),
      currency: balance.currency,
      raw_payload: { url: snapshot.url, evidence: balance.evidence }
    })
  }

  function collectPromotions(snapshot) {
    const text = snapshot.text || ''
    const keywords = ['优惠', '折扣', '赠送', 'bonus', 'discount', 'promotion', 'recharge']
    const promotions = text
      .split(/\n+/)
      .map((line) => line.trim())
      .filter((line) => line && keywords.some((keyword) => line.toLowerCase().includes(keyword.toLowerCase())))
      .slice(0, 20)
    if (promotions.length === 0) {
      return fail('PROMOTION_PAGE_UNSUPPORTED', 'no supported promotion entries were found')
    }
    return ok({
      source: 'chrome',
      promotions: promotions.map((line) => ({
        type: inferPromotionType(line),
        title: line.slice(0, 120),
        description: line,
        currency: inferCurrency([line]) === 'USD' ? 'USD' : 'CNY',
        runtime_status: 'monitor_only',
        balance_cents: 0,
        raw_payload: { url: snapshot.url, line }
      }))
    })
  }

  function collectBills(snapshot) {
    const lines = []
    for (const row of snapshot.rows || []) {
      const cells = normalizeCells(row.cells)
      const cost = findMoneyOrNumber(cells)
      const model = findModel(cells)
      const requestID = cells.find((value) => /req|chatcmpl|cmpl|[a-f0-9-]{16,}/i.test(value)) || ''
      if (cost === null || !model) continue
      lines.push({
        external_bill_id: requestID || `${snapshot.host || 'supplier'}-${row.index}`,
        external_request_id: requestID,
        model,
        currency: inferCurrency(cells),
        cost_cents: Math.round(cost * 100),
        input_tokens: findTokenValue(cells, ['input', 'prompt', '输入']),
        output_tokens: findTokenValue(cells, ['output', 'completion', '输出']),
        started_at: findDate(cells) || nowISO(),
        raw_payload: { cells, url: snapshot.url }
      })
    }
    if (lines.length === 0) {
      return fail('BILL_TABLE_NOT_FOUND', 'no supported billing rows were found')
    }
    return ok({ source: 'chrome', lines })
  }

  function collectHealth(snapshot) {
    const concurrency = findLabeledPair(snapshot.text || '', ['并发', 'concurrency'])
    if (!concurrency) {
      return fail('HEALTH_METRICS_NOT_FOUND', 'no supported health metrics were found')
    }
    return ok({
      source: 'chrome',
      captured_at: nowISO(),
      model: 'unknown',
      first_token_latency_ms: 0,
      total_latency_ms: 0,
      status_code: 200,
      observed_concurrency: concurrency.current,
      available_concurrency: Math.max(0, concurrency.limit - concurrency.current),
      concurrency_limit: concurrency.limit,
      concurrency_saturation_percent: concurrency.limit > 0 ? (concurrency.current / concurrency.limit) * 100 : 0,
      raw_payload: { url: snapshot.url, evidence: concurrency.evidence }
    })
  }

  function collectByTask(taskType, snapshot) {
    switch (taskType) {
      case 'fetch_rates':
        return collectRates(snapshot)
      case 'fetch_balance':
        return collectBalance(snapshot)
      case 'fetch_promotions':
        return collectPromotions(snapshot)
      case 'export_bills':
        return collectBills(snapshot)
      case 'fetch_health':
        return collectHealth(snapshot)
      default:
        return fail('UNSUPPORTED_TASK_TYPE', `unsupported task type: ${taskType}`)
    }
  }

  function normalizeCells(cells) {
    return (cells || []).map((value) => String(value || '').trim()).filter(Boolean)
  }

  function findModel(cells) {
    return cells.find((value) => /\b(gpt|claude|gemini|o[0-9]|text-|embedding|rerank|dall-e|whisper)[\w.-]*\b/i.test(value)) || ''
  }

  function findMoneyOrNumber(values) {
    const normalized = (values || []).map((value) => String(value || '').trim()).filter(Boolean)
    const explicit = normalized.filter((value) => /[$￥¥]|USD|CNY|RMB|price|cost|amount|费用|金额|价格/i.test(value))
    for (const value of explicit) {
      const parsed = parseFirstNumber(value)
      if (parsed !== null) return parsed
    }
    for (const value of normalized) {
      if (isNonMoneyValue(value)) continue
      const parsed = parseFirstNumber(value)
      if (parsed !== null) return parsed
    }
    return null
  }

  function parseFirstNumber(value) {
    const match = String(value || '').replace(/,/g, '').match(/(\d+(?:\.\d+)?)/)
    if (!match) return null
    const parsed = Number(match[1])
    return Number.isFinite(parsed) ? parsed : null
  }

  function isNonMoneyValue(value) {
    if (findModel([value])) return true
    if (/req|chatcmpl|cmpl|request[_-]?id|请求.?id/i.test(value)) return true
    if (/token|input|output|prompt|completion|输入|输出/i.test(value)) return true
    const date = new Date(value)
    if (!Number.isNaN(date.getTime())) return true
    return false
  }

  function findLabeledAmount(text, labels) {
    const lines = String(text || '').split(/\n+/)
    for (const line of lines) {
      const normalized = line.trim()
      if (!labels.some((label) => normalized.toLowerCase().includes(label.toLowerCase()))) continue
      const amount = findMoneyOrNumber([normalized])
      if (amount === null) continue
      return {
        amount,
        currency: inferCurrency([normalized]),
        evidence: normalized.slice(0, 300)
      }
    }
    return null
  }

  function findLabeledPair(text, labels) {
    const lines = String(text || '').split(/\n+/)
    for (const line of lines) {
      const normalized = line.trim()
      if (!labels.some((label) => normalized.toLowerCase().includes(label.toLowerCase()))) continue
      const match = normalized.match(/(\d+)\s*\/\s*(\d+)/)
      if (!match) continue
      return {
        current: Number(match[1]),
        limit: Number(match[2]),
        evidence: normalized.slice(0, 300)
      }
    }
    return null
  }

  function inferBillingMode(cells) {
    return cells.some((value) => /request|请求/i.test(value)) ? 'request' : 'token'
  }

  function inferPriceItem(cells) {
    const text = cells.join(' ').toLowerCase()
    if (/output|completion|输出/.test(text)) return 'output'
    if (/input|prompt|输入/.test(text)) return 'input'
    return 'mixed'
  }

  function inferUnit(cells) {
    const text = cells.join(' ').toLowerCase()
    if (/1m|million|百万/.test(text)) return '1m_tokens'
    if (/1k|thousand|千/.test(text)) return '1k_tokens'
    if (/request|请求/.test(text)) return 'request'
    return 'token'
  }

  function inferCurrency(cells) {
    const text = cells.join(' ')
    if (/USD|\$/.test(text)) return 'USD'
    if (/CNY|RMB|￥|¥|元/.test(text)) return 'CNY'
    return 'USD'
  }

  function inferPromotionType(line) {
    const lower = String(line || '').toLowerCase()
    if (/bonus|赠送|返/.test(lower)) return 'recharge_bonus'
    if (/discount|折扣|优惠/.test(lower)) return 'rate_discount'
    return 'other'
  }

  function findTokenValue(cells, labels) {
    for (const cell of cells) {
      const lower = cell.toLowerCase()
      if (!labels.some((label) => lower.includes(label.toLowerCase()))) continue
      const match = cell.replace(/,/g, '').match(/(\d+)/)
      if (match) return Number(match[1])
    }
    return 0
  }

  function findDate(cells) {
    for (const cell of cells) {
      const date = new Date(cell)
      if (!Number.isNaN(date.getTime())) return date.toISOString()
    }
    return ''
  }

  function nowISO() {
    return new Date().toISOString()
  }

  function ok(result) {
    return { ok: true, result }
  }

  function fail(errorCode, errorMessage) {
    return { ok: false, error_code: errorCode, error_message: errorMessage }
  }

  const parser = {
    collectByTask,
    collectRates,
    collectBalance,
    collectPromotions,
    collectBills,
    collectHealth,
    helpers: {
      findModel,
      findMoneyOrNumber,
      findLabeledAmount,
      findLabeledPair,
      inferBillingMode,
      inferPriceItem,
      inferUnit,
      inferCurrency,
      inferPromotionType,
      findTokenValue,
      findDate
    }
  }

  global.AdminPlusSub2APIParser = parser
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = parser
  }
})(typeof globalThis !== 'undefined' ? globalThis : window)
