import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录', public: true },
    },
    {
      path: '/signup',
      name: 'signup',
      component: () => import('@/views/SignUpView.vue'),
      meta: { title: '注册', public: true },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/Dashboard.vue'),
          meta: { title: '任务管理' },
        },
        {
          path: 'report/:taskId',
          name: 'report',
          component: () => import('@/views/ReportView.vue'),
          meta: { title: '报告查看' },
        },
        {
          path: 'trace/:taskId',
          name: 'trace',
          component: () => import('@/views/TraceView.vue'),
          meta: { title: 'Agent 追踪' },
        },
        {
          path: 'agents',
          name: 'agents',
          component: () => import('@/views/AgentsView.vue'),
          meta: { title: 'Agent 能力' },
        },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  const useMock = import.meta.env.VITE_USE_MOCK !== 'false'

  if (to.meta.public && auth.isLoggedIn && (to.name === 'login' || to.name === 'signup')) {
    return { path: '/' }
  }

  if (!useMock && to.matched.some((r) => r.meta.requiresAuth) && !auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  return true
})

router.afterEach((to) => {
  const title = (to.meta.title as string) ?? 'CompeteAI'
  document.title = `${title} · CompeteAI`
})

export default router
