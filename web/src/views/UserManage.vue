<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userApi, extractErrorMessage } from '@/api'
import type { User, BatchResult, LdapUser } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useTablePaging } from '@/composables/useTablePaging'
import { i18n } from '@/i18n'

const t = i18n.global.t
const auth = useAuthStore()

// ---------- 列表 ----------
const loading = ref(false)
const rows = ref<User[]>([])
const keyword = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    rows.value = await userApi.list()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    loading.value = false
  }
}

onMounted(load)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return rows.value
  return rows.value.filter(
    (u) =>
      u.username.toLowerCase().includes(kw) ||
      u.name.toLowerCase().includes(kw) ||
      u.email.toLowerCase().includes(kw),
  )
})

const { paged, currentPage, pageSize, total } = useTablePaging(filtered, 20)
const selected = ref<User[]>([])

// ---------- 新增 / 编辑 ----------
const editVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

const form = reactive({
  username: '',
  password: '',
  name: '',
  email: '',
  role: 'user' as 'admin' | 'user',
  enabled: true,
})

async function openCreate(): Promise<void> {
  editingId.value = null
  Object.assign(form, { username: '', password: '', name: '', email: '', role: 'user', enabled: true })
  editVisible.value = true
}

function openEdit(row: User): void {
  editingId.value = row.id
  Object.assign(form, {
    username: row.username,
    password: '',
    name: row.name,
    email: row.email,
    role: row.role,
    enabled: row.enabled,
  })
  editVisible.value = true
}

async function save(): Promise<void> {
  if (!form.username.trim() || !form.name.trim() || !form.email.trim()) {
    ElMessage.warning(t('users.username'))
    return
  }
  saving.value = true
  try {
    if (editingId.value === null) {
      await userApi.create({
        username: form.username.trim(),
        password: form.password,
        name: form.name.trim(),
        email: form.email.trim(),
        role: form.role,
      })
    } else {
      await userApi.update(editingId.value, {
        username: form.username.trim(),
        name: form.name.trim(),
        email: form.email.trim(),
        role: form.role,
        enabled: form.enabled,
      })
    }
    ElMessage.success(t('common.execSuccess'))
    editVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    saving.value = false
  }
}

// ---------- 启停 / 删除 / 批量 ----------
function isProtected(row: Pick<User, 'id' | 'username'>): boolean {
  return row.username === 'admin' || row.id === auth.user?.id
}

async function toggle(row: User): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  await userApi.toggle(row.id)
  ElMessage.success(t('common.execSuccess'))
  await load()
}

async function remove(row: User): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  await ElMessageBox.confirm(row.username, t('common.confirmDelete', { count: 1 }), { type: 'warning' })
  await userApi.delete(row.id)
  ElMessage.success(t('common.execSuccess'))
  await load()
}

function showBatchResult(res: BatchResult): void {
  ElMessage({ type: 'success', message: res.message, duration: 5000 })
}

async function batchDelete(): Promise<void> {
  const ids = selected.value.filter((u) => !isProtected(u)).map((u) => u.id)
  if (ids.length === 0) return
  await ElMessageBox.confirm(t('common.confirmDelete', { count: ids.length }), t('common.confirm'), {
    type: 'warning',
  })
  const res = await userApi.batchDelete(ids)
  showBatchResult(res)
  await load()
}

async function batchToggle(enabled: boolean): Promise<void> {
  const ids = selected.value.filter((u) => !isProtected(u)).map((u) => u.id)
  if (ids.length === 0) return
  const res = await userApi.batchToggle(ids, enabled)
  showBatchResult(res)
  await load()
}

// ---------- 重置密码 ----------
const resetVisible = ref(false)
const resetTarget = ref<User | null>(null)
const newPassword = ref('')
const resetting = ref(false)

function openReset(row: User): void {
  if (row.auth_source === 'ldap') {
    ElMessage.warning('LDAP 用户不支持重置密码')
    return
  }
  resetTarget.value = row
  newPassword.value = ''
  resetVisible.value = true
}

async function doReset(): Promise<void> {
  if (!resetTarget.value || !newPassword.value) return
  resetting.value = true
  try {
    await userApi.resetPassword(resetTarget.value.id, newPassword.value)
    ElMessage.success(t('common.execSuccess'))
    resetVisible.value = false
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    resetting.value = false
  }
}

// ---------- LDAP 导入 ----------
const ldapVisible = ref(false)
const ldapLoading = ref(false)
const importing = ref(false)
const ldapUsers = ref<(LdapUser & { _selected?: boolean })[]>([])
const ldapSelected = ref<LdapUser[]>([])

async function openLdap(): Promise<void> {
  ldapVisible.value = true
  ldapLoading.value = true
  try {
    ldapUsers.value = (await userApi.listLdap()).map((u) => ({ ...u, _selected: true }))
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    ldapLoading.value = false
  }
}

