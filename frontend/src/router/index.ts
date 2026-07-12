/**
 * SuperLLM route configuration.
 *
 * Fact source:
 * - current: setup, admin login, SuperLLM dashboard, ops, settings.
 * - dead: Sub2API public/user/payment/custom-page routes.
 */

import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { getSetupStatus } from '@/api/setup'
import { ADMIN_HOME, adminPlusRoutes } from './adminPlusRoutes'
import { resolveCompletedSetupRedirectPath } from './setupRedirect'
import { resolveDocumentTitle } from './title'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: adminPlusRoutes,
  scrollBehavior(_to, _from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    }
    return { top: 0 }
  }
})

let authInitialized = false

const navigationLoading = useNavigationLoadingState()
let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null

router.beforeEach(async (to, _from, next) => {
  navigationLoading.startNavigation()

  const authStore = useAuthStore()
  const appStore = useAppStore()

  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string)

  if (to.path === '/setup') {
    try {
      const status = await getSetupStatus()
      if (!status.needs_setup) {
        next(resolveCompletedSetupRedirectPath(authStore.isAuthenticated, authStore.isAdmin))
        return
      }
    } catch {
      // Keep setup reachable when the setup status endpoint is unavailable.
    }
  }

  const requiresAuth = to.meta.requiresAuth !== false
  const requiresAdmin = to.meta.requiresAdmin === true

  if (!requiresAuth) {
    if (authStore.isAuthenticated && to.path === '/login') {
      next(ADMIN_HOME)
      return
    }
    next()
    return
  }

  if (!authStore.isAuthenticated) {
    next({
      path: '/login',
      query: { redirect: to.fullPath }
    })
    return
  }

  if (requiresAdmin && !authStore.isAdmin) {
    next('/login')
    return
  }

  next()
})

router.afterEach((to) => {
  navigationLoading.endNavigation()

  if (!routePrefetch) {
    routePrefetch = useRoutePrefetch(router)
  }
  routePrefetch.triggerPrefetch(to)
})

router.onError((error) => {
  console.error('Router error:', error)

  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk') ||
    error.message?.includes('Loading CSS chunk') ||
    error.name === 'ChunkLoadError'

  if (isChunkLoadError) {
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()

    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      console.warn('Chunk load error detected, reloading page to fetch latest version...')
      window.location.reload()
    } else {
      console.error('Chunk load error persists after reload. Please clear browser cache.')
    }
  }
})

export default router
