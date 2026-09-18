<template>
  <div>
    <div class="toolbar">
      <div>
        <h2 class="page-title">出土文物</h2>
        <p class="page-sub">登记器物类型、材质、完整度与存放位置</p>
      </div>
      <button class="btn" @click="openCreate">新增文物</button>
    </div>

    <div class="card">
      <div class="filters">
        <label>
          探方筛选
          <select v-model="filterUnitId" @change="onFilterChange">
            <option value="">全部探方</option>
            <option v-for="u in units" :key="u.id" :value="String(u.id)">
              {{ u.site?.name || '' }} / {{ u.code }}
            </option>
          </select>
        </label>
        <label>
          器物类型
          <select v-model="filterType" @change="onFilterChange">
            <option value="">全部类型</option>
            <option v-for="t in artifactTypes" :key="t" :value="t">{{ t }}</option>
          </select>
        </label>
      </div>

      <div v-if="selectedIds.size" class="batch-bar">
        <span class="batch-count">已选 {{ selectedIds.size }} 件</span>
        <input
          v-model="batchStorageLoc"
          class="batch-input"
          placeholder="输入新的存放位置，如：库房A-架02"
          @keyup.enter="submitBatch"
        />
        <button class="btn" :disabled="batchLoading" @click="submitBatch">
          {{ batchLoading ? '提交中…' : '批量修改存放位置' }}
        </button>
        <button class="btn secondary small" :disabled="batchLoading" @click="clearSelection">
          取消选择
        </button>
        <span v-if="batchError" class="error">{{ batchError }}</span>
      </div>

      <table class="table">
        <thead>
          <tr>
            <th class="col-check">
              <input
                ref="headerCheck"
                type="checkbox"
                :checked="allOnPageSelected"
                @change="togglePage"
              />
            </th>
            <th>登记号</th>
            <th>探方</th>
            <th>器物类型</th>
            <th>材质</th>
            <th>完整度</th>
            <th>出土日期</th>
            <th>存放位置</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in list" :key="item.id">
            <td class="col-check">
              <input
                type="checkbox"
                :checked="selectedIds.has(item.id)"
                @change="toggleOne(item.id)"
              />
            </td>
            <td>{{ item.registerNo }}</td>
            <td>{{ item.unit?.code || '-' }}</td>
            <td><span class="tag">{{ item.artifactType }}</span></td>
            <td>{{ item.materialName || item.material?.name || '-' }}</td>
            <td>{{ item.completeness || '-' }}</td>
            <td>{{ formatDate(item.findDate) }}</td>
            <td>{{ item.storageLoc || '-' }}</td>
            <td>
              <button class="btn secondary small" @click="openEdit(item)">编辑</button>
              <button class="btn danger small" @click="remove(item)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!list.length" class="page-sub">暂无数据</p>
      <p v-if="error" class="error">{{ error }}</p>

      <div class="pagination">
        <span class="page-sub">共 {{ total }} 条，第 {{ page }} / {{ totalPages }} 页</span>
        <div class="page-controls">
          <label class="page-size">
            每页
            <select v-model.number="pageSize" @change="changePageSize">
              <option :value="10">10</option>
              <option :value="20">20</option>
              <option :value="50">50</option>
            </select>
          </label>
          <button class="btn secondary small" :disabled="page <= 1" @click="goPage(page - 1)">
            上一页
          </button>
          <button
            class="btn secondary small"
            :disabled="page >= totalPages"
            @click="goPage(page + 1)"
          >
            下一页
          </button>
        </div>
      </div>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ form.id ? '编辑文物' : '新增文物' }}</h3>
        <div class="form-grid">
          <label>
            所属探方
            <select v-model.number="form.unitId">
              <option :value="0" disabled>请选择</option>
              <option v-for="u in units" :key="u.id" :value="u.id">
                {{ u.site?.name || '' }} / {{ u.code }}
              </option>
            </select>
          </label>
          <label>
            登记号
            <input v-model="form.registerNo" />
          </label>
          <label>
            器物类型
            <select v-model="form.artifactType">
              <option v-for="t in artifactTypes" :key="t" :value="t">{{ t }}</option>
            </select>
          </label>
          <label>
            材质
            <select v-model="form.materialId">
              <option :value="null">未指定</option>
              <option v-for="m in materials" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select>
          </label>
          <label>
            完整度
            <select v-model="form.completeness">
              <option>完整</option>
              <option>残缺</option>
              <option>碎片</option>
            </select>
          </label>
          <label>
            出土日期
            <input v-model="form.findDate" type="date" />
          </label>
          <label class="full">
            存放位置
            <input v-model="form.storageLoc" />
          </label>
          <label class="full">
            描述
            <textarea v-model="form.description" />
          </label>
        </div>
        <p v-if="formError" class="error">{{ formError }}</p>
        <div class="modal-actions">
          <button class="btn secondary" @click="showModal = false">取消</button>
          <button class="btn" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import api from '../api/http'

const artifactTypes = ['陶片', '青铜器', '骨器', '玉器', '石器', '铁器', '其他']
const list = ref([])
const units = ref([])
const materials = ref([])
const filterUnitId = ref('')
const filterType = ref('')
const error = ref('')
const formError = ref('')
const showModal = ref(false)

// 服务端分页状态
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// 跨页保留的多选集合（存放文物 id）
const selectedIds = reactive(new Set())
const batchStorageLoc = ref('')
const batchError = ref('')
const batchLoading = ref(false)
const headerCheck = ref(null)

