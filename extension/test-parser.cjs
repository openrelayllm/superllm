const assert = require('node:assert/strict')
const parser = require('./src/content/parser.js')

const url = 'https://supplier.example.com/admin'

const rates = parser.collectRates({
  url,
  rows: [
    { index: 0, cells: ['Model', 'Input', 'Output'] },
    { index: 1, cells: ['gpt-4o-mini', 'input / 1M tokens', '$0.150'] },
    { index: 2, cells: ['claude-3-5-sonnet', 'output / 1M tokens', 'USD 3.000'] }
  ]
})
assert.equal(rates.ok, true)
assert.equal(rates.result.entries.length, 2)
assert.equal(rates.result.entries[0].model, 'gpt-4o-mini')
assert.equal(rates.result.entries[0].price_item, 'input')
assert.equal(rates.result.entries[0].unit, '1m_tokens')
assert.equal(rates.result.entries[0].price_micros, 150000)

const balance = parser.collectBalance({
  url,
  text: '账户信息\n可用余额：￥123.45\n到期时间：2026-12-31'
})
assert.equal(balance.ok, true)
assert.equal(balance.result.balance_cents, 12345)
assert.equal(balance.result.currency, 'CNY')

const promotions = parser.collectPromotions({
  url,
  text: '充值优惠：充 1000 赠送 20%\n其它说明'
})
assert.equal(promotions.ok, true)
assert.equal(promotions.result.promotions[0].type, 'recharge_bonus')

const bills = parser.collectBills({
  url,
  host: 'supplier.example.com',
  rows: [
    { index: 1, cells: ['2026-06-20T10:00:00Z', 'req-abc-1234567890abcdef', 'gpt-4o-mini', 'input tokens 1000', 'output tokens 300', '$1.23'] }
  ]
})
assert.equal(bills.ok, true)
assert.equal(bills.result.lines.length, 1)
assert.equal(bills.result.lines[0].external_request_id, 'req-abc-1234567890abcdef')
assert.equal(bills.result.lines[0].cost_cents, 123)
assert.equal(bills.result.lines[0].input_tokens, 1000)
assert.equal(bills.result.lines[0].output_tokens, 300)

const health = parser.collectHealth({
  url,
  text: '运行状态\n并发：7 / 10\n其它指标'
})
assert.equal(health.ok, true)
assert.equal(health.result.observed_concurrency, 7)
assert.equal(health.result.available_concurrency, 3)
assert.equal(health.result.concurrency_limit, 10)

const unsupported = parser.collectRates({ url, rows: [{ index: 0, cells: ['empty'] }] })
assert.equal(unsupported.ok, false)
assert.equal(unsupported.error_code, 'RATE_TABLE_NOT_FOUND')

console.log('extension parser tests passed')
