<script setup>
import { onBeforeUnmount } from 'vue'
import { RouterView, useRouter } from 'vue-router'

const router = useRouter()

const handleRouteChange = (path) => {
  router.push(path)
}

window.$wujie?.bus.$on('routeChange', handleRouteChange)

onBeforeUnmount(() => {
  window.$wujie?.bus.$off('routeChange', handleRouteChange)
})
</script>

<template>
  <RouterView />
</template>