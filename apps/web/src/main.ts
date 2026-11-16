import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import { setRefreshHandler, setUnauthorizedHandler } from '@mlf-erp/sdk'
import App from './App.vue'
import router from './router'
import { useAuthStore } from './stores/auth'
import './styles/index.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(ElementPlus)

const authStore = useAuthStore(pinia)
setRefreshHandler(() => authStore.refreshSession())
setUnauthorizedHandler(() => {
  authStore.clear()
  if (router.currentRoute.value.name !== 'login') {
    router.push({ name: 'login' })
  }
})

window.addEventListener('storage', (event) => {
  if (event.key !== 'mlf_access_token' && event.key !== 'mlf_refresh_token') {
    return
  }
  authStore.restoreFromStorage()
  if (authStore.hasToken && !authStore.user) {
    authStore.loadMe().catch(() => authStore.clear())
  }
})

app.mount('#app')
