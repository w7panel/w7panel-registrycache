import { ref, watch } from "vue"
import { Plus } from '@element-plus/icons-vue'

export default {
    props: ['data'], 
    setup(props, {expose}){
        const form = ref({});
        const setProxy = ref({
            show: false,
            index: 0,
            server_url: '',
            port: '',
        });

        watch(()=>props.data.cache_rules, (v)=>{
            if(!v){return}
            form.value.cache_rules = [...v] || [];
        },{
            immediate: true
        })
        watch(()=>props.data.registry_sources, (v)=>{
            if(!v){return}
            form.value.registry_sources = v?.map(i=>{
                return {
                    ...i,
                    server_url_pre: i?.server_url?.match(/^https?:\/\//)?.[0] || 'http://',
                    server_url_after: i?.server_url?.replace(/^https?:\/\//,''),
                }
            }) || [];
        },{
            immediate: true
        })

        expose({ form })

        return ()=><div className="df df-c">
            <div className="b">缓存规则</div>
            <table class="com-table mt-16 mb-30">
                <tr>
                    <td>是否缓存</td>
                    <td>
                        <el-tooltip content="0值为永不过期">
                            <span>
                                <span class="cursor va-middle">缓存时间(分钟)</span>
                                <el-icon class="ml-4 va-middle" color="#999"><WarningFilled /></el-icon>
                            </span>
                        </el-tooltip>
                    </td>
                    <td>镜像地址匹配</td>
                    <td>指定镜像仓库源</td>
                    <td>
                        <el-tooltip content="权重值越小优先级越高">
                            <span>
                                <span class="cursor va-middle">权重</span>
                                <el-icon class="ml-4 va-middle" color="#999"><WarningFilled /></el-icon>
                            </span>
                        </el-tooltip>
                    </td>
                    <td>分布下载</td>
                    <td>操作</td>
                </tr>
                {
                    form.value?.cache_rules?.map((i,index)=><tr>
                        <td>
                            <el-switch
                                modelValue={i.enable}
                                onUpdate:modelValue={v => i.enable = v}
                            ></el-switch>
                        </td>
                        <td>
                            <el-input
                                modelValue={i.cache_ttl}
                                onUpdate:modelValue={v => i.cache_ttl = v}
                                placeholder="请输入"
                                style="width:100px;"
                            ></el-input>
                        </td>
                        <td>
                            <div className="df">
                                <el-select
                                    modelValue={i.cache_type}
                                    onUpdate:modelValue={v => i.cache_type = v}
                                    placeholder="请选择"
                                    style="width:120px;"
                                    class="next-input-select"
                                >
                                    <el-option label="全部" value="all"></el-option>
                                    <el-option label="精准匹配" value="fix"></el-option>
                                    <el-option label="前缀匹配" value="prefix"></el-option>
                                    <el-option label="正则匹配" value="regex"></el-option>
                                </el-select>
                                <el-input-tag
                                    modelValue={i.repository_name}
                                    onUpdate:modelValue={v =>i.repository_name = v}
                                    placeholder="请输入"
                                    disabled={i.cache_type=='all'}
                                    class="pre-select-input"
                                    style="width:215px;"
                                />
                            </div>
                        </td>
                        <td>
                            <el-select
                                modelValue={i.assign_registry}
                                onUpdate:modelValue={v => i.assign_registry = v}
                                placeholder="请选择"
                            >
                                {
                                    form.value?.registry_sources?.map(i=>(<el-option label={i.server_url_pre+i.server_url_after} value={i.server_url_pre+i.server_url_after}></el-option>))
                                }
                            </el-select>
                        </td>
                        <td>
                            <el-input
                                modelValue={i.weight}
                                onUpdate:modelValue={v => i.weight = v}
                                placeholder="请输入"
                                type="number"
                                style="width:70px;"
                            ></el-input>
                        </td>
                        <td>
                            <el-switch
                                modelValue={i.distributed_cache}
                                onUpdate:modelValue={v => i.distributed_cache = v}
                            ></el-switch>
                        </td>
                        <td>
                            <span className="c-blue cursor" onClick={()=>{form.value.cache_rules.splice(index,1)}}>删除</span>
                        </td>
                    </tr>)
                }
                <tr className="bgrow">
                    <td colSpan={9} className="txt-c cursor" onClick={()=>{
                        let o = {
                            cache_type: 'all',
                            cache_ttl: 0,
                            enable: false,
                            repository_name: [],
                            weight: 0,
                            assign_registry: '',
                            distributed_cache: false,
                        }
                        if(form.value?.cache_rules){
                            form.value.cache_rules.push(o);
                        }else{
                            form.value.cache_rules = [o];
                        }
                    }}>
                        <div class="df ai-c jc-c">
                            <el-icon class="c-99"><Plus /></el-icon>
                            <span class="c-99 ml-4">新增</span>
                        </div>
                    </td>
                </tr>
            </table>
            <div className="b">镜像仓库源配置</div>
            <table className="com-table mt-16">
                <tr>
                    <td>仓库地址</td>
                    {/* <td>用户名</td>
                    <td>密码</td> */}
                    <td>
                        <el-tooltip content="权重值越小优先级越高">
                            <span>
                                <span class="cursor va-middle">权重</span>
                                <el-icon class="ml-4 va-middle" color="#999"><WarningFilled /></el-icon>
                            </span>
                        </el-tooltip>
                    </td>
                    <td>操作</td>
                </tr>
                {
                    form.value?.registry_sources?.map((i,index)=><tr>
                        <td>
                            <el-input
                                modelValue={i.server_url_after}
                                onUpdate:modelValue={v => i.server_url_after = v}
                                placeholder="请输入"
                                v-slots={{
                                    prepend: (scope)=>(<el-select
                                        style="width:100px;"
                                        placeholder="请选择"
                                        modelValue={i.server_url_pre}
                                        onUpdate:modelValue={v => i.server_url_pre = v}
                                    >
                                        <el-option label="http://" value="http://"></el-option>
                                        <el-option label="https://" value="https://"></el-option>
                                    </el-select>)
                                }}
                            ></el-input>
                        </td>
                        {/* <td>
                            <el-input
                                modelValue={i.username}
                                onUpdate:modelValue={v => i.username = v}
                                placeholder="请输入"
                            ></el-input>
                        </td>
                        <td>
                            <el-input
                                modelValue={i.password}
                                onUpdate:modelValue={v => i.password = v}
                                placeholder="请输入"
                                type="password"
                            ></el-input>
                        </td> */}
                        <td>
                            <el-input
                                modelValue={i.weight}
                                onUpdate:modelValue={v => i.weight = v}
                                placeholder="请输入"
                                style="width:120px;"
                                type="number"
                            ></el-input>
                        </td>
                        <td>
                            <span className="c-blue cursor" onClick={()=>{
                                setProxy.value = {
                                    show: true,
                                    index: index,
                                    server_url: i?.proxy?.server_url || '',
                                    port: i?.proxy?.port || '',
                                    username: i?.username || '',
                                    password: i?.password || '',
                                }
                            }}>设置</span>
                            <span className="c-blue cursor ml-10" onClick={()=>{form.value.registry_sources.splice(index,1)}}>删除</span>
                        </td>
                    </tr>)
                }
                <tr className="bgrow">
                    <td colSpan={9} className="txt-c cursor c-99" onClick={()=>{
                        let o = {
                            server_url_pre: 'http://',
                            server_url_after: '',
                            username: '',
                            password: '',
                            weight: '',
                            proxy: {
                                server_url: '',
                                port: '',
                            }
                        }
                        if(form.value?.registry_sources){
                            form.value.registry_sources.push(o);
                        }else{
                            form.value.registry_sources = [o];
                        }
                    }}>
                        <div class="df ai-c jc-c">
                            <el-icon class="c-99"><Plus /></el-icon>
                            <span class="c-99 ml-4">新增</span>
                        </div>
                    </td>
                </tr>
            </table>
            
            <el-dialog
                modelValue={setProxy.value.show}
                onUpdate:modelValue={v => setProxy.value.show = v}
                title="设置"
                width="600"
                v-slots={{
                    footer: ()=>(<div className="df ai-c jc-e">
                        <el-button onClick={()=>{setProxy.value.show=false;}}>取消</el-button>
                        <el-button onClick={()=>{
                            form.value.registry_sources[setProxy.value.index].proxy = {
                                server_url: setProxy.value.server_url,
                                port: setProxy.value.port,
                            }
                            form.value.registry_sources[setProxy.value.index].username = setProxy.value.username;
                            form.value.registry_sources[setProxy.value.index].password = setProxy.value.password;
                            setProxy.value.show = false;
                        }} type="primary">确定</el-button>
                    </div>)
                }}
            >
                <el-form
                    model={setProxy.value}
                    label-width="auto"
                    class="padding-20"
                >
                    <div className="b mb-20">访问代理</div>
                    <el-form-item label="代理地址">
                        <el-input
                            modelValue={setProxy.value.server_url}
                            onUpdate:modelValue={v => setProxy.value.server_url = v}
                            placeholder="请输入"
                        />
                    </el-form-item>
                    <el-form-item label="端口">
                        <el-input
                            modelValue={setProxy.value.port}
                            onUpdate:modelValue={v => setProxy.value.port = v}
                            placeholder="请输入"
                            type="number"
                        />
                    </el-form-item>
                    <div className="b mb-20 mt-30">镜像仓库权限</div>
                    <el-form-item label="用户名">
                        <el-input
                            modelValue={setProxy.value.username}
                            onUpdate:modelValue={v => setProxy.value.username = v}
                            placeholder="请输入"
                        ></el-input>
                    </el-form-item>
                    <el-form-item label="密码">
                        <el-input
                            modelValue={setProxy.value.password}
                            onUpdate:modelValue={v => setProxy.value.password = v}
                            placeholder="请输入"
                            type="password"
                        ></el-input>
                    </el-form-item>
                </el-form>
            </el-dialog>
        </div>
    }
}
