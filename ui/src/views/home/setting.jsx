import { ref,onMounted } from 'vue';
import ImageCache from './setting-components/ImageCache';
import ImageRepository from './setting-components/ImageRepository';
import axios from 'axios';
import { k8sproxy } from '@/utils/api';
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox,ElMessage } from 'element-plus'
import { Back } from '@element-plus/icons-vue';

export default {
    setup(){
        const route = useRoute();
        const router = useRouter();
        const appData = ref({
            cache_registry: {},
            cache_rules: [],
            registry_sources: [],
        });

        // const open = ref(false);
        const tabsActive = ref('1');
        const refCache = ref(null);
        const refRepository = ref(null);

        const group = route.query.group?.replace(/\/$/,'');
        const ingress_name = route.query.ingress_name;
        const fromList = route.query.fromList;

        // const ingressData = ref({});
        // const imageCacheApp = ref({});

        const getData = ()=>{
            if(!group){return}
            axios.post('/api/setting/get',{
                group: group
            }).then(res=>{
                let data = res?.data?.data || {};
                appData.value = data;
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

        const submit = ()=>{
            let o = {
                group: group,
                cache_storage_registry: {
                    ...refRepository.value.form,
                    server_url: refRepository.value.form?.server_url_pre + refRepository.value.form?.server_url_after,
                },
                repository_cache_rules: refCache.value.form.cache_rules,
                registry_sources: refCache.value.form.registry_sources?.map(i=>{
                    return {
                        ...i,
                        server_url: i.server_url_pre + i.server_url_after,
                    }
                }),
                extra: {
                    ingress_name: ingress_name,
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

                    <ImageRepository
                        ref={refRepository}
                        data={appData.value}
                    ></ImageRepository>
                </el-tab-pane>
            </el-tabs>

            <div className='mt-20'>
                <el-button type="primary" onClick={submit}>保存</el-button>
            </div>
        </div>)
    }
}
