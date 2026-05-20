# Vue Router 配置

## 概览

本目录包含 Sub2API 前端应用的 Vue Router 配置。路由实现了完整的导航系统，包括认证守卫、基于角色的访问控制和懒加载。

## 文件

- **index.ts**：主路由配置，包含路由定义和导航守卫。
- **meta.d.ts**：路由 meta 字段的 TypeScript 类型定义。

## 路由结构

### 公开路由（无需认证）

| Path | Component | 说明 |
|------|-----------|------|
| `/login` | LoginView | 用户登录页 |
| `/register` | RegisterView | 用户注册页 |

### 用户路由（需要认证）

| Path | Component | 说明 |
|------|-----------|------|
| `/` | - | 重定向到 `/dashboard` |
| `/dashboard` | DashboardView | 用户仪表盘和统计 |
| `/keys` | KeysView | API key 管理 |
| `/usage` | UsageView | 用量记录和统计 |
| `/redeem` | RedeemView | 兑换码界面 |
| `/profile` | ProfileView | 用户资料设置 |

### 管理员路由（需要管理员角色）

| Path | Component | 说明 |
|------|-----------|------|
| `/admin` | - | 重定向到 `/admin/dashboard` |
| `/admin/dashboard` | AdminDashboardView | 管理员仪表盘 |
| `/admin/users` | AdminUsersView | 用户管理 |
| `/admin/groups` | AdminGroupsView | 分组管理 |
| `/admin/accounts` | AdminAccountsView | 账号管理 |
| `/admin/proxies` | AdminProxiesView | 代理管理 |
| `/admin/redeem` | AdminRedeemView | 兑换码管理 |

### 特殊路由

| Path | Component | 说明 |
|------|-----------|------|
| `/:pathMatch(.*)` | NotFoundView | 404 错误页 |

## 导航守卫

### 认证守卫（beforeEach）

路由实现了完整的导航守卫：

1. **设置页面标题**：根据 route meta 更新 document title。
2. **检查认证状态**：
   - 公开路由（`requiresAuth: false`）无需登录即可访问。
   - 受保护路由需要认证。
   - 未认证时重定向到 `/login`。
3. **防止重复登录**：
   - 已认证用户访问登录/注册页时会被重定向走。
4. **基于角色的访问控制**：
   - 管理员路由（`requiresAdmin: true`）需要管理员角色。
   - 非管理员用户会被重定向到 `/dashboard`。
5. **保留原目标地址**：
   - 将原始 URL 保存到 query 参数，用于登录后跳转。

### 流程图

```text
User navigates to route
        ↓
Set page title from meta
        ↓
Is route public? ──Yes──→ Already authenticated? ──Yes──→ Redirect to /dashboard
        ↓ No                                        ↓ No
        ↓                                      Allow access
        ↓
Is user authenticated? ──No──→ Redirect to /login with redirect query
        ↓ Yes
        ↓
Requires admin role? ──Yes──→ Is user admin? ──No──→ Redirect to /dashboard
        ↓ No                                  ↓ Yes
        ↓                                     ↓
Allow access ←────────────────────────────────┘
```

## Route Meta 字段

每个路由都可以定义以下 meta 字段：

```typescript
interface RouteMeta {
  requiresAuth?: boolean // Default: true (requires authentication)
  requiresAdmin?: boolean // Default: false (admin access only)
  title?: string // Page title
  breadcrumbs?: Array<{
    // Breadcrumb navigation
    label: string
    to?: string
  }>
  icon?: string // Icon for navigation menu
  hideInMenu?: boolean // Hide from navigation menu
}
```

## 懒加载

所有路由组件都使用动态导入进行代码拆分：

```typescript
component: () => import('@/views/user/DashboardView.vue')
```

优点：

- 减小初始 bundle 体积。
- 加快首次页面加载。
- 组件按需加载。
- Vite 自动拆分代码。

## 认证 Store 集成

