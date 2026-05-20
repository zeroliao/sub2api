# 认证视图

本目录包含 Sub2API 前端应用的 Vue 3 认证相关视图。

## 组件

### LoginView.vue

供已有用户认证登录的页面。

**功能：**

- 用户名和密码输入，并带校验。
- Remember me 复选框，用于持久化会话。
- 表单校验和实时错误展示。
- 认证期间显示加载状态。
- 登录失败时展示错误消息。
- 登录成功后跳转到 dashboard。
- 为新用户提供注册链接。

**用法：**

```vue
<template>
  <LoginView />
</template>

<script setup lang="ts">
import { LoginView } from '@/views/auth'
</script>
```

**Route：**

- Path：`/login`
- Name：`Login`
- Meta：`{ requiresAuth: false }`

**校验规则：**

- Username：必填，至少 3 个字符。
- Password：必填，至少 6 个字符。

**行为：**

- 使用 credentials 调用 `authStore.login()`。
- 登录成功时显示 success toast。
- 失败时显示 error toast 和行内错误消息。
- 跳转到 `/dashboard`，或 query 参数中的原目标路由。
- 已认证用户访问登录页时会被重定向走。

### RegisterView.vue

供新用户创建账号的注册页面。

**功能：**

- 用户名、邮箱、密码和确认密码输入。
- 完整表单校验。
- 密码强度要求（8+ 字符，包含字母和数字）。
- 使用 regex 校验邮箱格式。
- 校验两次密码一致。
- 注册期间显示加载状态。
- 注册失败时展示错误消息。
- 成功后跳转到 dashboard。
- 为已有用户提供登录链接。

**用法：**

```vue
<template>
  <RegisterView />
</template>

<script setup lang="ts">
import { RegisterView } from '@/views/auth'
</script>
```

**Route：**

- Path：`/register`
- Name：`Register`
- Meta：`{ requiresAuth: false }`

**校验规则：**

- Username：必填，3-50 个字符，只允许字母、数字、下划线和连字符。
- Email：必填，符合邮箱格式（RFC 5322 regex）。
- Password：必填，至少 8 个字符，且必须包含至少一个字母和一个数字。
- Confirm Password：必填，必须与 password 一致。

**行为：**

- 使用 user data 调用 `authStore.register()`。
- 注册成功时显示 success toast。
- 失败时显示 error toast 和行内错误消息。
- 注册成功后跳转到 `/dashboard`。
- 已认证用户访问注册页时会被重定向走。

## 架构

### 组件结构

两个视图都遵循一致结构：

```text
<template>
  <AuthLayout>
    <div class="space-y-6">
      <!-- Title -->
      <!-- Form -->
      <!-- Error Message -->
      <!-- Submit Button -->
    </div>

    <template #footer>
      <!-- Footer Links -->
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
// Imports
// State
// Validation
// Form Handlers
</script>
```

### 状态管理

两个视图都会使用：

- `useAuthStore()`：执行认证动作，例如 login、register。
- `useAppStore()`：显示 toast 通知和 UI 反馈。
- `useRouter()`：导航和重定向。

### 校验策略

**客户端校验：**

- 提交表单时实时校验。
- 字段级错误消息。
- 完整校验规则。
- TypeScript 类型安全。

**服务端校验：**

- 后端 API 校验所有输入。
- 优雅处理错误响应。
- 显示用户友好的错误消息。

### 样式

**设计系统：**

- TailwindCSS utility classes。
- 一致的配色方案（indigo primary）。
- 响应式设计。
- 可访问的表单控件。
- 带 spinner 动画的加载状态。

**视觉反馈：**

- 无效字段显示红色边框。
- 输入框下方显示错误消息。
- API 错误显示全局错误 banner。
- 完成后显示 success toast。
- 提交按钮显示 loading spinner。

## 依赖

### Components

- `AuthLayout`：来自 `@/components/layout` 的认证页面布局包装器。

