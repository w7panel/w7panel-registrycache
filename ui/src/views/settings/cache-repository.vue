<template>
    <div class="padding-20">
        <el-alert
            v-if="loadError"
            title="全局配置加载失败，可以稍后重试"
            type="warning"
            show-icon
            :closable="false"
            class="mb-20"
        />
        <el-skeleton v-if="loading" :rows="4" animated />
        <template v-else>
            <CacheRepositoryForm v-model="form" />
            <div class="form-actions">
                <el-button type="primary" :loading="saving" @click="submit">保存配置</el-button>
            </div>
        </template>
    </div>
</template>

<script>
import { getGlobalCacheRepository, responseData, saveGlobalCacheRepository } from '../../api/config';
import CacheRepositoryForm from '../../components/cache-repository-form.vue';

const createForm = () => ({ repository_url: '', storage_path: '/', username: '', password: '' });

export default {
    name: 'GlobalCacheRepositorySetting',
    components: { CacheRepositoryForm },
    data() {
        return { form: createForm(), loading: true, saving: false, loadError: false };
    },
    created() {
        this.load();
    },
    methods: {
        async load() {
            this.loading = true;
            this.loadError = false;
            try {
                this.form = { ...createForm(), ...responseData(await getGlobalCacheRepository(true)) };
                this.normalizeStoragePath();
            } catch (e) {
                this.loadError = true;
            } finally {
                this.loading = false;
            }
        },
        normalizeStoragePath() {
            const value = this.form.storage_path?.trim() || '/';
            this.form.storage_path = value.startsWith('/') ? value : `/${value}`;
        },
        async submit() {
            this.normalizeStoragePath();
            if (!this.form.repository_url) {
                this.$message.warning('请输入镜像仓库地址');
                return;
            }
            try {
                const url = new URL(this.form.repository_url);
                if (!['http:', 'https:'].includes(url.protocol)) throw new Error();
            } catch (e) {
                this.$message.warning('请输入以 http:// 或 https:// 开头的有效地址');
                return;
            }

            this.saving = true;
            try {
                await saveGlobalCacheRepository(this.form);
                this.$message.success('全局缓存仓库配置已保存');
                this.loadError = false;
            } catch (e) {
                this.$message.error('保存失败，请稍后重试');
            } finally {
                this.saving = false;
            }
        },
    },
};
</script>

<style scoped>
.form-actions { display: flex; max-width: 760px; margin-top: 32px; justify-content: flex-start; }
</style>
