<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { lvsApi, serverApi, extractErrorMessage } from '@/api'
import type { LvsPreprodBinding, ServerResponse } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })
const emit = defineEmits<{ (e: 'changed'): void }>()

const bindings = ref<LvsPreprodBinding[]>([])
const preprodServers = ref<ServerResponse[]>([])
const loading = ref(false)
const saving = ref(false)

const form = ref({
  vs_tag: '',
  rs_env_tag: '',
  preprod_server_id: undefined as number | undefined,
})

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [bs, ps] = await Promise.all([lvsApi.listBindings(), serverApi.list({ type: 'preprod' })])
    bindings.value = bs
    preprodServers.value = ps
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)

async function save(): Promise<void> {
  if (!form.value.vs_tag.trim() || !form.value.rs_env_tag.trim() || !form.value.preprod_server_id) {
    ElMessage.warning(t('common.serverPlaceholder'))
    return
  }
  saving.value = true
  try {
    await lvsApi.saveBinding({
      vs_tag: form.value.vs_tag.trim(),
      rs_env_tag: form.value.rs_env_tag.trim(),
      preprod_server_id: form.value.preprod_server_id,
    })
    ElMessage.success(t('common.execSuccess'))
    form.value = { vs_tag: '', rs_env_tag: '', preprod_server_id: undefined }
    await loadAll()
    emit('changed')
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    saving.value = false
  }
}

async function remove(row: LvsPreprodBinding): Promise<void> {
  await ElMessageBox.confirm(`${row.vs_tag} ↔ ${row.rs_env_tag}`, t('common.confirmDelete', { count: 1 }), {
    type: 'warning',
  })
  await lvsApi.deleteBinding(row.id)
  ElMessage.success(t('common.execSuccess'))
  await loadAll()
  emit('changed')
}
</script>

<template>
  <el-dialog v-model="visible" :title="t('lvs.bindings')" width="640px" append-to-body>
    <el-table :data="bindings" v-loading="loading" size="small" max-height="280">
      <el-table-column prop="vs_tag" :label="t('lvs.vsTag')" min-width="110" />
      <el-table-column prop="rs_env_tag" :label="t('lvs.rsTag')" min-width="110" />
      <el-table-column label="Preprod" min-width="140">
        <template #default="{ row }">
          {{ preprodServers.find((s) => s.id === row.preprod_server_id)?.name || row.preprod_server_id }}
        </template>
      </el-table-column>
      <el-table-column :label="t('common.operation')" width="80" fixed="right">
        <template #default="{ row }">
          <el-button link type="danger" size="small" @click="remove(row as LvsPreprodBinding)">{{ t('common.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-divider content-position="left">{{ t('common.add') }}</el-divider>

    <div class="binding-form">
      <el-input v-model="form.vs_tag" :placeholder="t('lvs.vsTag')" />
      <el-input v-model="form.rs_env_tag" :placeholder="t('lvs.rsTag')" />
      <el-select v-model="form.preprod_server_id" :placeholder="t('lvs.preprodServer')" filterable>
        <el-option v-for="s in preprodServers" :key="s.id" :value="s.id" :label="s.name" />
      </el-select>
      <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
    </div>
  </el-dialog>
</template>

<style scoped>
.binding-form {
  display: grid;
  grid-template-columns: 1fr 1fr 1.4fr auto;
  gap: var(--space-2);
}

@media (max-width: 700px) {
  .binding-form {
    grid-template-columns: 1fr;
  }
}
</style>
