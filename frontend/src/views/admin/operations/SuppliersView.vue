<template>
  <AppLayout>
    <SupplierTableSection :vm="vm" />
    <SupplierRowActionsMenu :vm="vm" />
    <SupplierEditorDialogs :vm="vm" />
    <SupplierSessionDialog :vm="vm" />
    <SupplierChannelStatusDialog :vm="vm" />
    <SupplierGroupsDialog :vm="vm" />
    <SupplierChannelProbeDialog :vm="vm" />
    <SupplierProvisionDialog :vm="vm" />
    <SupplierScheduleDialogs :vm="vm" />
    <SupplierRepairDeleteDialogs :vm="vm" />
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import SupplierTableSection from './suppliers/SupplierTableSection.vue'
import SupplierRowActionsMenu from './suppliers/SupplierRowActionsMenu.vue'
import SupplierEditorDialogs from './suppliers/SupplierEditorDialogs.vue'
import SupplierSessionDialog from './suppliers/SupplierSessionDialog.vue'
import SupplierChannelStatusDialog from './suppliers/SupplierChannelStatusDialog.vue'
import SupplierChannelProbeDialog from './suppliers/SupplierChannelProbeDialog.vue'
import SupplierGroupsDialog from './suppliers/SupplierGroupsDialog.vue'
import SupplierProvisionDialog from './suppliers/SupplierProvisionDialog.vue'
import SupplierScheduleDialogs from './suppliers/SupplierScheduleDialogs.vue'
import SupplierRepairDeleteDialogs from './suppliers/SupplierRepairDeleteDialogs.vue'
import { useSuppliersViewModel } from './suppliers/useSuppliersViewModel'

const vm = useSuppliersViewModel()
</script>
