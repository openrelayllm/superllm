import type { SupplierCostSnapshot } from '@/api/admin/adminPlus'

type SnapshotLike = Pick<
  SupplierCostSnapshot,
  | 'completed_funding_amount_cents'
  | 'completed_funding_cash_cents'
  | 'recharge_actual_payment_cents'
  | 'entitlement_amount_cents'
  | 'usage_cost_cents'
  | 'refund_amount_cents'
  | 'adjustment_amount_cents'
  | 'expected_balance_cents'
  | 'actual_balance_cents'
  | 'balance_delta_cents'
>

export function supplierRechargeTotalCents(snapshot?: SnapshotLike | null): number {
  if (!snapshot) return 0
  return (snapshot.completed_funding_amount_cents || 0) + (snapshot.entitlement_amount_cents || 0)
}

export function supplierSnapshotRechargeMultiplier(snapshot?: SnapshotLike | null): number {
  const fundingCredited = Number(snapshot?.completed_funding_amount_cents || 0)
  const fundingCash = Number(snapshot?.completed_funding_cash_cents || 0)
  if (fundingCredited > 0 && fundingCash > 0 && fundingCredited > fundingCash) {
    const multiplier = fundingCredited / fundingCash
    return Number.isFinite(multiplier) && multiplier > 1 ? multiplier : 1
  }

  const credited = supplierRechargeTotalCents(snapshot)
  const snapshotPaid = Number(snapshot?.recharge_actual_payment_cents || 0)
  if (!Number.isFinite(credited) || !Number.isFinite(snapshotPaid) || credited <= 0 || snapshotPaid <= 0 || credited <= snapshotPaid) return 1
  const multiplier = credited / snapshotPaid
  return Number.isFinite(multiplier) && multiplier > 1 ? multiplier : 1
}

export function supplierDisplayUsageCents(snapshot?: SnapshotLike | null): number {
  if (!snapshot) return 0
  if (snapshot.actual_balance_cents === null || snapshot.actual_balance_cents === undefined) {
    return snapshot.usage_cost_cents || 0
  }
  const balanceBeforeUsage = supplierRechargeTotalCents(snapshot) -
    (snapshot.refund_amount_cents || 0) +
    (snapshot.adjustment_amount_cents || 0)
  return Math.max(balanceBeforeUsage - snapshot.actual_balance_cents, 0)
}

export function supplierExpectedBalanceCents(snapshot?: SnapshotLike | null): number {
  if (!snapshot) return 0
  const balanceBeforeUsage = supplierRechargeTotalCents(snapshot) -
    (snapshot.refund_amount_cents || 0) +
    (snapshot.adjustment_amount_cents || 0)
  return balanceBeforeUsage - supplierDisplayUsageCents(snapshot)
}

export function supplierBalanceDeltaCents(snapshot?: SnapshotLike | null): number | null {
  if (!snapshot) return null
  if (snapshot.actual_balance_cents === null || snapshot.actual_balance_cents === undefined) {
    return snapshot.balance_delta_cents ?? null
  }
  return snapshot.actual_balance_cents - supplierExpectedBalanceCents(snapshot)
}
