import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/register',
      name: 'Register',
      component: () => import('../views/RegisterView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/forgot-password',
      name: 'ForgotPassword',
      component: () => import('../views/ForgotPasswordView.vue'),
      meta: { requiresAuth: false }
    },
    {
      // 后台路由统一前缀为 /admin，需要登录。
      path: '/admin',
      component: () => import('../layouts/MainLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'Home', component: () => import('../views/HomeView.vue'), meta: { reminderView: 'today', title: '今天' } },
        { path: 'planned', name: 'Planned', component: () => import('../views/HomeView.vue'), meta: { reminderView: 'planned', title: '计划' } },
        { path: 'all', name: 'AllReminders', component: () => import('../views/HomeView.vue'), meta: { reminderView: 'all', title: '全部' } },
        { path: 'completed', name: 'Completed', component: () => import('../views/HomeView.vue'), meta: { reminderView: 'completed', title: '已完成' } },
        { path: 'list/:id', name: 'ReminderList', component: () => import('../views/HomeView.vue'), meta: { reminderView: 'all', title: '我的清单' } },
        { path: 'notifications', name: 'Notifications', component: () => import('../views/NotificationsView.vue'), meta: { title: '通知中心' } },
        { path: 'channels', name: 'Channels', component: () => import('../views/ChannelsView.vue'), meta: { title: '通知方式' } },
        { path: 'profile', name: 'Profile', component: () => import('../views/ProfileView.vue') },
        { path: 'settings', name: 'Settings', component: () => import('../views/SettingsView.vue') },
        { path: 'users', name: 'AdminUsers', component: () => import('../views/AdminUsersView.vue'), meta: { requiresAdmin: true } },
        { path: 'configs', name: 'AdminConfigs', component: () => import('../views/AdminConfigView.vue'), meta: { requiresAdmin: true } },
        // Provider credentials belong to “通知方式”; retain old URLs as a safe redirect.
        { path: 'providers', redirect: '/admin/channels' },
        { path: 'audit', name: 'AdminAudit', component: () => import('../views/AdminAuditView.vue'), meta: { requiresAdmin: true } },
      ]
    },
    {
      // 后期 / 用于免登录的门户或前端页面，当前先重定向到后台首页。
      path: '/',
      redirect: '/admin',
      meta: { requiresAuth: false }
    },
    {
      // 兜底路由：飞牛远程地址的入口可能落在不带末尾斜杠的网关前缀上
      //（如 /app/techfunway-reminders），vue-router 剥离 base 后得到的
      // 路径匹配不到任何路由，router-view 会渲染成一片空白（手机端白屏）。
      // 一律送回首页，未登录时由全局守卫转入登录页。
      path: '/:pathMatch(.*)*',
      redirect: '/',
      meta: { requiresAuth: false }
    }
  ]
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  await authStore.init()

  if (authStore.setupRequired && to.name !== 'Register') {
    // 首次安装默认走普通的用户名密码创建管理员流程，不做任何默认的飞牛
    // 授权；只有用户主动点击「使用飞牛 NAS 登录」并确认后，才进入飞牛绑定
    // 创建模式（注册页内会带上 fnos=bind 查询参数）。
    next({ name: 'Register' })
    return
  }

  if (to.meta.requiresAuth !== false && !authStore.isAuthenticated && authStore.requireLogin) {
    next({ name: 'Login' })
  } else if (to.meta.requiresAdmin && !authStore.isAdmin) {
    next({ name: 'Home' })
  } else {
    next()
  }
})

export default router