路由会与 Pinia auth store（`@/stores/auth`）集成：

```typescript
const authStore = useAuthStore()

// Check authentication status
authStore.isAuthenticated

// Check admin role
authStore.isAdmin
```

## 使用示例

### 编程式导航

```typescript
import { useRouter } from 'vue-router'

const router = useRouter()

// Navigate to a route
router.push('/dashboard')

// Navigate with query parameters
router.push({
  path: '/usage',
  query: { filter: 'today' }
})

// Navigate to admin route (will be blocked if not admin)
router.push('/admin/users')
```

### Route Links

```vue
<template>
  <!-- Simple link -->
  <router-link to="/dashboard">Dashboard</router-link>

  <!-- Named route -->
  <router-link :to="{ name: 'Keys' }">API Keys</router-link>

  <!-- With query parameters -->
  <router-link :to="{ path: '/usage', query: { page: 1 } }"> Usage </router-link>
</template>
```

### 检查当前路由

```typescript
import { useRoute } from 'vue-router'

const route = useRoute()

// Check if on admin page
const isAdminPage = route.path.startsWith('/admin')

// Get route meta
const requiresAdmin = route.meta.requiresAdmin
```

## 滚动行为

路由实现了自动滚动管理：

- **浏览器导航**：恢复已保存的滚动位置。
- **新路由**：滚动到页面顶部。
- **Hash 链接**：滚动到锚点（实现后）。

## 错误处理

路由包含导航失败的错误处理：

```typescript
router.onError((error) => {
  console.error('Router error:', error)
})
```

## 测试路由

测试导航守卫和路由访问：

1. **公开路由访问**：未登录时访问 `/login`。
2. **受保护路由**：未登录时尝试访问 `/dashboard`，应重定向。
3. **管理员访问**：普通用户登录后尝试访问 `/admin/users`，应重定向到 dashboard。
4. **管理员成功访问**：管理员登录后访问 `/admin/users`，应成功。
5. **404 处理**：访问不存在的路由，应显示 404 页面。

## 开发建议

### 添加新路由

1. 在 `routes` 数组中添加路由定义。
2. 创建对应 view 组件。
3. 设置合适的 meta 字段（`requiresAuth`、`requiresAdmin`）。
4. 使用 `() => import()` 做懒加载。
5. 更新本 README 中的路由文档。

### 调试导航

启用 Vue Router 调试：

```typescript
// In browser console
window.__VUE_ROUTER__ = router

// Check current route
router.currentRoute.value
```

### 常见问题

**问题**：刷新页面时 404

- **原因**：服务器没有按 SPA 配置。
- **解决**：将服务器配置为所有路由都返回 `index.html`。

**问题**：导航守卫执行两次

- **原因**：多次调用 `next()`。
- **解决**：确保每条代码路径只调用一次 `next()`。

**问题**：用户数据未加载

- **原因**：Auth store 未初始化。
- **解决**：在 `App.vue` 或 `main.ts` 中调用 `authStore.checkAuth()`。

## 安全注意事项

1. **仅客户端控制**：导航守卫只在客户端执行，服务端也必须校验。
2. **Token 校验**：API 每次请求都应验证 JWT token。
3. **角色校验**：后端必须校验管理员角色，不能只依赖前端。
4. **XSS 防护**：Vue 会自动转义模板内容。
5. **CSRF 防护**：状态变更操作应使用 CSRF token。

## 性能优化

1. **懒加载**：所有路由使用动态导入。
2. **代码拆分**：Vite 自动拆分路由 chunk。
3. **预取**：常用路径可以考虑添加 route prefetch。
4. **路由缓存**：Vue Router 会缓存组件实例。

## 后续增强

- [ ] 添加面包屑导航系统。
- [ ] 实现超出 admin/user 的路由权限。
- [ ] 添加路由过渡动画。
- [ ] 为预期导航实现 route prefetch。
- [ ] 添加导航分析追踪。
