<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="border-b border-gray-200 pb-4 dark:border-dark-700">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="flex min-w-0 items-start gap-3">
            <RouterLink to="/admin/suppliers" class="btn btn-secondary h-9 w-9 shrink-0 p-0" title="返回供应商列表">
              <Icon name="arrowLeft" size="sm" />
              <span class="sr-only">返回供应商列表</span>
            </RouterLink>
            <div class="min-w-0">
              <h1 class="truncate text-xl font-semibold text-gray-900 dark:text-gray-100">{{ supplier?.name || '供应商详情' }}</h1>
              <div v-if="supplier" class="mt-1 flex flex-wrap items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
                <span class="badge badge-gray">{{ supplier.type }}</span>
                <span>{{ supplier.api_base_url || supplier.dashboard_url || '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <nav class="mt-5 flex gap-5" aria-label="供应商详情">
          <button type="button" class="border-b-2 px-1 pb-2 text-sm font-medium" :class="activeTab === 'groups' ? 'border-primary-500 text-primary-700 dark:text-primary-300' : 'border-transparent text-gray-500 dark:text-dark-400'" @click="setTab('groups')">分组</button>
          <button type="button" class="border-b-2 px-1 pb-2 text-sm font-medium" :class="activeTab === 'tasks' ? 'border-primary-500 text-primary-700 dark:text-primary-300' : 'border-transparent text-gray-500 dark:text-dark-400'" @click="setTab('tasks')">任务记录</button>
        </nav>
      </header>

      <div v-if="loading" class="py-16 text-center text-sm text-gray-500 dark:text-dark-400">正在加载供应商...</div>
      <div v-else-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">{{ error }}</div>
      <SupplierGroupWorkspace v-else-if="supplier && activeTab === 'groups'" :vm="vm" />
      <SupplierTasksPanel v-else-if="supplier" :supplier-id="supplier.id" />
    </div>

    <SupplierProvisionDialog :vm="vm" />
    <SupplierRepairDeleteDialogs :vm="vm" />
    <SupplierChannelProbeDialog :vm="vm" />
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getSupplier } from '@/api/admin/adminPlus'
import type { Supplier } from '@/api/admin/adminPlus'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SupplierChannelProbeDialog from './suppliers/SupplierChannelProbeDialog.vue'
import SupplierGroupWorkspace from './suppliers/SupplierGroupWorkspace.vue'
import SupplierProvisionDialog from './suppliers/SupplierProvisionDialog.vue'
import SupplierRepairDeleteDialogs from './suppliers/SupplierRepairDeleteDialogs.vue'
import SupplierTasksPanel from './suppliers/SupplierTasksPanel.vue'
import { useSuppliersViewModel } from './suppliers/useSuppliersViewModel'

const route = useRoute()
const router = useRouter()
const vm: any = useSuppliersViewModel()
const supplier = ref<Supplier | null>(null)
const loading = ref(true)
const error = ref('')
const activeTab = ref(route.query.tab === 'tasks' ? 'tasks' : 'groups')

async function loadSupplierWorkspace() {
  const supplierID = Number(route.params.supplierId || 0)
  if (!supplierID) {
    error.value = '供应商 ID 无效'
    loading.value = false
    return
  }
  loading.value = true
  error.value = ''
  try {
    supplier.value = await getSupplier(supplierID)
    vm.initializeGroupsWorkspace(supplier.value)
    if (route.query.create_keys === '1') {
      await vm.previewEnsureCurrentKeys()
      const query = { ...route.query }
      delete query.create_keys
      await router.replace({ query })
    }
  } catch (cause) {
    error.value = (cause as { message?: string }).message || '加载供应商详情失败'
  } finally {
    loading.value = false
  }
}

function setTab(tab: 'groups' | 'tasks') {
  activeTab.value = tab
  void router.replace({ query: tab === 'tasks' ? { ...route.query, tab: 'tasks' } : {} })
}

watch(() => route.params.supplierId, loadSupplierWorkspace)
watch(() => route.query.tab, (tab) => {
  activeTab.value = tab === 'tasks' ? 'tasks' : 'groups'
})
onMounted(loadSupplierWorkspace)
onBeforeUnmount(() => vm.closeGroupsDialog())
</script>
