<template>
    <div class="public-home">
        <header class="public-header">
            <div class="header-inner">
                <button class="brand" type="button" aria-label="返回顶部" @click="scrollToSection('top')">
                    <img
                        class="brand-logo"
                        src="//cdn.w7.cc/ued/logo/logo.png?imageView2/5/w/100/h/32/format/webp"
                        alt="W7"
                        width="100"
                        height="32"
                    />
                    <span>镜像加速服务</span>
                </button>
                <nav>
                    <button type="button" @click="scrollToSection('guide')">使用说明</button>
                    <button type="button" @click="scrollToSection('generator')">配置生成器</button>
                </nav>
            </div>
        </header>

        <main ref="top">
            <section class="hero">
                <div class="hero-glow hero-glow-one" />
                <div class="hero-glow hero-glow-two" />
                <div class="hero-content">
                    <span class="hero-badge">CONTAINER MIRROR ACCELERATION</span>
                    <h1>更简单、更稳定地<br />获取容器镜像</h1>
                    <p>快速生成 Docker、Podman、Containerd、Nerdctl 和 K3s 配置。</p>
                    <div class="hero-actions">
                        <button class="primary-action" type="button" @click="scrollToSection('generator')">生成配置</button>
                        <button class="secondary-action" type="button" @click="scrollToSection('guide')">查看使用说明</button>
                    </div>
                </div>
            </section>

            <div class="page-container">
                <el-skeleton v-if="loading" :rows="8" animated class="content-card" />
                <section v-else ref="guide" class="content-card guide-section">
                    <div class="section-title">
                        <span>GET STARTED</span>
                        <h2>镜像加速使用说明</h2>
                        <p>先确认镜像所属仓库，再选择适用源站一致的加速地址。拉取时只替换仓库域名，镜像路径和标签保持不变。</p>
                    </div>

                    <div class="source-table-wrap">
                        <table v-if="sources.length" class="source-table">
                            <thead>
                                <tr>
                                    <th>适用源站</th>
                                    <th>加速地址</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="source in sources" :key="source.id">
                                    <td><code>{{ source.origin || '未标注' }}</code></td>
                                    <td><code>{{ source.url }}</code></td>
                                </tr>
                            </tbody>
                        </table>
                        <el-empty v-else description="暂未配置加速站点" />
                    </div>

                    <div class="examples">
                        <article>
                            <span class="step-number">01</span>
                            <h3>选择匹配的加速节点</h3>
                            <p>原镜像所属仓库必须与节点的适用源站一致，不同源站的节点不能混用。</p>
                            <pre><code>适用源站：{{ pullExample.origin }}
加速节点：{{ pullExample.mirror }}</code></pre>
                        </article>
                        <article>
                            <span class="step-number">02</span>
                            <h3>替换仓库域名</h3>
                            <p>保留原镜像路径和标签，仅将原仓库域名替换为加速域名。</p>
                            <pre><code>原镜像：{{ pullExample.original }}
加速后：{{ pullExample.accelerated }}</code></pre>
                        </article>
                    </div>
                </section>

                <div ref="generator" class="generator-anchor">
                    <MirrorConfigGenerator :sources="sources" />
                </div>

                <section v-if="pageSetting.markdown.trim()" class="content-card custom-content">
                    <div class="markdown-content" v-html="customContentHtml" />
                </section>
            </div>
        </main>

        <footer class="public-footer">
            <div class="footer-inner">
                <component
                    :is="copyrightLink ? 'a' : 'span'"
                    v-if="pageSetting.copyright"
                    class="footer-copyright"
                    :href="copyrightLink || undefined"
                    target="_blank"
                    rel="noopener noreferrer"
                >{{ pageSetting.copyright }}</component>
                <div class="footer-records">
                    <component
                        :is="icpBeianLink ? 'a' : 'span'"
                        v-if="pageSetting.icp_number"
                        :href="icpBeianLink || undefined"
                        target="_blank"
                        rel="noopener noreferrer"
                    >{{ pageSetting.icp_number }}</component>
                    <component
                        :is="policeBeianLink ? 'a' : 'span'"
                        v-if="pageSetting.police_number"
                        :href="policeBeianLink || undefined"
                        target="_blank"
                        rel="noopener noreferrer"
                    >{{ pageSetting.police_number }}</component>
                </div>
            </div>
        </footer>
    </div>
</template>

<script>
import MirrorConfigGenerator from '../../components/mirror-config-generator.vue';
import { getPublicSiteList, responseData } from '../../api/config';
import { markdownToHtml } from '../../utils/markdown';

const ICP_BEIAN_URL = 'https://beian.miit.gov.cn/';
const POLICE_BEIAN_URL_PREFIX = 'https://www.beian.gov.cn/portal/registerSystemInfo?recordcode=';
const DEFAULT_ORIGIN_URL = 'https://docker.io';

