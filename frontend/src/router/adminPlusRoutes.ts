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
    path: '/admin/collection/scheduler',
    name: 'AdminPlusCollectionScheduler',
    component: () => import('@/views/admin/operations/SchedulerView.vue'),
    meta: adminMeta('任务调度')
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
    name: 'AdminPlusAnnouncements',
    component: () => import('@/views/admin/operations/AnnouncementsView.vue'),
    meta: adminMeta('公告')
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
    redirect: redirectWithQuery('/admin/suppliers')
  },
  {
    path: '/admin/automation/audits',
    redirect: redirectWithQuery('/admin/suppliers')
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
    redirect: redirectWithQuery('/admin/monitoring/announcements')
  },
  {
    path: '/admin/operations/scheduler',
    redirect: redirectWithQuery('/admin/collection/scheduler')
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
    redirect: redirectWithQuery('/admin/suppliers')
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
