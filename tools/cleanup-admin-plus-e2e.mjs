#!/usr/bin/env node

import { execFileSync } from 'node:child_process'
import process from 'node:process'

const dbURL = process.env.ADMIN_PLUS_E2E_DB_URL || 'postgresql://root:root@127.0.0.1:5432/sub2api_admin_plus?sslmode=disable'
const redisURL = process.env.ADMIN_PLUS_E2E_REDIS_URL || 'redis://127.0.0.1:6379/0'
const execute = process.env.ADMIN_PLUS_CLEAN_E2E_EXECUTE === '1'
const allowNonLocal = process.env.ADMIN_PLUS_E2E_ALLOW_NON_LOCAL === '1'

main()

function main() {
  assertSafeTarget()
  const counts = queryCounts()
  console.log(formatCounts(counts))
  if (!execute) {
    console.log('dry-run only. Set ADMIN_PLUS_CLEAN_E2E_EXECUTE=1 to delete these E2E fixtures.')
    return
  }
  deleteFixtures()
  cleanupRedis()
  console.log('deleted E2E fixtures')
}

function queryCounts() {
  const sql = `
    SELECT 'admin_plus_suppliers', COUNT(*) FROM admin_plus_suppliers
    WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_supplier_bill_lines', COUNT(*) FROM admin_plus_supplier_bill_lines
    WHERE external_bill_id LIKE 'e2e-%' OR external_request_id LIKE 'e2e-%' OR model LIKE 'e2e-%' OR raw_payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_extension_tasks', COUNT(*) FROM admin_plus_extension_tasks
    WHERE device_id LIKE 'e2e-%' OR schedule_key LIKE '%e2e-%' OR payload::text LIKE '%e2e-%' OR result::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_rate_snapshots', COUNT(*) FROM admin_plus_rate_snapshots
    WHERE model LIKE 'e2e-%' OR raw_payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_rate_change_events', COUNT(*) FROM admin_plus_rate_change_events
    WHERE model LIKE 'e2e-%'
    UNION ALL
    SELECT 'admin_plus_balance_snapshots', COUNT(*) FROM admin_plus_balance_snapshots
    WHERE raw_payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_health_samples', COUNT(*) FROM admin_plus_health_samples
    WHERE model LIKE 'e2e-%' OR raw_payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_health_events', COUNT(*) FROM admin_plus_health_events
    WHERE model LIKE 'e2e-%'
    UNION ALL
    SELECT 'admin_plus_promotion_events', COUNT(*) FROM admin_plus_promotion_events
    WHERE title LIKE '%e2e-%' OR description LIKE '%e2e-%' OR raw_payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_action_recommendations', COUNT(*) FROM admin_plus_action_recommendations
    WHERE reason_code LIKE '%e2e-%' OR title LIKE '%e2e-%' OR description LIKE '%e2e-%' OR expected_impact LIKE '%e2e-%' OR signals::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'admin_plus_notification_deliveries', COUNT(*) FROM admin_plus_notification_deliveries
    WHERE dedupe_key LIKE '%e2e-%' OR payload::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'usage_logs', COUNT(*) FROM usage_logs
    WHERE request_id LIKE 'e2e-%' OR model LIKE 'e2e-%' OR requested_model LIKE 'e2e-%'
    UNION ALL
    SELECT 'api_keys', COUNT(*) FROM api_keys
    WHERE key LIKE 'sk-e2e-%' OR name LIKE 'e2e-%'
    UNION ALL
    SELECT 'accounts', COUNT(*) FROM accounts
    WHERE name LIKE 'e2e-%' OR extra::text LIKE '%admin-plus-e2e%' OR extra::text LIKE '%e2e-%'
    UNION ALL
    SELECT 'users', COUNT(*) FROM users
    WHERE email LIKE 'e2e-%@e2e.local';
  `
  const out = psql(['-At', '-F', '\t', '-c', sql])
  return out
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => {
      const [table, count] = line.split('\t')
      return { table, count: Number(count) }
    })
}

