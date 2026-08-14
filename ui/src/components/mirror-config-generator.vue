<template>
    <section class="generator-card">
        <div class="generator-heading">
            <div>
                <p class="generator-kicker">CONFIG GENERATOR</p>
                <h2>镜像配置生成器</h2>
                <p class="generator-description">选择需要使用的加速站点，生成适用于本机容器运行时的镜像配置。</p>
            </div>
            <div class="config-type">
                <span>配置类型</span>
                <el-select v-model="configType" size="large">
                    <el-option
                        v-for="item in configTypes"
                        :key="item.value"
                        :label="item.label"
                        :value="item.value"
                    />
                </el-select>
            </div>
        </div>

        <div class="source-toolbar">
            <div>
                <h3>选择加速节点</h3>
                <span>节点已按适用源站分组，同一源站可以选择多个节点。</span>
            </div>
            <div class="source-actions">
                <el-button :disabled="!selectableSources.length" @click="selectAll">全选</el-button>
                <el-button :disabled="!selectedIds.length" @click="clearSelection">清空</el-button>
            </div>
        </div>

        <el-checkbox-group v-if="selectableSources.length" v-model="selectedIds" class="source-groups">
            <section v-for="group in sourceGroups" :key="group.origin" class="source-group">
                <div class="source-group-heading">
                    <span>适用源站</span>
                    <code>{{ group.origin }}</code>
                    <small>{{ group.mirrors.length }} 个节点</small>
                </div>
                <div class="source-grid">
                    <el-checkbox
                        v-for="source in group.mirrors"
                        :key="source.id"
                        :value="source.id"
                        :label="source.id"
                        border
                        class="source-option"
                    >
                        <span class="source-name">{{ source.domain }}</span>
                        <span class="source-url">{{ source.url }}</span>
                    </el-checkbox>
                </div>
            </section>
        </el-checkbox-group>
        <el-empty v-else :description="emptySourceDescription" :image-size="72" class="source-empty" />
        <el-alert
            v-if="unconfiguredSources.length"
            :title="`${unconfiguredSources.length} 个站点未标注适用源站，无法生成运行时配置`"
            type="warning"
            show-icon
            :closable="false"
            class="source-warning"
        />

        <div class="config-result">
            <div class="result-heading">
                <div>
                    <h3>生成的配置</h3>
                    <span v-if="selectedSources.length">已选择 {{ selectedSources.length }} 个加速节点</span>
                    <span v-else>请先选择加速节点</span>
                </div>
                <el-button type="primary" :disabled="!generatedConfig" @click="copyConfig">
                    <el-icon><CopyDocument /></el-icon>
                    复制配置
                </el-button>
            </div>
            <div v-if="generatedConfig" class="config-guide">
                <div>
                    <span>保存位置</span>
                    <code>{{ configGuide.path }}</code>
                </div>
                <div>
                    <span>生效方式</span>
                    <code>{{ configGuide.apply }}</code>
                </div>
            </div>
            <pre><code>{{ generatedConfig || emptyConfigHint }}</code></pre>
        </div>
    </section>
</template>

<script>
import { CopyDocument } from '@element-plus/icons-vue';
import {
    generateMirrorConfig,
    groupMirrorSources,
    MIRROR_CONFIG_TYPES,
} from '../utils/mirror-config';

export default {
    name: 'MirrorConfigGenerator',
    components: { CopyDocument },
    props: {
        sources: { type: Array, default: () => [] },
    },
    data() {
        return {
            configType: 'docker',
            configTypes: MIRROR_CONFIG_TYPES,
            selectedIds: [],
        };
    },
    computed: {
        configGuide() {
            return this.configTypes.find((item) => item.value === this.configType) || this.configTypes[0];
        },
        selectableSources() {
            return this.sources.filter((item) => item.origin?.trim());
        },
        sourceGroups() {
            return groupMirrorSources(this.selectableSources);
        },
        selectedSources() {
            const selected = new Set(this.selectedIds);
            return this.selectableSources.filter((item) => selected.has(item.id));
        },
        unconfiguredSources() {
            return this.sources.filter((item) => !item.origin?.trim());
        },
        generatedConfig() {
            return generateMirrorConfig(this.configType, this.selectedSources);
        },
        emptySourceDescription() {
            if (!this.sources.length) return '暂未配置可用站点';
            return '暂未配置已标注适用源站的加速节点';
        },
        emptyConfigHint() {
            if (!this.selectableSources.length) return `// ${this.emptySourceDescription}`;
            if (!this.selectedSources.length) return '// 请先选择加速节点';
            return '// 暂无可生成的配置';
        },
    },
    watch: {
        configType() {
            this.syncSelection();
        },
        sources: {
            handler() {
                this.syncSelection();
            },
            immediate: true,
        },
    },
    methods: {
        syncSelection() {
            const availableIds = new Set(this.selectableSources.map((item) => item.id));
            this.selectedIds = this.selectedIds.filter((id) => availableIds.has(id));
        },
        selectAll() {
            this.selectedIds = this.selectableSources.map((item) => item.id);
        },
        clearSelection() {
            this.selectedIds = [];
        },
        async copyConfig() {
            try {
                if (navigator.clipboard && window.isSecureContext) {
                    await navigator.clipboard.writeText(this.generatedConfig);
                } else {
                    const textarea = document.createElement('textarea');
                    textarea.value = this.generatedConfig;
                    textarea.style.position = 'fixed';
                    textarea.style.opacity = '0';
                    document.body.appendChild(textarea);
                    textarea.select();
                    document.execCommand('copy');
                    textarea.remove();
                }
                this.$message.success('配置已复制');
            } catch (e) {
                this.$message.error('复制失败，请手动复制');
            }
        },
    },
};
</script>

