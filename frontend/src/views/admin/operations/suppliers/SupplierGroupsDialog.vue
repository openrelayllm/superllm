<template>
  <BaseDialog
    :show="groupsDialogOpen"
    :title="groupsSupplier ? `供应商分组 - ${groupsSupplier.name}` : '供应商分组'"
    width="full"
    @close="closeGroupsDialog"
  >
    <SupplierGroupWorkspace :vm="vm" />

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="closeGroupsDialog">关闭</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import BaseDialog from '@/components/common/BaseDialog.vue'
import SupplierGroupWorkspace from './SupplierGroupWorkspace.vue'

const props = defineProps<{ vm: any }>()
const { groupsDialogOpen, groupsSupplier, closeGroupsDialog } = props.vm
const vm = props.vm
</script>
