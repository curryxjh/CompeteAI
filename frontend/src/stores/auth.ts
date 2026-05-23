import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '@/api/auth'
import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
  getStoredEmail,
  saveAccessToken,
  saveTokens,
} from '@/utils/token'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(getAccessToken())
  const refreshToken = ref(getRefreshToken())
  const email = ref(getStoredEmail())
  const loading = ref(false)

  const isLoggedIn = computed(() => !!accessToken.value)

  async function signUp(payload: authApi.SignUpPayload) {
    loading.value = true
    try {
      return await authApi.signUp(payload)
    } finally {
      loading.value = false
    }
  }

  async function login(payload: authApi.LoginPayload) {
    loading.value = true
    try {
      const res = await authApi.login(payload)
      accessToken.value = res.accessToken
      refreshToken.value = res.refreshToken
      email.value = payload.email
      saveTokens(res.accessToken, res.refreshToken, payload.email)
      return res.message
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    const token = accessToken.value
    if (token) {
      try {
        await authApi.logout(token)
      } catch {
        // 会话已过期时仍清除本地状态
      }
    }
    clearSession()
  }

  async function refreshTokens() {
    const rt = refreshToken.value || getRefreshToken()
    if (!rt) {
      throw new Error('无 Refresh Token')
    }
    const newAccess = await authApi.refreshAccessToken(rt)
    accessToken.value = newAccess
    saveAccessToken(newAccess)
    return newAccess
  }

  function clearSession() {
    accessToken.value = ''
    refreshToken.value = ''
    email.value = ''
    clearTokens()
  }

  async function testPing() {
    if (!accessToken.value) {
      throw new Error('请先登录')
    }
    return authApi.ping(accessToken.value)
  }

  return {
    accessToken,
    refreshToken,
    email,
    loading,
    isLoggedIn,
    signUp,
    login,
    logout,
    refreshTokens,
    clearSession,
    testPing,
  }
})
