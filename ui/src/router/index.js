import { createRouter, createWebHashHistory } from 'vue-router'

const router = createRouter({
    history: createWebHashHistory(import.meta.env.BASE_URL),
    routes: [
        {
            path: '/',
            name: 'public-home',
            component: () => import('@/views/public/home.vue'),
        },
        {
            path: '/public',
            redirect: '/',
        },
        {
            path: '/cache',
            name: 'home',
            component: ()=>import('@/views/home/list'),
        },
        {
            path: '/cache/repository',
            name: 'cache-repository-setting',
            component: () => import('@/views/settings/cache-repository.vue'),
        },
        {
            path: '/cache/page-setting',
            name: 'public-page-setting',
            component: () => import('@/views/settings/page-setting.vue'),
        },
        {
            path: '/setting',
            name: 'setting',
            component: ()=>import('@/views/home/setting'),
        },
        {
            path: '/:pathMatch(.*)*',
            name: 'notFound',
            component: () => import('@/views/not-found/index.vue'),
        }
    ],
})

router.beforeEach((to) => {
    // 管理端作为无界微应用加载时仍保持原有默认入口；独立访问根路径才展示公开首页。
    if (to.name === 'public-home' && window.__POWERED_BY_WUJIE__) {
        return { name: 'home' };
    }
    return true;
})

export default router
