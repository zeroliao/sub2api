# 认证视图视觉指南

本文档描述认证视图的视觉设计和布局。

## 布局结构

LoginView 和 RegisterView 都使用 AuthLayout 组件，布局如下：

```
┌─────────────────────────────────────────────┐
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │                                     │   │
│  │         Sub2API Logo                │   │
│  │  "Subscription to API Conversion"   │   │
│  │                                     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │                                     │   │
│  │    [Form Content - White Card]     │   │
│  │                                     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│         [Footer Links]                      │
│                                             │
└─────────────────────────────────────────────┘

Background: Gradient (Indigo → White → Purple)
Card: White with rounded corners and shadow
Max Width: 28rem (448px)
Centered: Both horizontally and vertically
```

## LoginView 视觉设计

### 默认状态

```
┌─────────────────────────────────────────────┐
│                                             │
│         🔷 Sub2API                          │
│    Subscription to API Conversion Platform  │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │                                     │   │
│  │        Welcome Back                 │   │
│  │  Sign in to your account to continue│  │
│  │                                     │   │
│  │  Username                           │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ Enter your username          │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  Password                           │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ ••••••••••••••                 │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  ☐ Remember me                      │   │
│  │                                     │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │       Sign In                  │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│    Don't have an account? Sign up          │
│                                             │
└─────────────────────────────────────────────┘
```

### 加载状态

```
┌─────────────────────────────────────────────┐
│  ┌────────────────────────────────┐         │
│  │  Username                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ john_doe                 │  │         │
│  │  └──────────────────────────┘  │         │
│  │                                │         │
│  │  Password                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ ••••••••••••            │  │         │
│  │  └──────────────────────────┘  │         │
│  │                                │         │
│  │  ☑ Remember me                 │         │
│  │                                │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ ⟳ Signing in...         │  │ ← Spinner
│  │  └──────────────────────────┘  │         │
│  │      (Button disabled)         │         │
│  └────────────────────────────────┘         │
└─────────────────────────────────────────────┘
```

### 错误状态

```
┌─────────────────────────────────────────────┐
│  ┌────────────────────────────────┐         │
│  │  Username                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ jo                       │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Username must be at least 3 │ ← Red text
│  │     characters                 │         │
│  │                                │         │
│  │  Password                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │                          │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Password is required        │ ← Red text
│  │                                │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ ⚠ Invalid username or    │  │ ← Error banner
│  │  │   password. Please try   │  │
│  │  │   again.                 │  │
│  │  └──────────────────────────┘  │         │
│  │                                │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │       Sign In            │  │         │
│  │  └──────────────────────────┘  │         │
│  └────────────────────────────────┘         │
└─────────────────────────────────────────────┘
```

## RegisterView 视觉设计

### 默认状态

```
┌─────────────────────────────────────────────┐
│                                             │
│         🔷 Sub2API                          │
│    Subscription to API Conversion Platform  │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │                                     │   │
│  │        Create Account               │   │
│  │     Sign up to start using Sub2API  │   │
│  │                                     │   │
│  │  Username                           │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ Choose a username            │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  Email                              │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ your.email@example.com       │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  Password                           │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ Create a strong password     │ │   │
│  │  └────────────────────────────────┘ │   │
│  │  At least 8 characters with letters │  │
│  │  and numbers                        │   │
│  │                                     │   │
│  │  Confirm Password                   │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │ Confirm your password        │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  ┌────────────────────────────────┐ │   │
│  │  │     Create Account             │ │   │
│  │  └────────────────────────────────┘ │   │
│  │                                     │   │
│  │  By signing up, you agree to our   │   │
│  │  Terms of Service and Privacy Policy│  │
│  │                                     │   │
│  └─────────────────────────────────────┘   │
│                                             │
│   Already have an account? Sign in         │
│                                             │
└─────────────────────────────────────────────┘
```

### 校验错误

```
┌─────────────────────────────────────────────┐
│  ┌────────────────────────────────┐         │
│  │  Username                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ jane@smith               │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Username can only contain   │ ← Red text
│  │     letters, numbers, _, and - │         │
│  │                                │         │
│  │  Email                         │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ invalid-email            │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Please enter a valid email  │ ← Red text
│  │     address                    │         │
│  │                                │         │
│  │  Password                      │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ short                    │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Password must be at least 8 │ ← Red text
│  │     characters with letters    │         │
│  │     and numbers                │         │
│  │                                │         │
│  │  Confirm Password              │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ different                │  │ ← Red border
│  │  └──────────────────────────┘  │         │
│  │  ⚠ Passwords do not match      │ ← Red text
│  │                                │         │
│  └────────────────────────────────┘         │
└─────────────────────────────────────────────┘
```

