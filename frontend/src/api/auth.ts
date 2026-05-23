import axios from 'axios'

const authHttp = axios.create({
  timeout: 30000,
})

export interface SignUpPayload {
  email: string
  password: string
  confirmPassword: string
}

export interface LoginPayload {
  email: string
  password: string
}

export interface AuthTokens {
  accessToken: string
  refreshToken: string
}

function readTokens(headers: Record<string, unknown>): AuthTokens {
  const access =
    (headers['x-jwt-token'] as string | undefined) ??
    (headers['X-Jwt-Token'] as string | undefined) ??
    ''
  const refresh =
    (headers['x-refresh-token'] as string | undefined) ??
    (headers['X-Refresh-Token'] as string | undefined) ??
    ''
  return { accessToken: access, refreshToken: refresh }
}

export async function signUp(payload: SignUpPayload): Promise<string> {
  const res = await authHttp.post<string>('/users/signup', payload, {
    responseType: 'text',
  })
  if (!res.data.includes('成功')) {
    throw new Error(res.data || '注册失败')
  }
  return res.data
}

export async function login(payload: LoginPayload): Promise<AuthTokens & { message: string }> {
  const res = await authHttp.post<string>('/users/login', payload, {
    responseType: 'text',
  })
  if (!res.data.includes('成功')) {
    throw new Error(res.data || '登录失败')
  }
  const tokens = readTokens(res.headers as Record<string, unknown>)
  if (!tokens.accessToken || !tokens.refreshToken) {
    throw new Error('登录成功但未收到 Token，请检查后端响应头')
  }
  return { message: res.data, ...tokens }
}

export async function logout(accessToken: string): Promise<void> {
  await authHttp.post('/users/logout', null, {
    headers: { Authorization: `Bearer ${accessToken}` },
  })
}

export async function refreshAccessToken(refreshToken: string): Promise<string> {
  const res = await authHttp.post('/users/refresh_token', null, {
    headers: { Authorization: `Bearer ${refreshToken}` },
  })
  const access =
    (res.headers['x-jwt-token'] as string | undefined) ??
    (res.headers['X-Jwt-Token'] as string | undefined) ??
    ''
  if (!access) {
    throw new Error('刷新 Token 失败')
  }
  return access
}

export async function ping(accessToken?: string): Promise<{ message: string }> {
  const headers = accessToken
    ? { Authorization: `Bearer ${accessToken}` }
    : undefined
  const res = await authHttp.get<{ message: string }>('/ping', { headers })
  return res.data
}
