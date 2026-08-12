import { ref,onMounted } from 'vue';
import ImageCache from './setting-components/ImageCache';
import OriginConfig from './setting-components/OriginConfig';
import CacheRepositoryForm from '@/components/cache-repository-form.vue';
import axios from 'axios';
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Back } from '@element-plus/icons-vue';
import {
    cacheRegistryToRepository,
    getGlobalCacheRepository,
    repositoryToCacheRegistry,
    responseData,
} from '@/api/config';

const createRepository = () => ({
    repository_url: '',
    storage_path: '/',
    username: '',
    password: '',
});

export default {
    setup(){
        const route = useRoute();
        const router = useRouter();
        const appData = ref({
            cache_registry: {},
            cache_rules: [],
            registry_sources: [],
            origin_registry: {},
            extra: {},
        });

        // const open = ref(false);
        const tabsActive = ref('1');
        const refCache = ref(null);
        const refOrigin = ref(null);
        const extra = ref({});
        const globalRepositoryLoading = ref(true);
        const globalRepository = ref(createRepository());
        const cacheRepository = ref({
            ...createRepository(),
            mode: 'global',
        });

        const group = route.query.group?.replace(/\/$/,'');
        const ingress_name = route.query.ingress_name;
        const fromList = route.query.fromList;

        // const ingressData = ref({});
        // const imageCacheApp = ref({});

        const loadGlobalRepository = async () => {
            globalRepositoryLoading.value = true;
            try {
                globalRepository.value = {
                    ...createRepository(),
                    ...responseData(await getGlobalCacheRepository(true)),
                };
            } catch (e) {
                globalRepository.value = createRepository();
            } finally {
                globalRepositoryLoading.value = false;
            }
        };

        const normalizeRepositoryPath = () => {
            const value = cacheRepository.value.storage_path?.trim() || '/';
            cacheRepository.value.storage_path = value.startsWith('/') ? value : `/${value}`;
        };

        const getData = ()=>{
            if(!group){return}
            axios.post('/api/setting/get',{
                group: group
            }).then(res=>{
                let data = res?.data?.data || {};
                appData.value = data;
                extra.value = data.extra || {};
                const repository = cacheRegistryToRepository(data.cache_registry);
                const storedRepository = extra.value.cache_repository || {};
                cacheRepository.value = {
                    ...createRepository(),
                    ...repository,
                    mode: ['global', 'custom'].includes(storedRepository.mode)
                        ? storedRepository.mode
                        : (repository.repository_url ? 'custom' : 'global'),
                    storage_path: storedRepository.storage_path || repository.storage_path || '/',
                };
                normalizeRepositoryPath();
            })
            
            // let paneltoken = window.$wujie?.props?.paneltoken;
            // if(ingress_name){
            //     k8sproxy.get('/apis/networking.k8s.io/v1/namespaces/default/ingresses/' + ingress_name,{
            //         baseURL: '',
            //         customToken: paneltoken,
            //     }).then(res=>{
            //         // open.value = res?.data?.metadata?.annotations?.['w7.cc/registrycache'] === 'true';
            //         ingressData.value = res?.data;
            //     }).catch(()=>{})
            // }
        }

        const activeCacheRepository = () => {
            const repository = cacheRepository.value.mode === 'global'
                ? globalRepository.value
                : cacheRepository.value;
            return {
                repository_url: repository.repository_url || '',
                storage_path: cacheRepository.value.storage_path || '/',
                username: repository.username || '',
                password: repository.password || '',
            };
        };

        const updateCacheRepository = (value) => {
            cacheRepository.value = {
                ...cacheRepository.value,
                repository_url: cacheRepository.value.mode === 'custom'
                    ? value.repository_url
                    : cacheRepository.value.repository_url,
                storage_path: value.storage_path,
                username: cacheRepository.value.mode === 'custom'
                    ? value.username
                    : cacheRepository.value.username,
                password: cacheRepository.value.mode === 'custom'
                    ? value.password
                    : cacheRepository.value.password,
            };
        };

        const submit = ()=>{
            normalizeRepositoryPath();
            if (cacheRepository.value.mode === 'global' && !globalRepository.value.repository_url) {
                ElMessage.warning('请先配置全局缓存仓库');
                return;
            }
            if (cacheRepository.value.mode === 'custom' && !cacheRepository.value.repository_url) {
                ElMessage.warning('请输入镜像仓库地址');
                return;
            }

            const registrySources = (refCache.value?.form.registry_sources || [])
                .filter(item => item.server_url_after?.trim())
                .map(item => ({
                    ...item,
                    server_url: item.server_url_pre + item.server_url_after.trim().replace(/\/+$/, ''),
                }));
            const originForm = refOrigin.value?.form;
            const originHost = originForm?.server_url_after?.trim().replace(/\/+$/, '');
            const originRegistry = originHost ? {
                server_url: originForm.server_url_pre + originHost,
                username: originForm.username || '',
                password: originForm.password || '',
                weight: 0,
                proxy: originForm.proxy?.server_url ? {
                    server_url: originForm.proxy.server_url,
                    port: Number(originForm.proxy.port) || 0,
                } : null,
            } : {};

            let o = {
                group: group,
                cache_storage_registry: repositoryToCacheRegistry(activeCacheRepository()),
                repository_cache_rules: refCache.value.form.cache_rules || [],
                registry_sources: registrySources,
                origin_registry: originRegistry,
                extra: {
                    ...extra.value,
                    ingress_name: ingress_name,
                    cache_repository: {
                        mode: cacheRepository.value.mode,
                        storage_path: cacheRepository.value.storage_path,
                    },
                },
            }
            o.registry_sources.map(i=>{
                i.weight = Number(i.weight)
                if(i.proxy?.port){
                    i.proxy.port = Number(i.proxy.port)
                }
                if(!i.proxy?.server_url){
                    i.proxy = null;
                }
            })
            o.repository_cache_rules.map(i=>{
                i.repository_name = i.repository_name?.map(rn=>rn.replace(/^\//,''));
                i.weight = Number(i.weight);
                i.cache_ttl = Number(i.cache_ttl) || 0;
            })
            
            axios.post('/api/setting/set',o).then(res=>{
                ElMessage.success('操作成功');
            })
        }


        onMounted(()=>{
            loadGlobalRepository();
            getData();
        })

        return ()=>(<div class="padding-20">
            {fromList ? <div class="mb-20">
                {/* <el-page-header onBack={() => router.back()} title="返回" content="站点配置" /> */}
                    
                <div class="com-back df ai-c">
                    <span class="backbtn df ai-c" onClick={() => router.go(-1)}>
                        <el-icon class="backicon" color="#165DFF" size={20}><Back /></el-icon>
                        <span style={{ color: '#86909c', fontSize: '16px' }}>站点配置</span>
                        <span style={{ color: '#c9cdd4', padding: '0 5px', fontWeight: 900, fontSize: '16px' }}>/</span>
                        <span style={{ fontSize: '16px' }}>详情</span>
                    </span>
                </div>

            </div> : null}
            <el-tabs
                modelValue={tabsActive.value}
                onUpdate:modelValue={e => {tabsActive.value = e;} }
            >
                <el-tab-pane label="缓存镜像配置" name="1">
                    <ImageCache
                        ref={refCache}
                        data={appData.value}
                    ></ImageCache>
                    <div class="mt-30">
                        <div class="b mb-16">源站配置</div>
                        <OriginConfig
                        ref={refOrigin}
                        data={appData.value}
                        />
                    </div>
                </el-tab-pane>
                <el-tab-pane label="缓存镜像仓库配置" name="2">
                    <el-alert
                        show-icon
                        type="primary"
                        closable={false}
                        class="alert-style"
                        v-slots={{
                            title: () => (<div>关于缓存镜像仓库</div>),
                            default: () => (<ul class="alert-style-ul">
                                <li>镜像缓存服务的pull、push权限均来自下面配置的缓存镜像仓库，如果缓存镜像仓库设置为私有，会影响对镜像的拉取。</li>
                                <li>镜像仓库地址允许设置二级目录，比如w7-zpkv2-registry.default.svc.cluster.local:5000/cache, 例如实际镜像地址是 library/nginx:latest, 在缓存仓库中的镜像地址是 cache/library/nginx:latest</li>
                            </ul>)
                        }}
                    />

                    <div class="mt-20 mb-20 df ai-c jc-b df-ww" style="gap:12px;">
                        <el-radio-group
                            modelValue={cacheRepository.value.mode}
                            onUpdate:modelValue={value => { cacheRepository.value.mode = value; }}
                        >
                            <el-radio-button value="global" label="global">全局配置</el-radio-button>
                            <el-radio-button value="custom" label="custom">自定义</el-radio-button>
                        </el-radio-group>
                        <el-button onClick={() => router.push('/cache/repository')}>编辑全局配置</el-button>
                    </div>

                    {!globalRepositoryLoading.value
                        && cacheRepository.value.mode === 'global'
                        && !globalRepository.value.repository_url
                        ? <el-alert
                            title="尚未配置全局缓存仓库，请先完成全局配置"
                            type="warning"
                            show-icon
                            closable={false}
                            class="mb-20"
                        />
                        : null}

                    {globalRepositoryLoading.value
                        ? <el-skeleton rows={2} animated />
                        : <CacheRepositoryForm
                            modelValue={activeCacheRepository()}
                            repositoryDisabled={cacheRepository.value.mode === 'global'}
                            inherited={cacheRepository.value.mode === 'global'}
                            onUpdate:modelValue={updateCacheRepository}
                        />}
                </el-tab-pane>
            </el-tabs>

            <div className='mt-20'>
                <el-button type="primary" onClick={submit}>保存</el-button>
            </div>
        </div>)
    }
}