### 加载状态

```
┌─────────────────────────────────────────────┐
│  ┌────────────────────────────────┐         │
│  │  Username: jane_smith          │         │
│  │  Email: jane@example.com       │         │
│  │  Password: ••••••••••••        │         │
│  │  Confirm: ••••••••••••         │         │
│  │                                │         │
│  │  ┌──────────────────────────┐  │         │
│  │  │ ⟳ Creating account...   │  │ ← Spinner
│  │  └──────────────────────────┘  │         │
│  │      (All inputs disabled)     │         │
│  └────────────────────────────────┘         │
└─────────────────────────────────────────────┘
```

## 色板

### 主色

- **Indigo-600**: `#4F46E5` - Primary buttons, links, brand color
- **Indigo-700**: `#4338CA` - Button hover state
- **Indigo-500**: `#6366F1` - Focus ring

### 中性色

- **Gray-900**: `#111827` - Headings
- **Gray-700**: `#374151` - Labels
- **Gray-600**: `#4B5563` - Body text
- **Gray-500**: `#6B7280` - Helper text
- **Gray-300**: `#D1D5DB` - Borders
- **Gray-100**: `#F3F4F6` - Disabled backgrounds
- **White**: `#FFFFFF` - Card backgrounds

### 错误色

- **Red-600**: `#DC2626` - Error text
- **Red-500**: `#EF4444` - Error border, focus ring
- **Red-50**: `#FEF2F2` - Error banner background
- **Red-200**: `#FECACA` - Error banner border

### 成功色

- **Green-600**: `#16A34A` - Success text
- **Green-50**: `#F0FDF4` - Success banner background

### 背景渐变

- **From**: Indigo-100 (`#E0E7FF`)
- **Via**: White (`#FFFFFF`)
- **To**: Purple-100 (`#F3E8FF`)

## 字体排版

### 字体族

- **Default**: System font stack (`ui-sans-serif, system-ui, -apple-system, ...`)

### 字号

- **Headings (h2)**: `1.5rem` (24px), `font-bold`
- **Body**: `0.875rem` (14px), `font-normal`
- **Labels**: `0.875rem` (14px), `font-medium`
- **Helper text**: `0.75rem` (12px), `font-normal`
- **Error text**: `0.875rem` (14px), `font-normal`

### 行高

- **Headings**: `1.5`
- **Body**: `1.5`
- **Helper text**: `1.25`

## 间距

### 卡片间距

- **Padding**: `2rem` (32px) all sides
- **Gap between sections**: `1.5rem` (24px)
- **Gap between fields**: `1rem` (16px)

### 输入框间距

- **Padding**: `0.5rem 1rem` (8px 16px)
- **Label margin-bottom**: `0.25rem` (4px)
- **Error text margin-top**: `0.25rem` (4px)

### 按钮间距

- **Padding**: `0.5rem 1rem` (8px 16px)
- **Margin-top**: `1rem` (16px)

## 交互状态

### 输入框状态

**Default:**

```css
border: 1px solid #D1D5DB (gray-300)
focus: 2px ring #6366F1 (indigo-500)
```

**Error:**

```css
border: 1px solid #EF4444 (red-500)
focus: 2px ring #EF4444 (red-500)
```

**Disabled:**

```css
background: #F3F4F6 (gray-100)
cursor: not-allowed
opacity: 0.6
```

### 按钮状态

**Default:**

```css
background: #4F46E5 (indigo-600)
text: #FFFFFF (white)
shadow: shadow-sm
```

**Hover:**

```css
background: #4338CA (indigo-700)
transition: colors 150ms
```

**Focus:**

```css
outline: none
ring: 2px offset-2 #6366F1 (indigo-500)
```

**Disabled:**

```css
opacity: 0.5
cursor: not-allowed
```

**Loading:**

```css
opacity: 0.5
cursor: not-allowed
+ spinning icon
```

### 链接状态

**Default:**

```css
color: #4F46E5 (indigo-600)
font-weight: 500 (medium)
```

**Hover:**

```css
color: #6366F1 (indigo-500)
transition: colors 150ms
```

## 响应式设计

### 断点

**Mobile (< 640px):**

```
- Full width container
- Padding: 1rem (16px)
- Smaller text sizes
```

**Tablet (640px - 768px):**

```
- Max width: 28rem (448px)
- Centered layout
- Standard spacing
```

**Desktop (> 768px):**

```
- Max width: 28rem (448px)
- Centered layout
- Standard spacing
```

### 移动端优化

1. Touch-friendly tap targets (44px minimum)
2. Proper keyboard handling on mobile
3. Prevent zoom on input focus
4. Responsive font sizes
5. Full-width inputs
6. Adequate spacing for thumbs