<style scoped>
.generator-card { padding: 36px; overflow: hidden; background: #fff; border: 1px solid #e7eaf0; border-radius: 20px; box-shadow: 0 24px 60px rgb(33 56 97 / 8%); }
.generator-heading, .source-toolbar, .result-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; }
.generator-kicker { margin: 0 0 8px; color: #165dff; font-size: 12px; font-weight: 700; letter-spacing: .12em; }
h2, h3, p { margin-top: 0; }
h2 { margin-bottom: 10px; color: #17233d; font-size: 28px; }
h3 { margin-bottom: 6px; color: #17233d; font-size: 16px; }
.generator-description, .source-toolbar span, .result-heading span { margin-bottom: 0; color: #7a8499; line-height: 1.7; }
.config-type { display: flex; min-width: 270px; flex-direction: column; gap: 8px; color: #4e5969; font-size: 13px; }
.source-toolbar { align-items: center; margin-top: 36px; padding-top: 28px; border-top: 1px solid #edf0f5; }
.source-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.source-actions :deep(.el-button + .el-button) { margin-left: 0; }
.source-groups { display: block; margin-top: 18px; }
.source-group + .source-group { margin-top: 20px; padding-top: 20px; border-top: 1px solid #edf0f5; }
.source-group-heading { display: flex; min-width: 0; align-items: center; gap: 10px; }
.source-group-heading span { flex: 0 0 auto; color: #7a8499; font-size: 12px; }
.source-group-heading code { overflow: hidden; color: #344054; font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.source-group-heading small { flex: 0 0 auto; margin-left: auto; color: #98a2b3; }
.source-grid { display: grid; margin-top: 10px; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.source-grid :deep(.el-checkbox) { width: 100%; height: auto; min-height: 76px; margin-right: 0; padding: 14px 16px; align-items: flex-start; background: #f8faff; border-color: #e1e7f2; border-radius: 12px; }
.source-grid :deep(.el-checkbox.is-bordered.is-checked) { background: #eef4ff; border-color: #165dff; }
.source-grid :deep(.el-checkbox__label) { display: flex; min-width: 0; padding-left: 10px; flex-direction: column; }
.source-name, .source-url { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.source-name { color: #1d2939; font-weight: 600; }
.source-url { margin-top: 7px; color: #7a8499; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; }
.source-empty { margin-top: 18px; padding: 16px 0; background: #f8faff; border-radius: 12px; }
.source-warning { margin-top: 18px; }
.config-result { margin-top: 28px; }
.result-heading { align-items: center; margin-bottom: 12px; }
.result-heading :deep(.el-button .el-icon) { margin-right: 6px; }
.config-guide { display: grid; margin-bottom: 12px; padding: 14px 16px; background: #f8faff; border-radius: 10px; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.config-guide div { display: flex; min-width: 0; flex-direction: column; gap: 6px; }
.config-guide span { color: #7a8499; font-size: 12px; }
.config-guide code { overflow-wrap: anywhere; color: #344054; font-size: 12px; }
pre { min-height: 210px; max-height: 460px; margin: 0; padding: 24px; overflow: auto; color: #d8e4ff; background: #11182a; border-radius: 14px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 13px; line-height: 1.75; white-space: pre-wrap; }
@media (max-width: 900px) { .source-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 640px) {
    .generator-card { padding: 22px; border-radius: 16px; }
    .generator-heading, .source-toolbar, .result-heading { align-items: stretch; flex-direction: column; }
    .config-type { min-width: 0; }
    .source-actions { justify-content: flex-start; }
    .source-grid { grid-template-columns: 1fr; }
    .config-guide { grid-template-columns: 1fr; }
}
</style>