const safeHttpLink = (value) => {
    if (!value) return '';
    try {
        const url = new URL(value);
        return ['http:', 'https:'].includes(url.protocol) ? url.href : '';
    } catch (e) {
        return '';
    }
};

const buildPoliceBeianUrl = (name = '') => {
    const recordCode = String(name).match(/\d/g)?.join('') || '';
    return recordCode ? `${POLICE_BEIAN_URL_PREFIX}${recordCode}` : '';
};

const createPageSetting = () => ({
    markdown: '', icp_number: '', icp_link: '', police_number: '', police_link: '', copyright: '', copyright_link: '',
});

const withoutProtocol = (value = '') => value.trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');

const buildPullExample = (source = {}) => {
    const mirrorDomain = source.domain || 'mirror.example.com';

    const originLocation = withoutProtocol(source.origin) || 'registry.example.com';
    return {
        origin: source.origin || 'https://registry.example.com',
        mirror: source.url || 'https://mirror.example.com',
        original: `docker pull ${originLocation}/namespace/image:tag`,
        accelerated: `docker pull ${mirrorDomain}/namespace/image:tag`,
    };
};

export default {
    name: 'PublicHome',
    components: { MirrorConfigGenerator },
    data() {
        return { loading: true, sources: [], pageSetting: createPageSetting() };
    },
    computed: {
        customContentHtml() {
            return markdownToHtml(this.pageSetting.markdown);
        },
        pullExample() {
            const exampleSource = this.sources.find((item) => item.origin?.trim())
                || this.sources[0];
            return buildPullExample(exampleSource);
        },
        icpBeianLink() {
            return this.pageSetting.icp_link ? safeHttpLink(this.pageSetting.icp_link) : ICP_BEIAN_URL;
        },
        policeBeianLink() {
            return this.pageSetting.police_link
                ? safeHttpLink(this.pageSetting.police_link)
                : buildPoliceBeianUrl(this.pageSetting.police_number);
        },
        copyrightLink() {
            return safeHttpLink(this.pageSetting.copyright_link);
        },
    },
    created() {
        this.load();
    },
    methods: {
        scrollToSection(section) {
            this.$refs[section]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
        },
        async load() {
            try {
                const data = responseData(await getPublicSiteList(true));
                this.sources = this.normalizeSources(data);
                this.pageSetting = {
                    ...createPageSetting(),
                    ...(data?.global?.extra?.page_setting || {}),
                };
            } catch (e) {
                // 首页保留默认内容，接口错误由页面空状态承接。
            }
            this.loading = false;
        },
        normalizeSources(data) {
            const sourceMap = new Map();
            Object.entries(data || {}).forEach(([group, setting]) => {
                if (group === 'global') return;
                const origin = setting?.origin_registry?.server_url?.trim() || DEFAULT_ORIGIN_URL;
                group.split(',').map((item) => item.trim()).filter(Boolean).forEach((domain) => {
                    const protocol = domain === window.location.host ? window.location.protocol : 'https:';
                    sourceMap.set(domain, {
                        id: domain,
                        name: domain,
                        domain,
                        url: `${protocol}//${domain}`,
                        origin,
                    });
                });
            });
            return Array.from(sourceMap.values());
        },
    },
};
</script>

