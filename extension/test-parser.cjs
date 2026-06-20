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

const groups = parser.collectGroups({
  url,
  groupOptions: [
    {
      text: 'CLAUDE*0.95倍-只能用客户端和cli-纯血\nCLAUDE*0.95倍-只能用客户端和cli-纯血\n0.95x 倍率',
      className: ''
    },
    {
      text: 'private-u529-openai 私有\nPrivate subscription group for user 529 on openai.\n1x 倍率',
      className: ''
    },
    {
      text: 'PRO-7*24稳定【自己正价充的号兜底】\n0.145x 倍率',
      className: ''
    }
  ]
})
assert.equal(groups.ok, true)
assert.equal(groups.result.groups.length, 3)
assert.equal(groups.result.groups[0].rate_multiplier, 0.95)
assert.equal(groups.result.groups[1].platform, 'openai')
assert.equal(groups.result.groups[1].is_private, true)

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

const detailedBills = parser.collectBills({
  url,
  host: 'supplier.example.com',
  rows: [
    {
      index: 0,
      cells: [
        'API 密钥',
        '模型',
        '推理强度',
        '端点',
        '类型',
        '计费模式',
        '输入 Token',
        '输出 Token',
        '缓存读取 Token',
        '总 Token',
        '费用',
        '首 Token',
        '耗时',
        '时间',
        'User-Agent',
        '请求 ID'
      ]
    },
    {
      index: 1,
      cells: [
        'sk-prod',
        'gpt-5.5',
        'high',
        '/v1/responses',
        'responses',
        '按 Token',
        '1,200',
        '345',
        '600',
        '2,145',
        '￥0.87',
        '680ms',
        '2.4s',
        '2026-06-20T10:00:00Z',
        'OpenAI/Python',
        'req-detail-1234567890abcdef'
      ]
    }
  ]
})
assert.equal(detailedBills.ok, true)
assert.equal(detailedBills.result.lines[0].api_key_name, 'sk-prod')
assert.equal(detailedBills.result.lines[0].model, 'gpt-5.5')
assert.equal(detailedBills.result.lines[0].endpoint, '/v1/responses')
assert.equal(detailedBills.result.lines[0].request_type, 'responses')
assert.equal(detailedBills.result.lines[0].billing_mode, 'token')
assert.equal(detailedBills.result.lines[0].reasoning_effort, 'high')
assert.equal(detailedBills.result.lines[0].input_tokens, 1200)
assert.equal(detailedBills.result.lines[0].output_tokens, 345)
assert.equal(detailedBills.result.lines[0].cache_read_tokens, 600)
assert.equal(detailedBills.result.lines[0].total_tokens, 2145)
assert.equal(detailedBills.result.lines[0].cost_cents, 87)
assert.equal(detailedBills.result.lines[0].first_token_ms, 680)
assert.equal(detailedBills.result.lines[0].duration_ms, 2400)
assert.equal(detailedBills.result.lines[0].user_agent, 'OpenAI/Python')

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