async function doImport(): Promise<void> {
  const users = ldapSelected.value
  if (users.length === 0) return
  importing.value = true
  try {
    const res = await userApi.importLdap(users)
    showBatchResult(res)
    ldapVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('users.title') }}</h1>
        <p class="page-subtitle">{{ t('users.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button @click="openLdap">{{ t('users.ldapImport') }}</el-button>
        <el-button type="primary" @click="openCreate">{{ t('common.add') }}</el-button>
      </div>
    </div>

    <div class="page-actions" style="margin-bottom: var(--space-4)">
      <el-input
        v-model="keyword"
        :placeholder="t('logs.keyword')"
        clearable
        style="width: 220px"
      />
      <el-button :disabled="selected.length === 0" @click="batchToggle(true)">
        {{ t('common.batchToggle') }}
      </el-button>
      <el-button type="danger" :disabled="selected.length === 0" @click="batchDelete">
        {{ t('common.batchDelete') }}
      </el-button>
    </div>

    <div v-loading="loading" class="card table-card reveal d-1">
      <el-table :data="paged" row-key="id" @selection-change="(r: User[]) => (selected = r)">
        <el-table-column type="selection" width="42" />
        <el-table-column prop="username" :label="t('users.username')" min-width="130" sortable>
          <template #default="{ row }">
            <span class="mono">{{ row.username }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('users.name')" min-width="120" />
        <el-table-column prop="email" :label="t('users.email')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('users.role')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.role === 'admin' ? 'danger' : 'info'" effect="plain">
              {{ row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="auth_source" :label="t('users.authSource')" width="90" />
        <el-table-column :label="t('common.enabled')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'" effect="plain">
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.operation')" width="290" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openEdit(row as User)">{{ t('common.edit') }}</el-button>
            <el-button link type="warning" size="small" :disabled="row.auth_source === 'ldap'" @click="openReset(row as User)">
              {{ t('users.resetPassword') }}
            </el-button>
            <el-button
              link
              :type="row.enabled ? 'warning' : 'success'"
              size="small"
              :disabled="isProtected(row as Pick<User, 'id' | 'username'>)"
              @click="toggle(row as User)"
            >
              {{ row.enabled ? t('common.disabled') : t('common.enabled') }}
            </el-button>
            <el-button link type="danger" size="small" :disabled="isProtected(row as Pick<User, 'id' | 'username'>)" @click="remove(row as User)">
              {{ t('common.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          layout="total, sizes, prev, pager, next"
          :total="total"
          :current-page="currentPage"
          :page-size="pageSize"
          :page-sizes="[20, 50, 100]"
          @current-change="(p: number) => (currentPage = p)"
          @size-change="(s: number) => (pageSize = s)"
        />
      </div>
    </div>

    <!-- 新增/编辑 -->
    <el-dialog
      v-model="editVisible"
      :title="editingId === null ? t('users.createTitle') : t('users.editTitle')"
      width="520px"
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item :label="t('users.username')" required>
          <el-input v-model="form.username" :disabled="editingId !== null" class="mono" />
        </el-form-item>
        <el-form-item v-if="editingId === null" :label="t('users.password')" required>
          <el-input v-model="form.password" type="password" show-password :placeholder="t('users.passwordStrength')" />
        </el-form-item>
        <el-form-item :label="t('users.name')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('users.email')" required>
          <el-input v-model="form.email" type="email" />
        </el-form-item>
        <el-form-item :label="t('users.role')">
          <el-radio-group v-model="form.role" :disabled="editingId !== null && isProtected({ id: editingId, username: form.username })">
            <el-radio-button value="user">user</el-radio-button>
            <el-radio-button value="admin">admin</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="editingId !== null" :label="t('common.enabled')">
          <el-switch v-model="form.enabled" :disabled="isProtected({ id: editingId, username: form.username })" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog v-model="resetVisible" :title="t('users.resetPassword')" width="440px" append-to-body>
      <el-input
        v-model="newPassword"
        type="password"
        show-password
        :placeholder="t('users.passwordStrength')"
      />
      <template #footer>
        <el-button @click="resetVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="resetting" @click="doReset">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- LDAP 导入 -->
    <el-dialog v-model="ldapVisible" :title="t('users.ldapImport')" width="640px" append-to-body>
      <p class="ldap-desc">{{ t('users.ldapImportDesc') }}</p>
      <el-table
        v-loading="ldapLoading"
        :data="ldapUsers"
        size="small"
        max-height="340"
        @selection-change="(r: LdapUser[]) => (ldapSelected = r)"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column prop="username" :label="t('users.username')" min-width="130" />
        <el-table-column prop="name" :label="t('users.name')" min-width="110" />
        <el-table-column prop="email" :label="t('users.email')" min-width="180" show-overflow-tooltip />
      </el-table>
      <template #footer>
        <el-button @click="ldapVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">{{ t('users.import') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.table-card {
  padding: var(--space-3);
}

.pager {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-3) var(--space-2) 0;
}

.ldap-desc {
  color: var(--text-secondary);
  font-size: var(--text-sm);
  margin-top: 0;
}
</style>