<style scoped>
.public-home { min-height: 100vh; color: #202a3d; background: #f4f7fb; font-family: Inter, "PingFang SC", "Microsoft YaHei", sans-serif; }
.public-header { position: absolute; z-index: 10; top: 0; right: 0; left: 0; color: #fff; background: linear-gradient(180deg, rgb(0 0 0 / 76%) 0%, rgb(0 0 0 / 0%) 100%); }
.header-inner, .footer-inner { display: flex; width: min(1180px, calc(100% - 40px)); margin: 0 auto; align-items: center; justify-content: space-between; }
.header-inner { height: 76px; }
.brand { display: flex; padding: 0; align-items: center; gap: 10px; color: inherit; background: transparent; border: 0; cursor: pointer; font-family: inherit; font-size: 16px; font-weight: 700; }
.brand-logo { display: block; width: 100px; height: 32px; object-fit: contain; }
nav { display: flex; gap: 30px; }
nav button { padding: 0; color: rgb(255 255 255 / 76%); background: transparent; border: 0; cursor: pointer; font: inherit; }
nav button:hover { color: #fff; }
.hero { position: relative; min-height: 610px; overflow: hidden; color: #fff; background: radial-gradient(circle at 72% 28%, rgb(75 125 255 / 32%), transparent 32%), linear-gradient(125deg, #0b1530 0%, #101e44 54%, #102c5a 100%); }
.hero::after { position: absolute; right: -8%; bottom: -46%; width: 680px; height: 680px; background: repeating-linear-gradient(135deg, rgb(255 255 255 / 5%) 0 1px, transparent 1px 19px); border-radius: 50%; content: ""; }
.hero-glow { position: absolute; border-radius: 50%; filter: blur(8px); }
.hero-glow-one { top: 140px; right: 18%; width: 240px; height: 240px; background: rgb(49 112 255 / 18%); border: 1px solid rgb(107 151 255 / 22%); }
.hero-glow-two { right: 8%; bottom: 80px; width: 110px; height: 110px; background: rgb(77 221 255 / 18%); }
.hero-content { position: relative; z-index: 2; width: min(1180px, calc(100% - 40px)); margin: 0 auto; padding-top: 158px; }
.hero-badge { display: inline-block; padding: 7px 12px; color: #8eb6ff; background: rgb(50 111 255 / 13%); border: 1px solid rgb(91 140 255 / 24%); border-radius: 999px; font-size: 11px; font-weight: 700; letter-spacing: .12em; }
.hero h1 { max-width: 720px; margin: 24px 0 20px; font-size: clamp(42px, 6vw, 68px); line-height: 1.12; letter-spacing: -.035em; }
.hero p { max-width: 680px; margin: 0; color: rgb(225 234 255 / 72%); font-size: 17px; line-height: 1.8; }
.hero-actions { display: flex; margin-top: 34px; gap: 12px; }
.primary-action, .secondary-action { display: inline-flex; height: 46px; padding: 0 23px; align-items: center; justify-content: center; border: 0; border-radius: 9px; cursor: pointer; font: inherit; font-weight: 600; }
.primary-action { color: #fff !important; background: #3478f6; box-shadow: 0 12px 26px rgb(27 91 230 / 34%); }
.page-container { position: relative; z-index: 3; width: min(1180px, calc(100% - 40px)); margin: -54px auto 0; }
.content-card { padding: 42px; background: #fff; border: 1px solid #e7eaf0; border-radius: 20px; box-shadow: 0 24px 60px rgb(33 56 97 / 8%); }
.section-title span { color: #165dff; font-size: 14px; font-weight: 700; letter-spacing: .12em; }
.section-title h2 { margin: 10px 0; color: #17233d; font-size: 30px; }
.section-title p { margin: 0; color: #7a8499; font-size: 16px; line-height: 1.7; }
.source-table-wrap { margin-top: 28px; overflow: hidden; border: 1px solid #e7eaf0; border-radius: 12px; }
.source-table { width: 100%; border-collapse: collapse; }
.source-table th, .source-table td { padding: 15px 18px; text-align: left; border-bottom: 1px solid #edf0f5; }
.source-table th { color: #667085; background: #f8faff; font-size: 14px; font-weight: 600; }
.source-table tr:last-child td { border-bottom: 0; }
.source-table code { color: #475467; font-size: 14px; line-height: 1.6; }
.examples { display: grid; margin-top: 24px; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.examples article { position: relative; padding: 24px; overflow: hidden; background: #f8faff; border-radius: 14px; }
.examples h3 { margin: 0 0 8px; font-size: 17px; }
.examples p { margin: 0; color: #7a8499; font-size: 15px; line-height: 1.7; }
.examples pre { margin: 18px 0 0; padding: 14px 16px; overflow: auto; color: #d8e4ff; background: #11182a; border-radius: 8px; font-size: 14px; line-height: 1.7; }
.step-number { position: absolute; top: 16px; right: 20px; color: #dbe6fb; font-size: 30px; font-weight: 800; }
.generator-anchor { margin-top: 28px; scroll-margin-top: 24px; }
.custom-content { margin-top: 28px; }
.markdown-content { color: #344054; font-size: 16px; line-height: 1.85; }
.markdown-content :deep(h1), .markdown-content :deep(h2), .markdown-content :deep(h3) { margin-top: 1.4em; color: #17233d; }
.markdown-content :deep(h1:first-child), .markdown-content :deep(h2:first-child), .markdown-content :deep(h3:first-child) { margin-top: 0; }
.markdown-content :deep(pre) { padding: 20px; overflow: auto; color: #d8e4ff; background: #11182a; border-radius: 10px; }
.markdown-content :deep(code) { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.markdown-content :deep(blockquote) { margin-left: 0; padding: 12px 18px; color: #667085; background: #f8faff; border-left: 4px solid #165dff; }
.markdown-content :deep(a) { color: #165dff; }
.public-footer { margin-top: 72px; padding: 32px 0; color: #9ba8c2; background: #0b1429; text-align: center; }
.footer-inner { flex-direction: column; flex-wrap: wrap; justify-content: center; gap: 8px; }
.footer-records { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px 22px; }
.footer-records a, .footer-copyright { color: inherit; }
.footer-records a:hover, .footer-copyright:hover { color: #fff; }
.footer-copyright { margin-left: 0; text-align: center; }
@media (max-width: 760px) {
    nav { display: none; }
    .hero { min-height: 620px; }
    .hero-content { padding-top: 140px; }
    .content-card { padding: 24px; border-radius: 16px; }
    .source-table-wrap { overflow-x: auto; }
    .source-table { min-width: 680px; }
    .examples { grid-template-columns: 1fr; }
    .footer-inner { align-items: center; }
}
</style>
