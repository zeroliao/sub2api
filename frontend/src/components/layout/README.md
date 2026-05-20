# 布局组件

Sub2API 前端的 Vue 3 布局组件，基于 Composition API、TypeScript 和 TailwindCSS 构建。

## 组件

### 1. AppLayout.vue

主应用布局，包含侧边栏和顶部 header。

**用法：**

```vue
<template>
  <AppLayout>
    <!-- Your page content here -->
    <h1>Dashboard</h1>
    <p>Welcome to your dashboard!</p>
  </AppLayout>
</template>

<script setup lang="ts">
import { AppLayout } from '@/components/layout'
</script>
```

**功能：**

- 响应式侧边栏，可折叠。
- 顶部固定 header。
- 通过 slot 提供主内容区域。
- 根据侧边栏状态自动调整 margin。

---

### 2. AppSidebar.vue

导航侧边栏，包含用户区和管理员区。

**功能：**

- 顶部 Logo/品牌区域。
- 用户导航链接：
  - Dashboard
  - API Keys
  - Usage
  - Redeem
  - Profile
- 管理员导航链接（仅管理员可见）：
  - Admin Dashboard
  - Users
  - Groups
  - Accounts
  - Proxies
  - Redeem Codes
- 可通过按钮折叠侧边栏。
- 高亮当前路由。
- 使用 HTML entity 图标。
- 响应式，适配移动端。

**由 AppLayout 自动使用**，通常不需要单独导入。

---

### 3. AppHeader.vue

顶部 header，包含用户信息和操作入口。

**功能：**

- 移动端菜单切换按钮。
- 页面标题，来源可以是 route meta 或 slot。
- 用户余额展示（仅桌面端）。
- 用户下拉菜单：
  - Profile 链接
  - Logout 按钮
- 使用用户名首字母生成头像。
- 下拉菜单支持点击外部关闭。
- 响应式设计。

**自定义标题用法：**

```vue
<template>
  <AppLayout>
    <template #title> Custom Page Title </template>

    <!-- Your content -->
  </AppLayout>
</template>
```

**由 AppLayout 自动使用**，通常不需要单独导入。

---

### 4. AuthLayout.vue

用于认证页面（登录/注册）的居中布局。

**用法：**

```vue
<template>
  <AuthLayout>
    <!-- Login/Register form content -->
    <h2 class="mb-6 text-2xl font-bold">Login</h2>

    <form @submit.prevent="handleLogin">
      <!-- Form fields -->
    </form>

    <!-- Optional footer slot -->
    <template #footer>
      <p>
        Don't have an account?
        <router-link to="/register" class="text-indigo-600 hover:underline"> Sign up </router-link>
      </p>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { AuthLayout } from '@/components/layout'

function handleLogin() {
  // Login logic
}
</script>
```

**功能：**

- 居中的卡片容器。
- 渐变背景。
- 顶部 Logo/品牌区域。
- 主内容 slot。
- 可选 footer slot，用于链接。
- 完整响应式。

---

## 路由配置

要在 header 中设置页面标题，请在路由中添加 meta：

```typescript
// router/index.ts
const routes = [
  {
    path: '/dashboard',
    component: DashboardView,
    meta: { title: 'Dashboard' }
  },
  {
    path: '/api-keys',
    component: ApiKeysView,
    meta: { title: 'API Keys' }
  }
  // ...
]
```

---

## Store 依赖

这些组件使用以下 Pinia store：

- **useAuthStore**：用于用户认证状态、角色检查和 logout。
- **useAppStore**：用于侧边栏状态管理和 toast 通知。

请确保应用中已经正确初始化这些 store。

---

## 样式

所有组件都使用 TailwindCSS utility classes。确保 `tailwind.config.js` 包含组件路径：

```js
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}']
  // ...
}
```

---

## 图标

组件为简单起见使用 HTML entity 图标：

- &#128200; Chart（Dashboard）
- &#128273; Key（API Keys）
- &#128202; Bar Chart（Usage）
- &#127873; Gift（Redeem）
- &#128100; User（Profile）
- &#128268; Admin
- &#128101; Users
- &#128193; Folder（Groups）
- &#127760; Globe（Accounts）
- &#128260; Network（Proxies）
- &#127991; Ticket（Redeem Codes）

如有需要，可以替换为偏好的图标库，例如 Heroicons 或 Font Awesome。

---

## 移动端响应式

所有组件都完整响应式：

- **AppSidebar**：桌面端固定定位，移动端默认隐藏。
- **AppHeader**：小屏幕显示移动端菜单按钮，隐藏余额展示。
- **AuthLayout**：根据移动设备调整 padding 和卡片尺寸。

侧边栏使用 Tailwind 响应式断点（`md:`）调整行为。
