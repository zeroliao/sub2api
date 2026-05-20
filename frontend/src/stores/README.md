# Pinia Stores 文档

本目录包含 Sub2API 前端应用的所有 Pinia store。

## Store 概览

### 1. Auth Store（`auth.ts`）

管理用户认证状态、登录/退出登录和 token 持久化。

**State：**

- `user: User | null`：当前已认证用户。
- `token: string | null`：JWT 认证 token。

**Computed：**

- `isAuthenticated: boolean`：用户当前是否已认证。

**Actions：**

- `login(credentials)`：使用用户名/密码认证用户。
- `register(userData)`：注册新用户账号。
- `logout()`：清理认证状态并退出登录。
- `checkAuth()`：从 `localStorage` 恢复会话。
- `refreshUser()`：从服务端获取最新用户数据。

### 2. App Store（`app.ts`）

管理全局 UI 状态，包括侧边栏、加载状态和 toast 通知。

**State：**

- `sidebarCollapsed: boolean`：侧边栏折叠状态。
- `loading: boolean`：全局加载状态。
- `toasts: Toast[]`：当前活跃的 toast 通知。

**Computed：**

- `hasActiveToasts: boolean`：是否存在活跃 toast。

**Actions：**

- `toggleSidebar()`：切换侧边栏状态。
- `setSidebarCollapsed(collapsed)`：显式设置侧边栏状态。
- `setLoading(isLoading)`：设置加载状态。
- `showToast(type, message, duration?)`：显示 toast 通知。
- `showSuccess(message, duration?)`：显示成功 toast。
- `showError(message, duration?)`：显示错误 toast。
- `showInfo(message, duration?)`：显示信息 toast。
- `showWarning(message, duration?)`：显示警告 toast。
- `hideToast(id)`：隐藏指定 toast。
- `clearAllToasts()`：清除所有 toast。
- `withLoading(operation)`：在加载状态中执行异步操作。
- `withLoadingAndError(operation, errorMessage?)`：执行带加载和错误处理的异步操作。
- `reset()`：重置 store 到默认状态。

## 使用示例

### Auth Store

```typescript
import { useAuthStore } from '@/stores'

// In component setup
const authStore = useAuthStore()

// Initialize on app startup
authStore.checkAuth()

// Login
try {
  await authStore.login({ username: 'user', password: 'pass' })
  console.log('Logged in:', authStore.user)
} catch (error) {
  console.error('Login failed:', error)
}

// Check authentication
if (authStore.isAuthenticated) {
  console.log('User is logged in:', authStore.user?.username)
}

// Logout
authStore.logout()
```

### App Store

```typescript
import { useAppStore } from '@/stores'

// In component setup
const appStore = useAppStore()

// Sidebar control
appStore.toggleSidebar()
appStore.setSidebarCollapsed(true)

// Loading state
appStore.setLoading(true)
// ... do work
appStore.setLoading(false)

// Or use helper
await appStore.withLoading(async () => {
  const data = await fetchData()
  return data
})

// Toast notifications
appStore.showSuccess('Operation completed!')
appStore.showError('Something went wrong!', 5000)
appStore.showInfo('FYI: This is informational')
appStore.showWarning('Be careful!')

// Custom toast
const toastId = appStore.showToast('info', 'Custom message', undefined) // No auto-dismiss
// Later...
appStore.hideToast(toastId)
```

### 在 Vue 组件中组合使用

```vue
<script setup lang="ts">
import { useAuthStore, useAppStore } from '@/stores'
import { onMounted } from 'vue'

const authStore = useAuthStore()
const appStore = useAppStore()

onMounted(() => {
  // Check for existing session
  authStore.checkAuth()
})

async function handleLogin(username: string, password: string) {
  try {
    await appStore.withLoading(async () => {
      await authStore.login({ username, password })
    })
    appStore.showSuccess('Welcome back!')
  } catch (error) {
    appStore.showError('Login failed. Please check your credentials.')
  }
}

async function handleLogout() {
  authStore.logout()
  appStore.showInfo('You have been logged out.')
}
</script>

<template>
  <div>
    <button @click="appStore.toggleSidebar">Toggle Sidebar</button>

    <div v-if="appStore.loading">Loading...</div>

    <div v-if="authStore.isAuthenticated">
      Welcome, {{ authStore.user?.username }}!
      <button @click="handleLogout">Logout</button>
    </div>
    <div v-else>
      <button @click="handleLogin('user', 'pass')">Login</button>
    </div>
  </div>
</template>
```

## 持久化

- **Auth Store**：Token 和用户数据会自动持久化到 `localStorage`。
- Keys：`auth_token`、`auth_user`。
- 调用 `checkAuth()` 时恢复。
- **App Store**：不做持久化，页面刷新后 UI 状态会重置。

## TypeScript 支持

所有 store 都有完整 TypeScript 类型。从 `@/types` 导入类型：

```typescript
import type { User, Toast, ToastType } from '@/types'
```

## 测试

Store 可以重置为初始状态：

```typescript
// Auth store
authStore.logout() // Clears all auth state

// App store
appStore.reset() // Resets to defaults
```
