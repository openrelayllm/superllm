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
  it('只暴露 Admin Plus MVP0 当前路由和兼容重定向', () => {
    expect(collectRoutePaths(adminPlusRoutes)).toEqual([
      '/setup',
      '/login',
      '/',
      '/admin',
      '/admin/dashboard',
      '/admin/ops',
      '/admin/suppliers',
      '/admin/supplier-bindings',
      '/admin/scheduler',
      '/admin/scheduler/notifications',
      '/admin/collection/scheduler',
      '/admin/collection/plugin-tasks',
      '/admin/collection/sessions',
      '/admin/collection/site-discovery',
      '/admin/site-catalog',
      '/admin/mails',
      '/admin/events/announcements',
      '/admin/monitoring/rates',
      '/admin/monitoring/balances',
      '/admin/monitoring/health',
      '/admin/monitoring/account-runtime',
      '/admin/monitoring/announcements',
      '/admin/finance/costs',
      '/admin/finance/usage-costs',
      '/admin/finance/local-usage',
      '/admin/automation/actions',
      '/admin/automation/notifications',
      '/admin/automation/audits',
      '/admin/operations',
      '/admin/operations/suppliers',
      '/admin/operations/supplier-accounts',
      '/admin/operations/account-runtime',
      '/admin/operations/rates',
      '/admin/operations/balances',
      '/admin/operations/health',
      '/admin/operations/announcements',
      '/admin/operations/scheduler',
      '/admin/operations/extension-tasks',
      '/admin/operations/billing',
      '/admin/operations/actions',
      '/admin/operations/notifications',
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
      '/admin/backup',
      '/admin/finance/billing',
      '/admin/finance/reconciliation',
      '/admin/operations/promotions'
    ]

    for (const deadPath of deadPaths) {
      expect(paths).not.toContain(deadPath)
    }
  })

  it('后台业务页面必须要求管理员身份', () => {
    const adminRoutes = adminPlusRoutes.filter((route) =>
      [
        '/admin/dashboard',
        '/admin/suppliers',
        '/admin/supplier-bindings',
        '/admin/scheduler',
        '/admin/scheduler/notifications',
        '/admin/collection/sessions',
        '/admin/collection/site-discovery',
        '/admin/site-catalog',
        '/admin/mails',
        '/admin/finance/costs',
        '/admin/finance/usage-costs',
        '/admin/finance/local-usage',
        '/admin/settings'
      ].includes(route.path)
    )

    expect(adminRoutes).toHaveLength(13)
    for (const route of adminRoutes) {
      expect(route.meta?.requiresAuth).toBe(true)
      expect(route.meta?.requiresAdmin).toBe(true)
      expect(route.component).toBeDefined()
    }
  })

  it('旧入口和降级页面只作为兼容重定向', () => {
    const redirects = new Map(
      [
        ['/admin/operations', '/admin/suppliers'],
        ['/admin/operations/suppliers', '/admin/suppliers'],
        ['/admin/operations/supplier-accounts', '/admin/supplier-bindings'],
        ['/admin/ops', '/admin/suppliers'],
        ['/admin/monitoring/rates', '/admin/suppliers'],
        ['/admin/monitoring/health', '/admin/suppliers'],
        ['/admin/monitoring/account-runtime', '/admin/suppliers'],
        ['/admin/operations/account-runtime', '/admin/suppliers'],
        ['/admin/operations/rates', '/admin/suppliers'],
        ['/admin/monitoring/balances', '/admin/suppliers'],
        ['/admin/operations/balances', '/admin/suppliers'],
        ['/admin/operations/health', '/admin/suppliers'],
        ['/admin/events/announcements', '/admin/suppliers'],
        ['/admin/monitoring/announcements', '/admin/suppliers'],
        ['/admin/operations/announcements', '/admin/suppliers'],
        ['/admin/collection/scheduler', '/admin/scheduler'],
        ['/admin/operations/scheduler', '/admin/scheduler'],
        ['/admin/collection/plugin-tasks', '/admin/collection/sessions'],
        ['/admin/operations/extension-tasks', '/admin/collection/sessions'],
        ['/admin/operations/billing', '/admin/finance/costs'],
        ['/admin/automation/actions', '/admin/suppliers'],
        ['/admin/automation/notifications', '/admin/scheduler/notifications'],
        ['/admin/automation/audits', '/admin/suppliers'],
        ['/admin/operations/actions', '/admin/suppliers'],
        ['/admin/operations/notifications', '/admin/scheduler/notifications']
      ]
    )

    for (const [path, target] of redirects) {
      const route = adminPlusRoutes.find((item) => item.path === path)
      expect(route?.component).toBeUndefined()
      if (typeof route?.redirect === 'function') {
        expect(route.redirect({ query: { q: 'abc' } } as never)).toEqual({ path: target, query: { q: 'abc' } })
      } else {
        expect(route?.redirect).toBe(target)
      }
    }
  })
})
