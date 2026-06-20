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

  function collectGroups(snapshot) {
    const candidates = collectGroupCandidates(snapshot)
    const groups = []
    const seen = new Set()
    for (const candidate of candidates) {
      const parsed = parseGroupCandidate(candidate)
      if (!parsed.name) continue
      const key = `${parsed.name}|${parsed.rate_multiplier || ''}`
      if (seen.has(key)) continue
      seen.add(key)
      groups.push({
        name: parsed.name,
        description: parsed.description,
        platform: parsed.platform,
        rate_multiplier: parsed.rate_multiplier,
        is_private: parsed.is_private,
        raw_payload: {
          text: candidate.text,
          class_name: candidate.className,
          url: snapshot.url
        }
      })
    }
    if (groups.length === 0) {
      return fail('GROUP_LIST_NOT_FOUND', 'no supported supplier group options were found')
    }
    return ok({
      source: 'chrome',
      captured_at: nowISO(),
      groups
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
    let headers = []
    for (const row of snapshot.rows || []) {
      const cells = normalizeCells(row.cells)
      if (cells.length === 0) continue
      if (isBillHeaderRow(cells)) {
        headers = cells
        continue
      }
      const bill = parseBillRow(cells, headers, snapshot, row)
      if (!bill) continue
      lines.push(bill)
    }
    if (lines.length === 0) {
      return fail('BILL_TABLE_NOT_FOUND', 'no supported billing rows were found')
    }
    return ok({ source: 'chrome', lines })
  }

  function parseBillRow(cells, headers, snapshot, row) {
    const costCell = cellByHeader(cells, headers, ['费用', '金额', '成本', 'price', 'cost', 'amount'])
    const cost = costCell ? findMoneyOrNumber([costCell]) : findMoneyOrNumber(cells)
    const model = cellByHeader(cells, headers, ['模型', 'model']) || findModel(cells)
    const requestID = cellByHeader(cells, headers, ['请求 id', '请求id', 'request id', 'request_id', 'req id']) || findRequestID(cells)
    if (cost === null || !model) return null

    const inputTokens = parseIntegerCell(cellByHeader(cells, headers, ['输入 token', '输入token', 'input token', 'prompt token'])) ?? findTokenValue(cells, ['input', 'prompt', '输入'])
    const outputTokens = parseIntegerCell(cellByHeader(cells, headers, ['输出 token', '输出token', 'output token', 'completion token'])) ?? findTokenValue(cells, ['output', 'completion', '输出'])
    const cacheReadTokens = parseIntegerCell(cellByHeader(cells, headers, ['缓存读取 token', '缓存 token', 'cache read token', 'cache token', 'cached token'])) ?? findTokenValue(cells, ['cache', 'cached', '缓存'])
    const explicitTotalTokens = parseIntegerCell(cellByHeader(cells, headers, ['总 token', '总token', 'total token', 'total tokens']))
    const totalTokens = explicitTotalTokens ?? (inputTokens + outputTokens + cacheReadTokens)
    const firstTokenCell = cellByHeader(cells, headers, ['首 token', '首token', 'ttft', 'first token', 'first_token'])
    const durationCell = cellByHeader(cells, headers, ['耗时', '总耗时', 'duration', 'latency', 'time cost'])

    return {
      external_bill_id: requestID || `${snapshot.host || 'supplier'}-${row.index}`,
      external_request_id: requestID,
      api_key_name: cellByHeader(cells, headers, ['api 密钥', 'api key', 'apikey', 'key', '密钥']),
      model,
      endpoint: cellByHeader(cells, headers, ['端点', 'endpoint', 'path', 'url']),
      request_type: cellByHeader(cells, headers, ['类型', '请求类型', 'type', 'request type']),
      billing_mode: normalizeBillingMode(cellByHeader(cells, headers, ['计费模式', 'billing mode', 'billing'])) || inferBillingMode(cells),
      reasoning_effort: cellByHeader(cells, headers, ['推理强度', 'reasoning effort', 'effort']),
      currency: inferCurrency(costCell ? [costCell] : cells),
      cost_cents: Math.round(cost * 100),
      input_tokens: inputTokens,
      output_tokens: outputTokens,
      cache_read_tokens: cacheReadTokens,
      total_tokens: totalTokens,
      first_token_ms: parseDurationMs(firstTokenCell),
      duration_ms: parseDurationMs(durationCell),
      user_agent: cellByHeader(cells, headers, ['user-agent', 'user agent', 'ua']),
      started_at: findDate([cellByHeader(cells, headers, ['时间', '创建时间', '开始时间', 'time', 'created at', 'started at'])].filter(Boolean)) || findDate(cells) || nowISO(),
      raw_payload: { cells, headers, url: snapshot.url }
    }
  }

  function isBillHeaderRow(cells) {
    const text = cells.join(' ').toLowerCase()
    const billHeaderHits = [
      /api\s*key|api\s*密钥|密钥/i,
      /model|模型/i,
      /计费|billing|费用|amount|cost|price/i,
      /token/i,
      /耗时|latency|duration|ttft/i
    ].filter((pattern) => pattern.test(text)).length
    return billHeaderHits >= 2
  }

  function cellByHeader(cells, headers, aliases) {
    if (!headers || headers.length === 0) return ''
    for (let index = 0; index < headers.length && index < cells.length; index += 1) {
      const header = normalizeHeader(headers[index])
      if (!header) continue
      if (aliases.some((alias) => header.includes(normalizeHeader(alias)))) {
        return cells[index] || ''
      }
    }
    return ''
  }

  function normalizeHeader(value) {
    return String(value || '')
      .toLowerCase()
      .replace(/[_-]+/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
  }

  function findRequestID(cells) {
    return cells.find((value) => /req|chatcmpl|cmpl|request[_-]?id|请求.?id|[a-f0-9-]{16,}/i.test(value)) || ''
  }

  function parseIntegerCell(value) {
    if (!value) return null
    const match = String(value).replace(/,/g, '').match(/(\d+)/)
    if (!match) return null
    const parsed = Number(match[1])
    return Number.isFinite(parsed) ? parsed : null
  }

  function parseDurationMs(value) {
    if (!value) return 0
    const text = String(value).trim().replace(/,/g, '')
    const match = text.match(/(\d+(?:\.\d+)?)\s*(ms|毫秒|s|sec|秒)?/i)
    if (!match) return 0
    const parsed = Number(match[1])
    if (!Number.isFinite(parsed)) return 0
    const unit = (match[2] || '').toLowerCase()
    if (unit === 's' || unit === 'sec' || unit === '秒') return Math.round(parsed * 1000)
    return Math.round(parsed)
  }

  function normalizeBillingMode(value) {
    const text = String(value || '').toLowerCase()
    if (!text) return ''
    if (/request|按次|请求/.test(text)) return 'request'
    if (/image|图片/.test(text)) return 'image'
    if (/token|按量/.test(text)) return 'token'
    return text.slice(0, 60)
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
      case 'fetch_groups':
        return collectGroups(snapshot)
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

  function collectGroupCandidates(snapshot) {
    const nodes = snapshot.groupOptions || []
    if (nodes.length > 0) {
      return nodes
    }
    const fromRows = (snapshot.rows || [])
      .map((row) => ({ text: normalizeCells(row.cells).join('\n'), className: '' }))
      .filter((item) => /倍率|rate|private|私有|default|claude|openai|gemini|anthropic|antigravity/i.test(item.text))
    const fromText = String(snapshot.text || '')
      .split(/\n(?=.*(?:倍率|rate|私有|private|default|claude|openai|gemini|anthropic|antigravity))/i)
      .map((text) => ({ text: text.trim(), className: '' }))
      .filter((item) => item.text)
    return [...fromRows, ...fromText]
  }

  function parseGroupCandidate(candidate) {
    const text = String(candidate.text || '').replace(/\r/g, '\n').trim()
    const lines = text.split(/\n+/).map((line) => line.trim()).filter(Boolean)
    const rateMultiplier = parseRateMultiplier(text)
    const privateMatch = /private|私有/i.test(text)
    const platform = inferPlatform(`${text} ${candidate.className || ''}`)
    let name = ''
    let description = ''
    for (const line of lines) {
      if (/^\d+(?:\.\d+)?x?\s*倍率$/i.test(line)) continue
      if (/^倍率[:：]/.test(line)) continue
      if (/^(private|私有)$/i.test(line)) continue
      if (!name) {
        name = cleanupGroupName(line)
        continue
      }
      if (!description && line !== name && !/^\d+(?:\.\d+)?x?\s*倍率$/i.test(line)) {
        description = line
      }
    }
    if (!description && lines.length > 1) {
      description = lines.find((line) => line !== name && !/倍率|private|私有/i.test(line)) || ''
    }
    return {
      name,
      description,
      platform,
      rate_multiplier: rateMultiplier,
      is_private: privateMatch
    }
  }

  function cleanupGroupName(value) {
    return String(value || '')
      .replace(/\b(private|私有)\b/gi, '')
      .replace(/私有/g, '')
      .replace(/\d+(?:\.\d+)?x?\s*倍率/gi, '')
      .replace(/\s+/g, ' ')
      .trim()
  }

  function parseRateMultiplier(text) {
    const match = String(text || '').match(/(\d+(?:\.\d+)?)\s*x?\s*倍率/i)
    if (!match) return null
    const value = Number(match[1])
    return Number.isFinite(value) ? value : null
  }

  function inferPlatform(text) {
    const lower = String(text || '').toLowerCase()
    if (lower.includes('anthropic') || lower.includes('claude')) return 'anthropic'
    if (lower.includes('openai') || lower.includes('gpt')) return 'openai'
    if (lower.includes('gemini')) return 'gemini'
    if (lower.includes('antigravity')) return 'antigravity'
    return ''
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
    collectGroups,
    collectBalance,
    collectPromotions,
    collectBills,
    collectHealth,
    helpers: {
      findModel,
      parseGroupCandidate,
      parseRateMultiplier,
      inferPlatform,
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
