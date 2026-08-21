import { ref, onMounted } from 'vue';
import axios from 'axios';
import { Plus, WarningFilled } from '@element-plus/icons-vue';
import { ElMessage } from 'element-plus';
import { useRouter } from 'vue-router';
import { k8sproxy } from '@/utils/api';
import CreateSite from './setting-components/createSite';

export default {
    name: 'list',
    setup() {
        const tableData = ref([]);
        const router = useRouter();
        const form = ref({
            show: false,
            data: {},
        });
        const syncStatus = ref({
            show: false,
            loading: false,
            host: '',
            list: [],
        });
        const refCreateSite = ref(null);

        const getList = () => {
            axios.post('/api/setting/list').then(res => {
                let data = res?.data?.data || {};

                tableData.value = Object.entries(data)
                    .filter(([group]) => group !== 'global')
                    .map(arr => {
                        arr[1].host = arr[0];
                        arr[1].ingress_name = arr[1]?.extra?.ingress_name || '';
                        return arr[1];
                    });
            });
        };

        const toEdit = (row) => {
            router.push(`/setting?group=${row.group}&ingress_name=${row.ingress_name}&fromList=true`);
        };

        const openPublicHome = () => {
            const route = router.resolve({ name: 'public-home' });
            window.open(route.href, '_blank', 'noopener');
        };

        const toDelete = (row) => {
            axios.post('/api/setting/del', {
                group: row.group,
            }).then(() => {
                let ingressName = row.ingress_name;
                return k8sproxy.delete(`/apis/networking.k8s.io/v1/namespaces/default/ingresses/${ingressName}`, {
                    baseURL: '',
                    customToken: window.$wujie?.props?.paneltoken,
                });
            }).then(() => {
                ElMessage.success('操作成功');
                getList();
            }).catch(() => { });
        };

        const createSite = async (formData) => {
            const backend = {
                service: {
                    name: window.$wujie?.props?.group,
                    port: { number: Number(window.$wujie?.props?.servicePort) }
                }
            };

            const domainToname = function (str) {
                return str.replace(/\*/g, 'x').replace(/(\.|\/|_)/g, '-').toLowerCase();
            };

            const createName = function (len) {
                len = len || 8;
                let s = 'abcdefghijklmnopqrstuvwxyz';
                let p = '';
                for (var i = 0; i < len; i++) {
                    p = p + s[parseInt(Math.random() * s.length)];
                }
                return p;
            };

            const domains = (formData?.domains || [])
                .map(item => ({
                    ...item,
                    url_after: item?.url_after?.trim()?.replace(/^https?:\/\//, '')?.replace(/\/+$/, ''),
                }))
                .filter(item => item.url_after);

            if (domains.length === 0) {
                ElMessage.warning('请先输入至少一个域名');
                return;
            }

            const paneltoken = window.$wujie?.props?.paneltoken;
            const uniqueDomains = Array.from(new Map(domains.map(item => [item.url_after, item])).values());

            try {
                let item = uniqueDomains[0];
                const domain = item.url_after;
                const groupDomain = uniqueDomains.map(item => item.url_after).join(',');
                const children = uniqueDomains.slice(1).map(item => ({
                    name: 'ing-' + createName(),
                    host: item.url_after,
                    autoSsl: item.auto_https,
                    sslRedirect: false,
                }));

                let ingressName = 'ing-' + createName();
                let data = {
                    apiVersion: 'networking.k8s.io/v1',
                    kind: 'Ingress',
                    metadata: {
                        name: ingressName,
                        namespace: 'default',
                        annotations: {
                            'kubernetes.io/ingress.class': 'higress',
                            'higress.io/resource-definer': 'higress',
                            ...(children.length > 0 ? { 'w7.cc/child-hosts': JSON.stringify(children) } : {}),
                        },
                        labels: {
                            'higress.io/resource-definer': 'higress',
                            app: window.$wujie?.props?.group,
                            group: window.$wujie?.props?.group,
                        },
                    },
                    spec: {
                        rules: [
                            {
                                host: domain,
                                http: {
                                    paths: [
                                        {
                                            path: '/',
                                            pathType: 'Prefix',
                                            backend,
                                        },
                                    ],
                                },
                            },
                        ],
                    },
                };

                if (item.auto_https) {
                    data.metadata.annotations['cert-manager.io/cluster-issuer'] = 'w7-letsencrypt-prod';
                    data.metadata.annotations['cert-manager.io/renew-before'] = '30m';
                    data.metadata.annotations['w7.cc/ssl-redirect'] = 'false';
                    data.spec.tls = [{
                        hosts: [domain],
                        secretName: domainToname(domain) + '-tls-secret'
                    }];
                }

                await k8sproxy.post('/apis/networking.k8s.io/v1/namespaces/default/ingresses', data, {
                    baseURL: '',
                    customToken: paneltoken,
                });

                await axios.post('/api/setting/set', {
                    group: groupDomain,
                    cache_storage_registry: {},
                    repository_cache_rules: [],
                    registry_sources: [],
                    extra: {
                        ingress_name: ingressName,
                        cache_repository: {
                            mode: 'global',
                            storage_path: '/',
                        },
                    },
                });

                form.value.show = false;
                getList();
                ElMessage.success('创建成功');
            } catch (e) { }
        };

        const toHttpsConfig = (row) => {
            let data = { domainName: row.ingress_name };
            window.$wujie?.bus?.$emit?.('domainCert', data);
            console.log('domainCert', data);
        };

        const openSyncStatus = async (row) => {
            syncStatus.value = { show: true, loading: true, host: row.host, list: [] };
            try {
                const res = await axios.get('/api/sync-status/list', { params: { host: row.host } });
                syncStatus.value.list = res?.data?.data?.list || [];
            } catch (e) {
                syncStatus.value.list = [];
            } finally {
                syncStatus.value.loading = false;
            }
        };

        const statusProgress = (item) => {
            const total = item?.expected_blobs?.length || 0;
            const completed = item?.completed_blobs?.length || 0;
            return total > 0 ? Math.min(100, Math.floor(completed * 100 / total)) : 0;
        };

        onMounted(() => {
            getList();
        });

        return () => (<div className="padding-20">
            <div className="mb-20 df jc-b ai-c df-ww" style="gap:12px;">
                <el-button
                    type="primary"
                    icon={() => <el-icon><Plus /></el-icon>}
                    onClick={async () => {
                        form.value = {
                            show: true,
                            data: {},
                        };
                    }}
                >添加站点</el-button>
            </div>
            <el-table
                data={tableData.value}
                style={{ width: '100%' }}
                class="table-header"
            >
                <el-table-column
                    prop="host"
                    label="域名"
                />
                <el-table-column
                    label="操作"
                    width="390px"
                    v-slots={{
                        default: (scope) => (<>
                            <span class="c-blue cursor" onClick={() => { toEdit(scope.row); }}>修改</span>
                            <span class="c-blue cursor ml-16" onClick={() => { openSyncStatus(scope.row); }}>查看同步状态</span>
                            <span class="c-blue cursor ml-16" onClick={() => { toHttpsConfig(scope.row); }}>https配置</span>
                            <el-popconfirm
                                title="确定要删除吗？"
                                icon={WarningFilled}
                                placement="left-start"
                                popper-class="delete-popconfirm"
                                onConfirm={() => { toDelete(scope.row); }}
                                v-slots={{
                                    reference: () => (
                                        <span class="c-blue cursor ml-16">删除</span>
                                    ),
                                    actions: ({ confirm, cancel }) => (
                                        <div class="delete-popconfirm__actions">
                                            <el-button size="small" type="danger" onClick={confirm}>确定</el-button>
                                            <el-button size="small" onClick={cancel}>取消</el-button>
                                        </div>
                                    )
                                }}
                            />
                        </>)
                    }}
                />
            </el-table>
            <el-dialog
                modelValue={form.value.show}
                onUpdate:modelValue={e => { form.value.show = e; }}
                title="添加站点"
                width="660"
                v-slots={{
                    footer: () => (
                        <div class="df ai-c jc-e">
                            <el-button onClick={() => { form.value.show = false; }}>取消</el-button>
                            <el-button type="primary" onClick={() => {
                                let currentForm = refCreateSite.value.form;
                                createSite(currentForm);
                            }}>确定</el-button>
                        </div>
                    )
                }}
            >
                <CreateSite
                    v-if={form.value.show}
                    ref={refCreateSite}
                    data={form.value.data}
                />
            </el-dialog>
            <el-dialog
                modelValue={syncStatus.value.show}
                onUpdate:modelValue={e => { syncStatus.value.show = e; }}
                title={`同步状态：${syncStatus.value.host}`}
                width="1080"
            >
                <div v-loading={syncStatus.value.loading} style="min-height: 120px;">
                    {syncStatus.value.list.length === 0 && !syncStatus.value.loading ? (
                        <el-empty description="当前没有未完成或失败的同步任务" />
                    ) : (
                        <el-table data={syncStatus.value.list} style={{ width: '100%' }} class="table-header">
                            <el-table-column prop="repository" label="目标仓库" min-width="220" />
                            <el-table-column prop="reference" label="镜像版本" width="180" />
                            <el-table-column label="状态" width="130">
                                {{ default: (scope) => (
                                    <el-tag type={scope.row.status === 'failed' ? 'danger' : 'primary'}>
                                        {scope.row.status === 'failed' ? '同步失败' : '同步中'}
                                    </el-tag>
                                ) }}
                            </el-table-column>
                            <el-table-column label="进度" min-width="280">
                                {{ default: (scope) => {
                                    const completed = scope.row.completed_blobs?.length || 0;
                                    const total = scope.row.expected_blobs?.length || 0;
                                    return <div class="sync-progress-cell" style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                                        <el-progress percentage={statusProgress(scope.row)} status={scope.row.status === 'failed' ? 'exception' : undefined} />
                                        <span>{completed}/{total}</span>
                                    </div>;
                                } }}
                            </el-table-column>
                            <el-table-column label="失败信息" min-width="240">
                                {{ default: (scope) => scope.row.error || Object.values(scope.row.failed_blobs || {}).join('; ') || '-' }}
                            </el-table-column>
                            <el-table-column label="更新时间" width="190">
                                {{ default: (scope) => scope.row.updated_at ? new Date(scope.row.updated_at).toLocaleString() : '-' }}
                            </el-table-column>
                        </el-table>
                    )}
                </div>
            </el-dialog>
        </div>);
    }
};
