import type { RouteRecordRaw } from 'vue-router'
import { describe, expect, it } from 'vitest'
import { adminPlusRoutes } from '@/router/adminPlusRoutes'

function collectRoutePaths(routes: RouteRecordRaw[]): string[] {
  const paths: string[] = []

  const visit = (route: RouteRecordRaw) => {
    paths.push(route.path)
    route.children?.forEach(visit)
  }

  routes.forEach(visit)
  return paths
}

describe('adminPlusRoutes', () => {
  it('只暴露 Admin Plus MVP0 当前路由', () => {
    expect(collectRoutePaths(adminPlusRoutes)).toEqual([
      '/setup',
      '/login',
      '/',
      '/admin',
      '/admin/dashboard',
      '/admin/ops',
      '/admin/operations',
      '/admin/operations/suppliers',
      '/admin/operations/rates',
      '/admin/operations/balances',
      '/admin/operations/health',
      '/admin/operations/promotions',
      '/admin/operations/extension-tasks',
      '/admin/operations/billing',
      '/admin/operations/actions',
      '/admin/settings',
      '/:pathMatch(.*)*'
    ])
  })

  it('不回流 Sub2API 用户端、支付、OAuth 和旧后台页面', () => {
    const paths = collectRoutePaths(adminPlusRoutes)
    const deadPaths = [
      '/register',
      '/forgot-password',
      '/reset-password',
      '/verify-email',
      '/oauth/callback',
      '/auth/wechat/callback',
      '/keys',
      '/usage',
      '/profile',
      '/payment',
      '/orders',
      '/channels',
      '/admin/users',
      '/admin/accounts',
      '/admin/channels',
      '/admin/groups',
      '/admin/payment',
      '/admin/subscriptions',
      '/admin/redeem',
      '/admin/backup'
    ]

    for (const deadPath of deadPaths) {
      expect(paths).not.toContain(deadPath)
    }
  })

  it('后台业务页面必须要求管理员身份', () => {
    const adminRoutes = adminPlusRoutes.filter((route) =>
      [
        '/admin/dashboard',
        '/admin/ops',
        '/admin/operations/suppliers',
        '/admin/operations/rates',
        '/admin/operations/balances',
        '/admin/operations/health',
        '/admin/operations/promotions',
        '/admin/operations/extension-tasks',
        '/admin/operations/billing',
        '/admin/operations/actions',
        '/admin/settings'
      ].includes(route.path)
    )

    expect(adminRoutes).toHaveLength(11)
    for (const route of adminRoutes) {
      expect(route.meta?.requiresAuth).toBe(true)
      expect(route.meta?.requiresAdmin).toBe(true)
    }
  })
})
