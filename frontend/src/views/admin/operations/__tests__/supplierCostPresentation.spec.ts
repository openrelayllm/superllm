import { describe, expect, it } from 'vitest'
import type { SupplierCostSnapshot } from '@/api/admin/adminPlus'
import { supplierSnapshotRechargeMultiplier } from '../supplierCostPresentation'

function snapshot(overrides: Partial<SupplierCostSnapshot> = {}): SupplierCostSnapshot {
  return {
    id: 1,
    supplier_id: 7,
    currency: 'USD',
    completed_funding_amount_cents: 0,
    completed_funding_cash_cents: 0,
    recharge_actual_payment_cents: 0,
    entitlement_amount_cents: 0,
    usage_cost_cents: 0,
    refund_amount_cents: 0,
    adjustment_amount_cents: 0,
    expected_balance_cents: 0,
    captured_at: '2026-06-22T00:00:00Z',
    created_at: '2026-06-22T00:00:00Z',
    ...overrides
  }
}

describe('supplierCostPresentation', () => {
  it('从充值订单到账额度和现金实付推导充值倍率', () => {
    expect(supplierSnapshotRechargeMultiplier(snapshot({
      completed_funding_amount_cents: 10000,
      completed_funding_cash_cents: 1000,
      recharge_actual_payment_cents: 10000
    }))).toBe(10)
  })

  it('没有订单现金实付时从快照总实付兜底推导', () => {
    expect(supplierSnapshotRechargeMultiplier(snapshot({
      completed_funding_amount_cents: 10000,
      entitlement_amount_cents: 2000,
      recharge_actual_payment_cents: 1200
    }))).toBe(10)
  })

  it('实付金额不低于到账额度时不放大倍率', () => {
    expect(supplierSnapshotRechargeMultiplier(snapshot({
      completed_funding_amount_cents: 10000,
      completed_funding_cash_cents: 10000,
      recharge_actual_payment_cents: 10000
    }))).toBe(1)
  })
})
