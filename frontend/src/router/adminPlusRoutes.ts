import type { RouteRecordRaw } from 'vue-router'

export const ADMIN_HOME = '/admin/dashboard'

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
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Ops Monitoring',
      titleKey: 'admin.ops.title',
      descriptionKey: 'admin.ops.description'
    }
  },
  {
    path: '/admin/operations',
    redirect: '/admin/operations/suppliers'
  },
  {
    path: '/admin/operations/suppliers',
    name: 'AdminPlusSuppliers',
    component: () => import('@/views/admin/operations/SuppliersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '供应商管理'
    }
  },
  {
    path: '/admin/operations/supplier-accounts',
    name: 'AdminPlusSupplierAccounts',
    component: () => import('@/views/admin/operations/SupplierAccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '账号/Key 绑定'
    }
  },
  {
    path: '/admin/operations/rates',
    name: 'AdminPlusRates',
    component: () => import('@/views/admin/operations/RatesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '费率监控'
    }
  },
  {
    path: '/admin/operations/balances',
    name: 'AdminPlusBalances',
    component: () => import('@/views/admin/operations/BalancesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '余额监控'
    }
  },
  {
    path: '/admin/operations/health',
    name: 'AdminPlusHealth',
    component: () => import('@/views/admin/operations/HealthView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '健康监控'
    }
  },
  {
    path: '/admin/operations/promotions',
    name: 'AdminPlusPromotions',
    component: () => import('@/views/admin/operations/PromotionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '优惠监控'
    }
  },
  {
    path: '/admin/operations/extension-tasks',
    name: 'AdminPlusExtensionTasks',
    component: () => import('@/views/admin/operations/ExtensionTasksView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '插件任务'
    }
  },
  {
    path: '/admin/operations/billing',
    name: 'AdminPlusBilling',
    component: () => import('@/views/admin/operations/BillingReconciliationView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '账单对账'
    }
  },
  {
    path: '/admin/operations/actions',
    name: 'AdminPlusActions',
    component: () => import('@/views/admin/operations/ActionRecommendationsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '动作建议'
    }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'System Settings',
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description'
    }
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
