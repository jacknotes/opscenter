import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/',
    component: () => import('../components/Layout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: '总览' }
      },
      {
        path: 'lvs',
        name: 'LvsManage',
        component: () => import('../views/LvsManage.vue'),
        meta: { title: 'LVS管理' }
      },
      {
        path: 'nginx',
        name: 'NginxManage',
        component: () => import('../views/NginxManage.vue'),
        meta: { title: 'Nginx管理' }
      },
      {
        path: 'k8s',
        name: 'K8sDeploy',
        component: () => import('../views/K8sDeploy.vue'),
        meta: { title: 'K8s部署' }
      },
      {
        path: 'preprod',
        name: 'PreprodScale',
        component: () => import('../views/PreprodScale.vue'),
        meta: { title: '预生产缩扩容' }
      },
      {
        path: 'logs',
        name: 'OpLog',
        component: () => import('../views/OpLog.vue'),
        meta: { title: '操作日志' }
      },
      {
        path: 'servers',
        name: 'ServerManage',
        component: () => import('../views/ServerManage.vue'),
        meta: { title: '服务器管理', admin: true }
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.path === '/login') {
    next()
  } else if (!token) {
    next('/login')
  } else {
    next()
  }
})

export default router