### Stores

- `authStore`：来自 `@/stores/auth` 的认证状态管理。
- `appStore`：来自 `@/stores/app` 的应用状态和 toast。

### Libraries

- Vue 3 Composition API
- Vue Router，用于导航。
- Pinia，用于状态管理。
- TypeScript，用于类型安全。

## 使用示例

### 基础登录流程

```typescript
// User enters credentials
formData.username = 'john_doe'
formData.password = 'SecurePass123'

// Submit form
await handleLogin()

// On success:
// - authStore.login() called
// - Token and user stored
// - Success toast shown
// - Redirected to /dashboard

// On error:
// - Error message displayed
// - Error toast shown
// - Form remains editable
```

### 基础注册流程

```typescript
// User enters registration data
formData.username = 'jane_smith'
formData.email = 'jane@example.com'
formData.password = 'SecurePass123'
formData.confirmPassword = 'SecurePass123'

// Submit form
await handleRegister()

// On success:
// - authStore.register() called
// - Token and user stored
// - Success toast shown
// - Redirected to /dashboard

// On error:
// - Error message displayed
// - Error toast shown
// - Form remains editable
```

## 错误处理

### 客户端错误

```typescript
// Validation errors
errors.username = 'Username must be at least 3 characters'
errors.email = 'Please enter a valid email address'
errors.password = 'Password must be at least 8 characters with letters and numbers'
errors.confirmPassword = 'Passwords do not match'
```

### 服务端错误

```typescript
// API error responses
{
  response: {
    data: {
      detail: 'Username already exists'
    }
  }
}

// Displayed as:
errorMessage.value = 'Username already exists'
appStore.showError('Username already exists')
```

## 可访问性

- 使用语义化 HTML 元素（`<label>`、`<input>`、`<button>`）。
- label 设置正确的 `for` 属性。
- 加载状态使用 ARIA 属性。
- 支持键盘导航。
- 管理焦点。
- 提供错误提示。
- 保证足够的颜色对比度。

## 测试注意事项

### 单元测试

- 表单校验逻辑。
- 错误处理。
- 状态管理。
- 路由导航。

### 集成测试

- 完整登录流程。
- 完整注册流程。
- 错误场景。
- 重定向行为。

### E2E 测试

- 完整用户旅程。
- 表单交互。
- API 集成。
- 成功/错误状态。

## 后续增强

可考虑的改进：

- OAuth/SSO 集成（Google、GitHub）。
- Two-factor authentication（2FA）。
- 密码强度指示器。
- 邮箱验证流程。
- 忘记密码功能。
- 社交登录按钮。
- CAPTCHA 集成。
- 会话超时提醒。
- 密码可见性切换。
- Autofill 支持增强。

## 安全注意事项

- 密码永远不记录、不展示。
- 生产环境必须使用 HTTPS。
- JWT token 安全存储在 `localStorage`。
- API 启用 CORS 防护。
- Vue 自动转义模板内容以防 XSS。
- 使用基于 token 的认证做 CSRF 防护。
- 后端 API 做 rate limiting。
- 输入清理。
- 安全密码要求。

## 性能

- 路由懒加载。
- 最小化 bundle 体积。
- 快速首次渲染。
- 使用 reactive refs 优化重新渲染。
- 避免不必要的 watchers。
- 高效表单校验。

## 浏览器支持

- 现代浏览器（Chrome、Firefox、Safari、Edge）。
- 需要 ES2015+。
- Flexbox 和 CSS Grid。
- Tailwind CSS utilities。
- Vue 3 runtime。

## 相关文档

- [Auth Store Documentation](/src/stores/README.md#auth-store)
- [AuthLayout Component](/src/components/layout/README.md#authlayout)
- [Router Configuration](/src/router/index.ts)
- [API Documentation](/src/api/README.md#authentication)
- [Type Definitions](/src/types/index.ts)
