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

  if (route.name === 'report') return '/'

  if (route.name === 'trace') return '/'

  return route.path

})



const pageTitle = computed(() => (route.meta.title as string) ?? 'CompeteAI')



function go(path: string) {

  router.push(path)

}



async function onLogout() {

  await auth.logout()

  ElMessage.success('已退出登录')

  if (!useMock) {

    router.push({ name: 'login' })

  }

}



async function onTestPing() {

  try {

    const res = await auth.testPing()

    ElMessage.success(`后端 /ping：${res.message}`)

  } catch (e) {

    ElMessage.error(e instanceof Error ? e.message : 'Ping 失败')

  }

}

</script>



<template>

  <el-container class="layout">

    <el-aside width="220px" class="aside">

      <div class="brand" @click="go('/')">

        <span class="brand-icon">CA</span>

        <div>

          <div class="brand-title">CompeteAI</div>

          <div class="brand-sub">竞品分析 Agent 系统</div>

        </div>

      </div>

      <el-menu

        :default-active="activeMenu"

        router

        class="menu"

      >

        <el-menu-item index="/" @click="go('/')">

          <span>任务管理</span>

        </el-menu-item>

        <el-menu-item index="/agents" @click="go('/agents')">

          <span>Agent 能力</span>

        </el-menu-item>

      </el-menu>

    </el-aside>



    <el-container>

      <el-header class="header">

        <div class="header-left">

          <h1>{{ pageTitle }}</h1>

          <span class="header-hint">AI 驱动的多 Agent 竞品分析</span>

        </div>

        <div class="header-right">

          <el-tag v-if="useMock" size="small" type="info">Mock 数据</el-tag>

          <template v-if="auth.isLoggedIn">

            <span class="user-email">{{ auth.email }}</span>

            <el-button size="small" @click="onTestPing">测试 Ping</el-button>

            <el-button size="small" @click="onLogout">退出</el-button>

          </template>

          <template v-else>

            <el-button size="small" @click="router.push('/login')">登录</el-button>

            <el-button size="small" type="primary" @click="router.push('/signup')">

              注册

            </el-button>

          </template>

        </div>

      </el-header>

      <el-main class="main">

        <router-view />

      </el-main>

    </el-container>

  </el-container>

</template>



<style scoped>

.layout {

  min-height: 100vh;

}

.aside {

  background: #1d1e2c;

  color: #fff;

  display: flex;

  flex-direction: column;

}

.brand {

  display: flex;

  gap: 12px;

  padding: 20px 16px;

  cursor: pointer;

  align-items: center;

}

.brand-icon {

  width: 40px;

  height: 40px;

  border-radius: 10px;

  background: linear-gradient(135deg, #409eff, #67c23a);

  display: flex;

  align-items: center;

  justify-content: center;

  font-weight: 700;

  font-size: 14px;

}

.brand-title {

  font-weight: 600;

  font-size: 15px;

}

.brand-sub {

  font-size: 11px;

  opacity: 0.65;

  margin-top: 2px;

}

.menu {

  border-right: none;

  background: transparent;

  flex: 1;

}

.menu :deep(.el-menu-item) {

  color: rgba(255, 255, 255, 0.85);

}

.menu :deep(.el-menu-item.is-active) {

  background: rgba(64, 158, 255, 0.2);

  color: #fff;

}

.header {

  display: flex;

  align-items: center;

  justify-content: space-between;

  gap: 12px;

  border-bottom: 1px solid #ebeef5;

  background: #fff;

}

.header-left {

  display: flex;

  align-items: baseline;

  gap: 12px;

}

.header-right {

  display: flex;

  align-items: center;

  gap: 8px;

}

.header h1 {

  margin: 0;

  font-size: 18px;

  font-weight: 600;

}

.header-hint {

  font-size: 13px;

  color: #909399;

}

.user-email {

  font-size: 13px;

  color: #606266;

  max-width: 180px;

  overflow: hidden;

  text-overflow: ellipsis;

  white-space: nowrap;

}

.main {

  background: #f5f7fa;

  min-height: calc(100vh - 60px);

}

</style>


