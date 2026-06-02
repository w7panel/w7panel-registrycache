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
            target: 'https://idc.w7.com',
            rewrite: (path) => {
                return '/panel-api/v1/microapp/w7-registrycache-tluypvia/proxy' + path
            },
            changeOrigin: true
        },
        '/k8s-proxy': {
            target: 'https://idc.w7.com',
            changeOrigin: true
        }
    }
  }
})
