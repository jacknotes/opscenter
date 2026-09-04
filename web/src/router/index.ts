import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { i18n } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { getToken } from '@/utils/session'

// 路由表：meta.public 免登录；meta.adminOnly 仅管理员；meta.titleKey 文档标题 i18n key
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true, titleKey: 'app.name' },
  },
  {
    path: '/',
    component: () => import('@/components/AppLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard/index.vue'),
        meta: { titleKey: 'nav.dashboard' },
      },
      {
        path: 'lvs',
        name: 'LvsManage',
        component: () => import('@/views/LvsManage/index.vue'),
        meta: { titleKey: 'nav.lvs' },
      },
      {
        path: 'nginx',
        name: 'NginxManage',
        component: () => import('@/views/NginxManage/index.vue'),
        meta: { titleKey: 'nav.nginx' },
      },
      {
        path: 'k8s',
        name: 'K8sDeploy',
        component: () => import('@/views/K8sDeploy.vue'),
        meta: { titleKey: 'nav.k8s' },
      },
      {
        path: 'preprod',
        name: 'PreprodScale',
        component: () => import('@/views/PreprodScale.vue'),
        meta: { titleKey: 'nav.preprod' },
      },
      {
        path: 'logs',
        name: 'OpLog',
        component: () => import('@/views/OpLog.vue'),
        meta: { titleKey: 'nav.logs' },
      },
      {
        path: 'servers',
        name: 'ServerManage',
        component: () => import('@/views/ServerManage.vue'),
        meta: { titleKey: 'nav.servers', adminOnly: true },
      },
      {
        path: 'users',
        name: 'UserManage',
        component: () => import('@/views/UserManage.vue'),
        meta: { titleKey: 'nav.users', adminOnly: true },
      },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

const t = i18n.global.t

router.beforeEach((to) => {
  const auth = useAuthStore()
  // 未登录访问受保护页 → 跳登录（带回调）
  if (!to.meta.public && !getToken()) {
    return { name: 'Login', query: { redirect: encodeURIComponent(to.fullPath) } }
  }
  // 已登录访问登录页 → 回首页
  if (to.name === 'Login' && auth.isLoggedIn) {
    return { path: '/dashboard' }
  }
  // 非 admin 访问 adminOnly → 回首页
  if (to.meta.adminOnly && !auth.isAdmin) {
    return { path: '/dashboard' }
  }
  return true
})

router.afterEach((to) => {
  const key = (to.meta.titleKey as string) || 'app.name'
  document.title = `${t(key)} · OpsCenter`
})

export default router
