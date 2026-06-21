<template>
  <AppLayout>
    <div class="p-6 text-sm text-gray-500 dark:text-dark-400">正在进入调度与插件采集...</div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'

const router = useRouter()

onMounted(() => {
  void router.replace('/admin/collection/plugin-tasks')
})
</script>
