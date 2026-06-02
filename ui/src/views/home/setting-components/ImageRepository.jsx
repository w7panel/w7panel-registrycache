import { ref,watch } from "vue"

export default {
    props: ['data'],
    setup(props,{ expose }){
        const form = ref({
            server_url_pre: '',
            server_url_after: '',
            username: '',
            password: '',
            // cache_namespace_suffix: '',
        })
        watch(()=>props.data.cache_registry, (v)=>{
            if(!v){return;}
            form.value = {
                ...form.value,
                ...v,
            }
            form.value.server_url_pre = form.value?.server_url?.match(/^https?:\/\//)?.[0] || 'http://';
            form.value.server_url_after = form.value?.server_url?.replace(/^https?:\/\//,'');
        },{
            immediate: true
        })

        expose({ form });
        
        return ()=>(<>
            <el-form
                model={form.value}
                label-width="auto"
                class="padding-20"
            >
                <el-form-item label="镜像仓库地址">
                    <el-input
                        modelValue={form.value.server_url_after}
                        onUpdate:modelValue={v => form.value.server_url_after = v}
                        placeholder="请输入"
                        v-slots={{
                            prepend: (scope)=>(<el-select
                                style="width:100px;"
                                placeholder="请选择"
                                modelValue={form.value.server_url_pre}
                                onUpdate:modelValue={v => form.value.server_url_pre = v}
                            >
                                <el-option label="http://" value="http://"></el-option>
                                <el-option label="https://" value="https://"></el-option>
                            </el-select>)
                        }}
                    />
                </el-form-item>
                <el-form-item label="用户名">
                    <el-input
                        modelValue={form.value.username}
                        onUpdate:modelValue={v => form.value.username = v}
                        placeholder="请输入"
                    />
                </el-form-item>
                <el-form-item label="密码">
                    <el-input
                        modelValue={form.value.password}
                        onUpdate:modelValue={v => form.value.password = v}
                        placeholder="请输入"
                        type="password"
                    />
                </el-form-item>
                {/* <el-form-item
                    label="镜像Namespace后缀"
                    v-slots={{
                        label: () => (
                            <span class="df ai-c">
                                <span>镜像Namespace后缀</span>
                                <el-popover
                                    content="同步镜像到缓存仓库,会给镜像地址中的 namespace 添加后缀， 例如 docker pull {domain}/library/xxx:xxx, 同步到仓库中的镜像地址是 {domain}/library-{后缀}/xxx:xxx"
                                    placement="top-start"
                                    width="400"
                                    v-slots={{
                                        reference: ()=>(
                                            <el-icon class="c-99 ml-4 fs-14"><QuestionFilled /></el-icon>
                                        )
                                    }}
                                >
                                </el-popover>
                            </span>
                        )
                    }}
                >
                    <el-input
                        modelValue={form.value.cache_namespace_suffix}
                        onUpdate:modelValue={v => form.value.cache_namespace_suffix = v}
                        placeholder="请输入"
                    />
                </el-form-item> */}
            </el-form>
        </>)
    }
}