function deleteFixtures() {
  const sql = `
    DELETE FROM admin_plus_notification_deliveries
    WHERE dedupe_key LIKE '%e2e-%' OR payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_action_recommendations
    WHERE reason_code LIKE '%e2e-%' OR title LIKE '%e2e-%' OR description LIKE '%e2e-%' OR expected_impact LIKE '%e2e-%' OR signals::text LIKE '%e2e-%';

    DELETE FROM admin_plus_supplier_bill_lines
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR external_bill_id LIKE 'e2e-%'
       OR external_request_id LIKE 'e2e-%'
       OR model LIKE 'e2e-%'
       OR raw_payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_extension_tasks
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR device_id LIKE 'e2e-%'
       OR schedule_key LIKE '%e2e-%'
       OR payload::text LIKE '%e2e-%'
       OR result::text LIKE '%e2e-%';

    DELETE FROM admin_plus_health_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR model LIKE 'e2e-%';

    DELETE FROM admin_plus_health_samples
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR model LIKE 'e2e-%'
       OR raw_payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_promotion_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR title LIKE '%e2e-%'
       OR description LIKE '%e2e-%'
       OR raw_payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_balance_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%');

    DELETE FROM admin_plus_balance_snapshots
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR raw_payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_rate_change_events
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR model LIKE 'e2e-%';

    DELETE FROM admin_plus_rate_snapshots
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR model LIKE 'e2e-%'
       OR raw_payload::text LIKE '%e2e-%';

    DELETE FROM admin_plus_supplier_accounts
    WHERE supplier_id IN (SELECT id FROM admin_plus_suppliers WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%')
       OR supplier_account_identifier LIKE 'e2e-%';

    DELETE FROM admin_plus_suppliers
    WHERE name LIKE 'e2e-%' OR notes LIKE '%e2e-%';

    DELETE FROM usage_logs
    WHERE request_id LIKE 'e2e-%' OR model LIKE 'e2e-%' OR requested_model LIKE 'e2e-%';

    DELETE FROM api_keys
    WHERE key LIKE 'sk-e2e-%' OR name LIKE 'e2e-%';

    DELETE FROM accounts
    WHERE name LIKE 'e2e-%' OR extra::text LIKE '%admin-plus-e2e%' OR extra::text LIKE '%e2e-%';

    DELETE FROM users
    WHERE email LIKE 'e2e-%@e2e.local';
  `
  psql(['-v', 'ON_ERROR_STOP=1', '-c', sql])
}

function cleanupRedis() {
  try {
    execFileSync('redis-cli', ['-u', redisURL, '--scan', '--pattern', 'concurrency:account:*'], { encoding: 'utf8' })
    execFileSync('redis-cli', ['-u', redisURL, '--scan', '--pattern', 'wait:account:*'], { encoding: 'utf8' })
  } catch {
    // Redis runtime keys expire naturally; database cleanup is the important part.
  }
}

function psql(args) {
  return execFileSync('psql', [dbURL, '-v', 'ON_ERROR_STOP=1', ...args], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe']
  }).trim()
}

function formatCounts(counts) {
  return counts
    .filter((item) => item.count > 0)
    .map((item) => `${item.table}: ${item.count}`)
    .join('\n') || 'no E2E fixtures found'
}

function assertSafeTarget() {
  if (allowNonLocal) return
  const dbHost = new URL(dbURL).hostname
  const redisHost = new URL(redisURL).hostname
  assert(isLocalHost(dbHost), `refuse to clean non-local database host: ${dbHost}`)
  assert(isLocalHost(redisHost), `refuse to clean non-local Redis host: ${redisHost}`)
}

function isLocalHost(hostname) {
  return ['localhost', '127.0.0.1', '::1'].includes(hostname)
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message)
  }
}
