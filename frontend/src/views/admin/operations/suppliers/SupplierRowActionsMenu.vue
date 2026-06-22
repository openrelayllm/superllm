<template>
<Teleport to="body">
  <div
    v-if="rowActionsMenuSupplier"
    data-supplier-row-actions-menu
    class="fixed z-[1200] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800"
    :style="rowActionsMenuStyle"
    role="menu"
    @click.stop
  >
    <div class="p-2">
      <button class="row-action-menu-item" role="menuitem" @click="openRowStatusDialog">
        <span class="row-action-menu-icon bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
          <Icon name="checkCircle" size="sm" />
        </span>
        <span>状态</span>
      </button>
      <button class="row-action-menu-item" role="menuitem" @click="openRowSessionDialog">
        <span class="row-action-menu-icon bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">
          <Icon name="shield" size="sm" />
        </span>
        <span>会话</span>
      </button>
      <button class="row-action-menu-item" role="menuitem" @click="openRowGroupsDialog">
        <span class="row-action-menu-icon bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
          <Icon name="database" size="sm" />
        </span>
        <span>分组</span>
      </button>
      <button class="row-action-menu-item" role="menuitem" @click="openRowChannelStatusDialog">
        <span class="row-action-menu-icon bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300">
          <Icon name="chart" size="sm" />
        </span>
        <span>渠道状态</span>
      </button>
      <a
        v-if="rowActionsMenuSupplier?.third_party_recharge_url"
        class="row-action-menu-item"
        role="menuitem"
        :href="rowActionsMenuSupplier.third_party_recharge_url"
        target="_blank"
        rel="noopener noreferrer"
        @click="closeRowActionsMenu"
      >
        <span class="row-action-menu-icon bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">
          <Icon name="externalLink" size="sm" />
        </span>
        <span>第三方兑换</span>
      </a>
      <button
        v-else
        class="row-action-menu-item"
        role="menuitem"
        disabled
        title="未配置第三方兑换入口，请编辑供应商补齐"
      >
        <span class="row-action-menu-icon bg-gray-100 text-gray-400 dark:bg-gray-700 dark:text-gray-500">
          <Icon name="externalLink" size="sm" />
        </span>
        <span>第三方兑换</span>
      </button>
      <div class="my-2 border-t border-gray-100 dark:border-gray-700"></div>
      <button class="row-action-menu-item text-red-600 dark:text-red-300" role="menuitem" @click="openRowDeleteDialog">
        <span class="row-action-menu-icon bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300">
          <Icon name="trash" size="sm" />
        </span>
        <span>删除</span>
      </button>
    </div>
  </div>
</Teleport>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ vm: any }>()
const {
  rowActionsMenuSupplier,
  rowActionsMenuStyle,
  closeRowActionsMenu,
  openRowStatusDialog,
  openRowSessionDialog,
  openRowGroupsDialog,
  openRowChannelStatusDialog,
  openRowDeleteDialog
} = props.vm
</script>

<style scoped>
.row-action-menu-item {
  @apply flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-gray-700 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-200 dark:hover:bg-gray-700;
}

.row-action-menu-icon {
  @apply flex h-8 w-8 shrink-0 items-center justify-center rounded-md;
}
</style>