const allOnPageSelected = computed(
  () => list.value.length > 0 && list.value.every((item) => selectedIds.has(item.id))
)

// 表头复选框的半选状态（部分选中时）
watch(
  [() => list.value.map((i) => i.id), selectedIds],
  async () => {
    await nextTick()
    const el = headerCheck.value
    if (!el) return
    const some = list.value.some((item) => selectedIds.has(item.id))
    el.indeterminate = some && !allOnPageSelected.value
  },
  { deep: true, immediate: true }
)

const form = reactive({
  id: null,
  unitId: 0,
  materialId: null,
  registerNo: '',
  artifactType: '陶片',
  completeness: '完整',
  findDate: '',
  description: '',
  storageLoc: ''
})

function formatDate(v) {
  if (!v) return '-'
  return String(v).slice(0, 10)
}

async function loadMeta() {
  const [u, m] = await Promise.all([api.get('/units'), api.get('/materials')])
  units.value = u.data
  materials.value = m.data
}

async function load() {
  error.value = ''
  try {
    const params = { page: page.value, pageSize: pageSize.value }
    if (filterUnitId.value) params.unitId = filterUnitId.value
    if (filterType.value) params.artifactType = filterType.value
    const { data } = await api.get('/finds', { params })
    list.value = data.items
    total.value = data.total
    // 当前页因数据变化越界时，回退到最后一页重新拉取
    if (page.value > totalPages.value) {
      page.value = totalPages.value
      await load()
    }
  } catch (e) {
    error.value = e.response?.data?.error || '加载失败'
  }
}

function onFilterChange() {
  page.value = 1
  load()
}

function changePageSize() {
  page.value = 1
  load()
}

function goPage(p) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
  load()
}

function toggleOne(id) {
  batchError.value = ''
  if (selectedIds.has(id)) {
    selectedIds.delete(id)
  } else {
    selectedIds.add(id)
  }
}

function togglePage(e) {
  batchError.value = ''
  if (e.target.checked) {
    list.value.forEach((item) => selectedIds.add(item.id))
  } else {
    list.value.forEach((item) => selectedIds.delete(item.id))
  }
}

function clearSelection() {
  selectedIds.clear()
  batchStorageLoc.value = ''
  batchError.value = ''
}

async function submitBatch() {
  batchError.value = ''
  if (!selectedIds.size) {
    batchError.value = '请至少选择一件文物'
    return
  }
  if (!batchStorageLoc.value.trim()) {
    batchError.value = '存放位置不能为空'
    return
  }
  batchLoading.value = true
  try {
    await api.post('/finds/batch-storage', {
      findIds: [...selectedIds],
      storageLoc: batchStorageLoc.value.trim()
    })
    clearSelection()
    await load() // 以服务端返回为准刷新，不做本地内存假更新
  } catch (e) {
    batchError.value = e.response?.data?.error || '批量修改失败，请稍后重试'
  } finally {
    batchLoading.value = false
  }
}

function openCreate() {
  Object.assign(form, {
    id: null,
    unitId: units.value[0]?.id || 0,
    materialId: materials.value[0]?.id ?? null,
    registerNo: '',
    artifactType: '陶片',
    completeness: '完整',
    findDate: '',
    description: '',
    storageLoc: ''
  })
  formError.value = ''
  showModal.value = true
}

function openEdit(item) {
  Object.assign(form, {
    id: item.id,
    unitId: item.unitId,
    materialId: item.materialId,
    registerNo: item.registerNo,
    artifactType: item.artifactType,
    completeness: item.completeness || '完整',
    findDate: formatDate(item.findDate) === '-' ? '' : formatDate(item.findDate),
    description: item.description || '',
    storageLoc: item.storageLoc || ''
  })
  formError.value = ''
  showModal.value = true
}

async function save() {
  formError.value = ''
  try {
    const payload = {
      unitId: form.unitId,
      materialId: form.materialId || null,
      registerNo: form.registerNo,
      artifactType: form.artifactType,
      completeness: form.completeness,
      findDate: form.findDate || null,
      description: form.description,
      storageLoc: form.storageLoc
    }
    if (form.id) {
      await api.put(`/finds/${form.id}`, payload)
    } else {
      await api.post('/finds', payload)
    }
    showModal.value = false
    await load()
  } catch (e) {
    formError.value = e.response?.data?.error || '保存失败'
  }
}

async function remove(item) {
  if (!confirm(`确认删除文物「${item.registerNo}」？`)) return
  try {
    await api.delete(`/finds/${item.id}`)
    selectedIds.delete(item.id)
    await load()
  } catch (e) {
    alert(e.response?.data?.error || '删除失败')
  }
}

onMounted(async () => {
  await loadMeta()
  await load()
})
</script>

<style scoped>
.filters {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.filters label {
  min-width: 200px;
}

.col-check {
  width: 2.25rem;
  text-align: center;
}

.col-check input[type='checkbox'] {
  width: 1rem;
  height: 1rem;
  cursor: pointer;
}

.batch-bar {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-wrap: wrap;
  padding: 0.7rem 0.85rem;
  margin-bottom: 0.9rem;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #f7f0e4;
}

.batch-count {
  font-weight: 600;
  color: var(--accent);
  white-space: nowrap;
}

.batch-input {
  flex: 1 1 220px;
  min-width: 180px;
}

.btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
  margin-top: 0.9rem;
}

.page-controls {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.page-size {
  flex-direction: row;
  align-items: center;
  gap: 0.35rem;
  white-space: nowrap;
}

.page-size select {
  padding: 0.3rem 0.45rem;
}
</style>
