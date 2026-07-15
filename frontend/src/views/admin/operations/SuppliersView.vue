<template>
  <AppLayout>
    <SupplierTableSection :vm="vm" />
    <SupplierRowActionsMenu :vm="vm" />
    <SupplierEditorDialogs :vm="vm" />
    <SupplierSessionDialog :vm="vm" />
    <SupplierChannelStatusDialog :vm="vm" />
    <SupplierDetailDialog :vm="vm" />
    <SupplierGroupsDialog :vm="vm" />
    <SupplierChannelProbeDialog :vm="vm" />
    <SupplierProvisionDialog :vm="vm" />
    <SupplierScheduleDialogs :vm="vm" />
    <SupplierRepairDeleteDialogs :vm="vm" />
  </AppLayout>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import SupplierTableSection from './suppliers/SupplierTableSection.vue'
import SupplierRowActionsMenu from './suppliers/SupplierRowActionsMenu.vue'
import SupplierEditorDialogs from './suppliers/SupplierEditorDialogs.vue'
import SupplierSessionDialog from './suppliers/SupplierSessionDialog.vue'
import SupplierChannelStatusDialog from './suppliers/SupplierChannelStatusDialog.vue'
import SupplierChannelProbeDialog from './suppliers/SupplierChannelProbeDialog.vue'
import SupplierDetailDialog from './suppliers/SupplierDetailDialog.vue'
import SupplierGroupsDialog from './suppliers/SupplierGroupsDialog.vue'
import SupplierProvisionDialog from './suppliers/SupplierProvisionDialog.vue'
import SupplierScheduleDialogs from './suppliers/SupplierScheduleDialogs.vue'
import SupplierRepairDeleteDialogs from './suppliers/SupplierRepairDeleteDialogs.vue'
import { useSuppliersViewModel } from './suppliers/useSuppliersViewModel'

const vm = useSuppliersViewModel()
const route = useRoute()
const router = useRouter()

const legacySupplierID = route.query.open === 'groups' ? Number(route.query.supplier_id || 0) : 0
if (legacySupplierID > 0) {
  const query = route.query.tool === 'key-plan' ? { create_keys: '1' } : undefined
  void router.replace({ path: `/admin/suppliers/${legacySupplierID}`, query })
}
</script>
