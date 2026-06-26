import type { RouteLocationGeneric, RouteRecordRaw } from 'vue-router'

export const ADMIN_HOME = '/admin/dashboard'

const adminMeta = (title: string, extra: Record<string, unknown> = {}) => ({
  requiresAuth: true,
  requiresAdmin: true,
  title,
  ...extra
})

const redirectWithQuery = (path: string) => (to: RouteLocationGeneric) => ({
  path,
  query: to.query
})

export const adminPlusRoutes: RouteRecordRaw[] = [
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/',
    redirect: ADMIN_HOME
  },
  {
    path: '/admin',
    redirect: ADMIN_HOME
  },
  {
    path: ADMIN_HOME,
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: adminMeta('Admin Dashboard', {
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    })
  },
  {
    path: '/admin/ops',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/suppliers',
    name: 'AdminPlusSuppliers',
    component: () => import('@/views/admin/operations/SuppliersView.vue'),
    meta: adminMeta('供应商管理')
  },
  {
    path: '/admin/supplier-bindings',
    name: 'AdminPlusSupplierBindings',
    component: () => import('@/views/admin/operations/SupplierAccountsView.vue'),
    meta: adminMeta('账号/Key 绑定')
  },
  {
    path: '/admin/scheduler',
    name: 'AdminPlusSchedulerCenter',
    component: () => import('@/views/admin/scheduler/SchedulerCenterView.vue'),
    meta: adminMeta('调度中心')
  },
  {
    path: '/admin/scheduler/notifications',
    name: 'AdminPlusNotificationCenter',
    component: () => import('@/views/admin/scheduler/NotificationCenterView.vue'),
    meta: adminMeta('通知中心')
  },
  {
    path: '/admin/system-logs',
    name: 'AdminPlusSystemLogs',
    component: () => import('@/views/admin/operations/SystemLogsView.vue'),
    meta: adminMeta('系统日志')
  },
  {
    path: '/admin/collection/scheduler',
    redirect: redirectWithQuery('/admin/scheduler')
  },
  {
    path: '/admin/collection/plugin-tasks',
    redirect: redirectWithQuery('/admin/collection/sessions')
  },
  {
    path: '/admin/collection/sessions',
    name: 'AdminPlusCollectionSessions',
    component: () => import('@/views/admin/operations/ExtensionTasksView.vue'),
    meta: adminMeta('采集会话')
  },
  {
    path: '/admin/collection/site-discovery',
    name: 'AdminPlusSiteDiscovery',
    component: () => import('@/views/admin/operations/SiteDiscoveryView.vue'),
    meta: adminMeta('渠道索引采集')
  },
  {
    path: '/admin/site-catalog',
    name: 'AdminPlusSiteCatalog',
    component: () => import('@/views/admin/operations/SiteCatalogView.vue'),
    meta: adminMeta('网址目录')
  },
  {
    path: '/admin/mails',
    name: 'AdminPlusMailVerification',
    component: () => import('@/views/admin/operations/MailVerificationView.vue'),
    meta: adminMeta('邮箱验证码')
  },
  {
    path: '/admin/proxy',
    name: 'AdminPlusProxyManager',
    component: () => import('@/views/admin/operations/ProxyManagerView.vue'),
    meta: adminMeta('代理出口管理')
  },
  {
    path: '/admin/events/announcements',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/monitoring/rates',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/monitoring/balances',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/monitoring/health',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/monitoring/account-runtime',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/monitoring/announcements',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/finance/costs',
    name: 'AdminPlusSupplierCosts',
    component: () => import('@/views/admin/operations/SupplierCostsView.vue'),
    meta: adminMeta('成本对账')
  },
  {
    path: '/admin/finance/usage-costs',
    name: 'AdminPlusSupplierUsageCosts',
    component: () => import('@/views/admin/operations/SupplierUsageCostsView.vue'),
    meta: adminMeta('用量消耗')
  },
  {
    path: '/admin/finance/local-usage',
    name: 'AdminPlusLocalUsage',
    component: () => import('@/views/admin/operations/LocalUsageView.vue'),
    meta: adminMeta('本地用量')
  },
  {
    path: '/admin/automation/actions',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/automation/notifications',
    redirect: redirectWithQuery('/admin/scheduler/notifications')
  },
  {
    path: '/admin/automation/audits',
    redirect: redirectWithQuery('/admin/system-logs')
  },
  {
    path: '/admin/operations',
    redirect: '/admin/suppliers'
  },
  {
    path: '/admin/operations/suppliers',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/supplier-accounts',
    redirect: redirectWithQuery('/admin/supplier-bindings')
  },
  {
    path: '/admin/operations/account-runtime',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/rates',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/balances',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/health',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/announcements',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/scheduler',
    redirect: redirectWithQuery('/admin/scheduler')
  },
  {
    path: '/admin/operations/extension-tasks',
    redirect: redirectWithQuery('/admin/collection/sessions')
  },
  {
    path: '/admin/operations/billing',
    redirect: redirectWithQuery('/admin/finance/costs')
  },
  {
    path: '/admin/operations/actions',
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/operations/notifications',
    redirect: redirectWithQuery('/admin/scheduler/notifications')
  },
  {
    path: '/admin/operations/audits',
    redirect: redirectWithQuery('/admin/system-logs')
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: adminMeta('System Settings', {
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description'
    })
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      requiresAuth: false,
      title: '404 Not Found'
    }
  }
]
