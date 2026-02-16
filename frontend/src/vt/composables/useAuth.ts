import { ref } from 'vue'
import vtApi, { type UserProfile } from '../../api/vt'
import { getAuthKey, setAuthKey } from '../../api/auth'
import { client } from '../../api/vt'

const user = ref<UserProfile | null>(null)
const isAuthenticated = ref(!!getAuthKey())

export function useAuth() {
  async function login(login: string, password: string, remember: boolean) {
    const authKey = await vtApi.auth.login({ login, password, remember })
    setAuthKey(authKey)
    client.setHeader('Authorization2', authKey)
    client.token = { value: authKey }
    isAuthenticated.value = true
    await loadProfile()
  }

  async function logout() {
    try {
      await vtApi.auth.logout()
    } finally {
      setAuthKey(null)
      client.setHeader('Authorization2', '')
      client.token = { value: undefined }
      user.value = null
      isAuthenticated.value = false
    }
  }

  async function loadProfile() {
    if (!getAuthKey()) return
    try {
      user.value = await vtApi.auth.profile()
      isAuthenticated.value = true
    } catch {
      user.value = null
      isAuthenticated.value = false
    }
  }

  async function changePassword(password: string) {
    const newAuthKey = await vtApi.auth.changePassword({ password })
    setAuthKey(newAuthKey)
  }

  return {
    user,
    isAuthenticated,
    login,
    logout,
    loadProfile,
    changePassword,
  }
}
