import { createRouter, createWebHistory,createWebHashHistory } from 'vue-router'

const router = createRouter({
    history: createWebHashHistory(import.meta.env.BASE_URL),
    routes: [
        {
            path: '/',
            name: 'home',
            component: ()=>import('@/views/home/list'),
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

export default router