## 动画

### 过渡

- Color changes: `150ms ease-in-out`
- Opacity changes: `150ms ease-in-out`
- Transform: `150ms ease-in-out`

### 加载 Spinner

```css
@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
animation: spin 1s linear infinite;
```

### Toast 动画

- Enter: Slide in from right + fade in
- Exit: Slide out to right + fade out
- Duration: 300ms

## 可访问性特性

### 视觉指示

- Clear focus states (2px ring)
- Error states (red border + red text)
- Loading states (spinner + text)
- Success states (green toast)

### 颜色对比度

- Text on white: > 7:1 (AAA)
- Labels on white: > 4.5:1 (AA)
- Buttons: > 4.5:1 (AA)
- Error text: > 4.5:1 (AA)

### 交互元素

- Minimum size: 44x44px (mobile)
- Clear hover states
- Distinct disabled states
- Keyboard accessible

### 屏幕阅读器支持

- Proper labels on all inputs
- ARIA attributes where needed
- Error announcements
- Loading state announcements

## 图标

### 加载 Spinner

```svg
<svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
</svg>
```

### 错误图标

```svg
<svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
</svg>
```

## 浏览器兼容性

### 支持的浏览器

- Chrome/Edge: Latest 2 versions
- Firefox: Latest 2 versions
- Safari: Latest 2 versions
- Mobile Safari: iOS 14+
- Chrome Mobile: Latest 2 versions

### 使用的 CSS 特性

- Flexbox (full support)
- CSS Grid (full support)
- CSS Transitions (full support)
- CSS Custom Properties (full support)
- Gradient backgrounds (full support)

### 使用的 JavaScript 特性

- ES2015+ syntax
- Async/await
- Optional chaining
- Nullish coalescing
- Modules

## 打印样式

不适用于认证页面，用户通常不应打印登录表单。

## 深色模式考虑

**后续增强：**

- Dark mode toggle in user preferences
- System preference detection
- Persistent dark mode setting
- Adjusted color palette for dark backgrounds

```css
/* Example dark mode colors (not implemented yet) */
dark:bg-gray-900
dark:text-white
dark:border-gray-700
```

## 性能指标

### 目标指标

- First Contentful Paint (FCP): < 1s
- Largest Contentful Paint (LCP): < 2.5s
- Time to Interactive (TTI): < 3s
- Cumulative Layout Shift (CLS): < 0.1
- First Input Delay (FID): < 100ms

### 优化策略

- 懒加载非关键资源。
- 最小化初始 bundle 体积。
- 使用高效动画（transform、opacity）。
- 优化图片（logo、icons）。
- 预连接 API 域名。
- 缓存静态资源。

## 组件大小

### Bundle 影响

- LoginView.vue: ~4 KB (minified)
- RegisterView.vue: ~6 KB (minified)
- AuthLayout.vue: ~1 KB (minified)
- Total: ~11 KB (excluding dependencies)

### 依赖

- Vue 3: ~40 KB (runtime)
- Vue Router: ~15 KB
- Pinia: ~10 KB
- Total framework overhead: ~65 KB (gzipped)

## 测试清单

### 视觉回归测试

- [ ] Default state (login)
- [ ] Default state (register)
- [ ] Loading state
- [ ] Error state (validation)
- [ ] Error state (API)
- [ ] Success state
- [ ] Mobile view
- [ ] Tablet view
- [ ] Desktop view
- [ ] Focus states
- [ ] Hover states

### 跨浏览器测试

- [ ] Chrome (Windows, Mac, Linux)
- [ ] Firefox (Windows, Mac, Linux)
- [ ] Safari (Mac, iOS)
- [ ] Edge (Windows)
- [ ] Chrome Mobile (Android)
- [ ] Safari Mobile (iOS)

### 可访问性测试

- [ ] Keyboard navigation
- [ ] Screen reader (NVDA)
- [ ] Screen reader (JAWS)
- [ ] Screen reader (VoiceOver)
- [ ] Color contrast
- [ ] Focus indicators
- [ ] Error announcements

## 设计资产

### Figma/Sketch 文件

不适用，当前直接使用 Tailwind 在代码中设计。

### Design Tokens

- 在 Tailwind config 中定义。
- 与设计系统保持一致。
- 可在所有组件中复用。

### 图标体系

- 内联 SVG icons。
- Heroicons（outline 和 solid）。
- 保持一致的 stroke width。
- 通过合适的 ARIA labels 保证可访问性。

---

**说明：** 本视觉指南用于参考和文档说明。实际实现位于 Vue 组件中，并使用 TailwindCSS classes。
