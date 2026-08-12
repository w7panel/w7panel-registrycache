import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
// import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
    plugins: [
        vue(),
        vueJsx(),
        // vueDevTools(),
    ],
    base: './',
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url))
        },
    },
    server: {
    proxy: {
        '/api': {
            target: 'http://172.16.1.137:8000',
            changeOrigin: true
        },
        '/k8s-proxy': {
            target: 'http://172.16.1.137:8000',
            changeOrigin: true
        }
    }
  }
})
