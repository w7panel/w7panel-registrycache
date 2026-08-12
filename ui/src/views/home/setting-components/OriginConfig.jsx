import { ref, watch } from 'vue'

const emptyOrigin = () => ({
    server_url_pre: 'https://',
    server_url_after: '',
    username: '',
    password: '',
    proxy: {
        server_url: '',
        port: '',
    },
})

export default {
    props: ['data'],
    setup(props, { expose }) {
        const form = ref(emptyOrigin())

        watch(() => props.data.origin_registry, (value) => {
            const origin = value || {}
            form.value = {
                ...emptyOrigin(),
                ...origin,
                server_url_pre: origin.server_url?.match(/^https?:\/\//)?.[0] || 'https://',
                server_url_after: origin.server_url?.replace(/^https?:\/\//, '') || '',
                proxy: {
                    ...emptyOrigin().proxy,
                    ...(origin.proxy || {}),
                },
            }
        }, { immediate: true })

        expose({ form })

        return () => <div>
            <el-alert
                type="info"
                show-icon
                closable={false}
                class="mb-20"
                title="源站是最终兜底仓库；未指定固定镜像源时，会在其他镜像仓库源均不可用后尝试。若没有镜像仓库源，则必须配置源站。"
            />
            <el-form model={form.value} label-width="auto" class="padding-20">
                <el-form-item label="源站仓库地址">
                    <el-input
                        modelValue={form.value.server_url_after}
                        onUpdate:modelValue={value => form.value.server_url_after = value}
                        placeholder="请输入源站仓库地址，例如 registry-1.docker.io"
                        v-slots={{
                            prepend: () => <el-select
                                style="width:100px;"
                                modelValue={form.value.server_url_pre}
                                onUpdate:modelValue={value => form.value.server_url_pre = value}
                            >
                                <el-option label="https://" value="https://" />
                                <el-option label="http://" value="http://" />
                            </el-select>
                        }}
                    />
                </el-form-item>
                <el-form-item label="用户名">
                    <el-input
                        modelValue={form.value.username}
                        onUpdate:modelValue={value => form.value.username = value}
                        placeholder="请输入"
                    />
                </el-form-item>
                <el-form-item label="密码">
                    <el-input
                        modelValue={form.value.password}
                        onUpdate:modelValue={value => form.value.password = value}
                        placeholder="请输入"
                        type="password"
                        show-password
                    />
                </el-form-item>
                <el-form-item label="访问代理地址">
                    <el-input
                        modelValue={form.value.proxy.server_url}
                        onUpdate:modelValue={value => form.value.proxy.server_url = value}
                        placeholder="可选"
                    />
                </el-form-item>
                <el-form-item label="访问代理端口">
                    <el-input
                        modelValue={form.value.proxy.port}
                        onUpdate:modelValue={value => form.value.proxy.port = value}
                        placeholder="可选"
                        type="number"
                    />
                </el-form-item>
            </el-form>
        </div>
    },
}
