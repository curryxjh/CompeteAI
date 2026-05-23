<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const formRef = ref<FormInstance>()

const form = reactive({
  email: (route.query.email as string) ?? '',
  password: '',
})

const rules: FormRules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

async function onSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  try {
    const msg = await auth.login({ email: form.email, password: form.password })
    ElMessage.success(msg || '登录成功')
    router.replace((route.query.redirect as string) || '/')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '登录失败')
  }
}
</script>

<template>
  <div class="auth-page">
    <div class="bg-ambient" />

    <div class="auth-card">
      <div class="auth-header">
        <div class="auth-logo">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5z"/>
            <path d="M2 17l10 5 10-5"/>
            <path d="M2 12l10 5 10-5"/>
          </svg>
        </div>
        <h1>欢迎回来</h1>
        <p>登录以继续使用 CompeteAI</p>
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="onSubmit">
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="you@example.com" size="large" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            size="large"
            @keyup.enter="onSubmit"
          />
        </el-form-item>
        <button type="button" class="btn-accent submit-btn" :disabled="auth.loading" @click="onSubmit">
          {{ auth.loading ? '登录中...' : '登录' }}
        </button>
      </el-form>

      <p class="auth-footer">
        还没有账号？<router-link to="/signup">立即注册</router-link>
      </p>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--bg-base);
  position: relative;
}

.bg-ambient {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(ellipse 70% 50% at 50% 40%, rgba(99, 102, 241, 0.04), transparent 50%),
    radial-gradient(ellipse 50% 40% at 50% 80%, rgba(124, 58, 237, 0.02), transparent 50%);
}

.auth-card {
  position: relative;
  width: 100%;
  max-width: 400px;
  padding: 40px 36px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-elevated);
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;
}

.auth-logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  background: linear-gradient(135deg, var(--accent), #7c3aed);
  color: #fff;
  margin-bottom: 20px;
}

.auth-header h1 {
  margin: 0 0 8px;
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.auth-header p {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}

.submit-btn {
  width: 100%;
  justify-content: center;
  margin-top: 4px;
  padding: 11px 24px;
  font-size: 14px;
  font-weight: 600;
}

.auth-footer {
  text-align: center;
  margin: 24px 0 0;
  font-size: 13px;
  color: var(--text-muted);
}

.auth-footer a {
  color: var(--accent-light);
  font-weight: 500;
}

:deep(.el-form-item__label) {
  font-size: 13px !important;
  font-weight: 500 !important;
  color: var(--text-secondary) !important;
  padding-bottom: 6px !important;
}

:deep(.el-input__wrapper) {
  border-radius: var(--radius-sm) !important;
}
</style>
