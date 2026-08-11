<template>
    <div class="padding-20">
        <el-alert
            v-if="loadError"
            title="页面设置加载失败，可以稍后重试"
            type="warning"
            show-icon
            :closable="false"
            class="mb-20"
        />
        <el-skeleton v-if="loading" :rows="10" animated />
        <el-form
            v-else
            ref="pageSettingForm"
            :model="form"
            :rules="rules"
            label-position="left"
            label-width="180px"
            class="page-setting-form"
        >
            <section class="setting-section">
                <div class="section-heading">
                    <h2>自定义内容</h2>
                    <span>留空时首页底部不展示自定义内容。</span>
                </div>
                <MdEditor
                    ref="markdownEditor"
                    id="public-page-markdown-editor"
                    v-model="form.markdown"
                    class="markdown-editor"
                    :preview="true"
                    :toolbars-exclude="['github', 'save']"
                    placeholder="# 镜像加速服务\n\n在这里编写公开首页内容……"
                />
            </section>

            <section class="setting-section footer-setting">
                <div class="section-heading">
                    <h2>页脚信息</h2>
                    <span>备案链接留空时自动使用官方查询地址，版权链接留空时只展示文字。</span>
                </div>
                <div class="footer-fields">
                    <el-form-item label="ICP备案号">
                        <el-input v-model.trim="form.icp_number" placeholder="例如：晋ICP备XXXXXXXX号" />
                    </el-form-item>
                    <el-form-item label="ICP备案跳转链接" prop="icp_link">
                        <el-input v-model.trim="form.icp_link" placeholder="填写后会覆盖默认值" />
                    </el-form-item>
                    <el-form-item label="公安备案号">
                        <el-input v-model.trim="form.police_number" placeholder="例如：晋公网安备XXXXXXXX号" />
                    </el-form-item>
                    <el-form-item label="公安备案跳转链接" prop="police_link">
                        <el-input v-model.trim="form.police_link" placeholder="填写后会覆盖默认值" />
                    </el-form-item>
                    <el-form-item label="版权信息">
                        <el-input v-model.trim="form.copyright" placeholder="例如：© 2026 W7" />
                    </el-form-item>
                    <el-form-item label="版权跳转链接" prop="copyright_link">
                        <el-input v-model.trim="form.copyright_link" placeholder="https://example.com" />
                    </el-form-item>
                </div>
            </section>

            <div class="form-actions">
                <el-button type="primary" :loading="saving" @click="submit">保存设置</el-button>
            </div>
        </el-form>
    </div>
</template>

<script>
import { getPageSetting, responseData, savePageSetting } from '../../api/config';
import { MdEditor } from 'md-editor-v3';
import 'md-editor-v3/lib/style.css';

const createForm = () => ({
    markdown: '',
    icp_number: '',
    icp_link: '',
    police_number: '',
    police_link: '',
    copyright: '',
    copyright_link: '',
});

export default {
    name: 'PublicPageSetting',
    components: { MdEditor },
    data() {
        const validateOptionalUrl = (rule, value, callback) => {
            if (!value) {
                callback();
                return;
            }
            try {
                const url = new URL(value);
                if (!['http:', 'https:'].includes(url.protocol)) throw new Error();
                callback();
            } catch (e) {
                callback(new Error('请输入以 http:// 或 https:// 开头的有效链接'));
            }
        };
        return {
            form: createForm(),
            loading: true,
            saving: false,
            loadError: false,
            rules: {
                icp_link: [{ validator: validateOptionalUrl, trigger: 'blur' }],
                police_link: [{ validator: validateOptionalUrl, trigger: 'blur' }],
                copyright_link: [{ validator: validateOptionalUrl, trigger: 'blur' }],
            },
        };
    },
    created() {
        this.load();
    },
    methods: {
        async load() {
            this.loading = true;
            this.loadError = false;
            try {
                this.form = { ...createForm(), ...responseData(await getPageSetting(true)) };
            } catch (e) {
                this.loadError = true;
            } finally {
                this.loading = false;
                await this.$nextTick();
                this.$refs.markdownEditor?.togglePreviewOnly(true);
            }
        },
        async submit() {
            const valid = await this.$refs.pageSettingForm.validate().catch(() => false);
            if (!valid) return;
            this.saving = true;
            try {
                await savePageSetting(this.form);
                this.$message.success('公开首页设置已保存');
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
.page-setting-form { max-width: 1180px; }
.section-heading span { color: #86909c; }
.section-heading { display: flex; margin-bottom: 18px; align-items: flex-start; flex-direction: column; gap: 0; }
h2 { margin-top: 0; margin-bottom: 8px; font-size: 17px; }
.setting-section { padding-bottom: 28px; }
.markdown-editor { height: 520px; }
.footer-fields { max-width: 760px; }
.footer-fields :deep(.el-form-item) { margin-bottom: 20px; }
.form-actions { display: flex; justify-content: flex-start; }
@media (max-width: 760px) {
    .footer-fields :deep(.el-form-item) { display: block; }
    .footer-fields :deep(.el-form-item__label) { width: auto !important; }
    .footer-fields :deep(.el-form-item__content) { margin-left: 0 !important; }
}
</style>
