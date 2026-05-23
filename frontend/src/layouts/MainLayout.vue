<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const useMock = import.meta.env.VITE_USE_MOCK !== 'false'

const activeMenu = computed(() => {
  if (route.name === 'agents') return '/agents'
  if (route.name === 'chat') return '/chat'
  return route.path.startsWith('/report') || route.path.startsWith('/trace') ? '/' : route.path
})

const pageTitle = computed(() => (route.meta.title as string) ?? 'CompeteAI')

function go(path: string) {
  router.push(path)
}

async function onLogout() {
  await auth.logout()
  ElMessage.success('已退出登录')
  if (!useMock) router.push({ name: 'login' })
}

async function onTestPing() {
  try {
    const res = await auth.testPing()
    ElMessage.success(`ping → ${res.message}`)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : 'Ping 失败')
  }
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand" @click="go('/')">
        <div class="brand-mark">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <div class="brand-text">
          <div class="brand-name">CompeteAI</div>
          <div class="brand-sub">agent workspace</div>
        </div>
      </div>

      <nav class="nav">
        <div class="nav-label">Workspace</div>
        <button
          class="nav-item"
          :class="{ active: activeMenu === '/' }"
          @click="go('/')"
        >
          <svg class="nav-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2"/>
            <rect x="9" y="3" width="6" height="4" rx="1"/>
            <path d="M9 12h6M9 16h4"/>
          </svg>
          <span>任务管理</span>
        </button>
        <button
          class="nav-item"
          :class="{ active: activeMenu === '/agents' }"
          @click="go('/agents')"
        >
          <svg class="nav-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2a4 4 0 0 1 4 4c0 1.5-.8 2.8-2 3.4V12h1a7 7 0 0 1 7 7v1H3v-1a7 7 0 0 1 7-7h1V9.4A4 4 0 0 1 12 2z"/>
            <path d="M9 20v1a3 3 0 0 0 6 0v-1"/>
          </svg>
          <span>Agent 能力</span>
        </button>
        <button
          class="nav-item"
          :class="{ active: activeMenu === '/chat' }"
          @click="go('/chat')"
        >
          <svg class="nav-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 3c-4 0-7 2.5-7 6 0 2.2 1.2 4.1 3 5.2V19l4-2 4 2v-4.8c1.8-1.1 3-3 3-5.2 0-3.5-3-6-7-6z"/>
            <path d="M9.5 10.5h.01M14.5 10.5h.01"/>
          </svg>
          <span>AI 对话</span>
        </button>
      </nav>

      <div class="sidebar-foot">
        <span class="status-dot" :class="{ on: auth.isLoggedIn }" />
        <span class="status-text">{{ auth.isLoggedIn ? 'session active' : 'offline' }}</span>
        <span v-if="useMock" class="mock-chip">mock</span>
      </div>
    </aside>

    <div class="main-col">
      <header class="topbar">
        <div class="topbar-left">
          <h1 class="topbar-title">{{ pageTitle }}</h1>
        </div>
        <div class="topbar-right">
          <template v-if="auth.isLoggedIn">
            <span class="topbar-email">{{ auth.email }}</span>
            <button type="button" class="btn-ghost btn-sm" @click="onTestPing">ping</button>
            <button type="button" class="btn-ghost btn-sm" @click="onLogout">退出</button>
          </template>
          <template v-else>
            <button type="button" class="btn-ghost btn-sm" @click="router.push('/login')">登录</button>
            <button type="button" class="btn-accent btn-sm" @click="router.push('/signup')">注册</button>
          </template>
        </div>
      </header>

      <main class="content" :class="{ 'content--flush': route.meta.flush }">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  min-height: 100vh;
  background: var(--bg-base);
}

/* ---- Sidebar ---- */
.sidebar {
  width: var(--sidebar-w);
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border-subtle);
  background: var(--bg-panel);
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 24px 22px;
  cursor: pointer;
  transition: opacity 0.15s;
}

.brand:hover {
  opacity: 0.8;
}

.brand-mark {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: linear-gradient(135deg, var(--accent), #7c3aed);
  color: #fff;
  flex-shrink: 0;
}

.brand-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.brand-sub {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 1px;
  font-family: var(--font-mono);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

/* ---- Nav ---- */
.nav {
  flex: 1;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 4px 12px 10px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  background: none;
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  text-align: left;
  transition: all 0.15s;
  font-family: var(--font-sans);
}

.nav-icon {
  flex-shrink: 0;
  opacity: 0.45;
  transition: opacity 0.15s;
}

.nav-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.nav-item:hover .nav-icon {
  opacity: 0.7;
}

.nav-item.active {
  color: var(--text-primary);
  background: var(--accent-dim);
  font-weight: 600;
}

.nav-item.active .nav-icon {
  opacity: 1;
  color: var(--accent);
}

/* ---- Sidebar foot ---- */
.sidebar-foot {
  padding: 16px 22px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--text-muted);
  flex-shrink: 0;
}

.status-dot.on {
  background: var(--success);
  box-shadow: 0 0 6px rgba(16, 185, 129, 0.4);
}

.status-text {
  flex: 1;
}

.mock-chip {
  font-size: 9px;
  padding: 1px 6px;
  border-radius: 999px;
  border: 1px solid var(--border-default);
  color: var(--text-muted);
  font-family: var(--font-mono);
}

/* ---- Main column ---- */
.main-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--bg-base);
}

/* ---- Topbar ---- */
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--topbar-h);
  padding: 0 28px;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(12px);
  flex-shrink: 0;
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.topbar-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
}

.topbar-email {
  font-size: 12px;
  color: var(--text-muted);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
}

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}

/* ---- Content ---- */
.content {
  flex: 1;
  padding: 28px;
  overflow: auto;
}

.content--flush {
  padding: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
</style>
