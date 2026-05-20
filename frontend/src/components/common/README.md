# 通用组件

本目录包含基于 Composition API、TypeScript 和 TailwindCSS 构建的可复用 Vue 3 组件。

## 组件

### DataTable.vue

通用数据表格组件，支持排序、加载状态和自定义单元格渲染。

**Props：**

- `columns: Column[]`：列定义数组，包含 key、label、sortable 和 formatter。
- `data: any[]`：要展示的数据对象数组。
- `loading?: boolean`：是否显示加载骨架。
- `defaultSortKey?: string`：默认排序 key，仅在没有持久化排序状态时使用。
- `defaultSortOrder?: 'asc' | 'desc'`：默认排序方向，默认 `asc`。
- `sortStorageKey?: string`：将排序状态（key + order）持久化到 `localStorage`。
- `rowKey?: string | (row: any) => string | number`：行 key 字段或解析函数，默认 `row.id`，否则回退到索引。

**Slots：**

- `empty`：自定义空状态内容。
- `cell-{key}`：指定列的自定义单元格渲染器，接收 `row` 和 `value`。

**用法：**

```vue
<DataTable
  :columns="[
    { key: 'name', label: 'Name', sortable: true },
    { key: 'email', label: 'Email' },
    { key: 'status', label: 'Status', formatter: (val) => val.toUpperCase() }
  ]"
  :data="users"
  :loading="isLoading"
>
  <template #cell-actions="{ row }">
    <button @click="editUser(row)">Edit</button>
  </template>
</DataTable>
```

---

### Pagination.vue

分页组件，包含页码、导航和 page size 选择器。

**Props：**

- `total: number`：总条目数。
- `page: number`：当前页，从 1 开始。
- `pageSize: number`：每页条目数。
- `pageSizeOptions?: number[]`：可选 page size，默认 `[10, 20, 50, 100]`。

**Events：**

- `update:page`：页码变化时触发。
- `update:pageSize`：page size 变化时触发。

**用法：**

```vue
<Pagination
  :total="totalUsers"
  :page="currentPage"
  :pageSize="pageSize"
  @update:page="currentPage = $event"
  @update:pageSize="pageSize = $event"
/>
```

---

### Modal.vue

弹窗组件，支持自定义尺寸和关闭行为。

**Props：**

- `show: boolean`：控制弹窗是否显示。
- `title: string`：弹窗标题。
- `size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'`：弹窗尺寸，默认 `md`。
- `closeOnEscape?: boolean`：按 Escape 时关闭，默认 `true`。
- `closeOnClickOutside?: boolean`：点击遮罩时关闭，默认 `true`。

**Events：**

- `close`：弹窗需要关闭时触发。

**Slots：**

- `default`：弹窗主体内容。
- `footer`：弹窗底部内容。

**用法：**

```vue
<Modal :show="showModal" title="Edit User" size="lg" @close="showModal = false">
  <form @submit.prevent="saveUser">
    <!-- Form content -->
  </form>

  <template #footer>
    <button @click="showModal = false">Cancel</button>
    <button @click="saveUser">Save</button>
  </template>
</Modal>
```

---

### ConfirmDialog.vue

基于 Modal 组件构建的确认对话框。

**Props：**

- `show: boolean`：控制对话框是否显示。
- `title: string`：对话框标题。
- `message: string`：确认消息。
- `confirmText?: string`：确认按钮文字，默认 `Confirm`。
- `cancelText?: string`：取消按钮文字，默认 `Cancel`。
- `danger?: boolean`：是否使用危险/红色样式，默认 `false`。

**Events：**

- `confirm`：用户确认时触发。
- `cancel`：用户取消时触发。

**用法：**

```vue
<ConfirmDialog
  :show="showDeleteConfirm"
  title="Delete User"
  message="Are you sure you want to delete this user? This action cannot be undone."
  confirm-text="Delete"
  cancel-text="Cancel"
  danger
  @confirm="deleteUser"
  @cancel="showDeleteConfirm = false"
/>
```

---

### StatCard.vue

统计卡片组件，用于展示指标，并可显示变化趋势。

**Props：**

- `title: string`：卡片标题。
- `value: number | string`：主显示值。
- `icon?: Component`：图标组件。
- `change?: number`：百分比变化值。
- `changeType?: 'up' | 'down' | 'neutral'`：变化方向，默认 `neutral`。
- `formatValue?: (value) => string`：自定义值格式化函数。

**用法：**

```vue
<StatCard title="Total Users" :value="1234" :icon="UserIcon" :change="12.5" change-type="up" />
```

---

### Toast.vue

Toast 通知组件，会自动展示 app store 中的 toast。

**用法：**

```vue
<!-- Add once in App.vue or layout -->
<Toast />
```

```typescript
// Trigger toasts from anywhere using the app store
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

appStore.addToast({
  type: 'success',
  title: 'Success!',
  message: 'User created successfully',
  duration: 3000
})

appStore.addToast({
  type: 'error',
  message: 'Failed to delete user'
})
```

---

### LoadingSpinner.vue

简单的动画加载 spinner。

**Props：**

- `size?: 'sm' | 'md' | 'lg' | 'xl'`：spinner 尺寸，默认 `md`。
- `color?: 'primary' | 'secondary' | 'white' | 'gray'`：spinner 颜色，默认 `primary`。

**用法：**

```vue
<LoadingSpinner size="lg" color="primary" />
```

---

### EmptyState.vue

空状态占位组件，包含图标、消息和可选操作按钮。

**Props：**

- `icon?: Component`：图标组件。
- `title: string`：空状态标题。
- `description: string`：空状态描述。
- `actionText?: string`：操作按钮文字。
- `actionTo?: string | object`：Router link 目标。
- `actionIcon?: boolean`：是否在按钮中显示 plus 图标，默认 `true`。

**Slots：**

- `icon`：自定义图标内容。
- `action`：自定义操作按钮/链接。

**用法：**

```vue
<EmptyState
  title="No users found"
  description="Get started by creating your first user account."
  action-text="Add User"
  :action-to="{ name: 'users-create' }"
/>
```

## 导入

可以按名称导入组件：

```typescript
import { DataTable, Pagination, Modal } from '@/components/common'
```

也可以直接导入具体组件：

```typescript
import DataTable from '@/components/common/DataTable.vue'
```

## 特性

所有组件都包含：

- **TypeScript 支持**：提供合适的类型定义。
- **可访问性**：包含 ARIA 属性和键盘导航。
- **响应式设计**：适配移动端布局。
- **TailwindCSS 样式**：保持一致的设计语言。
- **Vue 3 Composition API**：使用 `<script setup>`。
- **Slot 支持**：方便自定义内容。
