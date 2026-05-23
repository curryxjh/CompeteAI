const ACCESS_KEY = 'competeai_access_token'
const REFRESH_KEY = 'competeai_refresh_token'
const EMAIL_KEY = 'competeai_email'

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_KEY) ?? ''
}

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_KEY) ?? ''
}

export function getStoredEmail(): string {
  return localStorage.getItem(EMAIL_KEY) ?? ''
}

export function saveTokens(access: string, refresh: string, email?: string) {
  localStorage.setItem(ACCESS_KEY, access)
  localStorage.setItem(REFRESH_KEY, refresh)
  if (email) {
    localStorage.setItem(EMAIL_KEY, email)
  }
}

export function saveAccessToken(access: string) {
  localStorage.setItem(ACCESS_KEY, access)
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
  localStorage.removeItem(EMAIL_KEY)
}
