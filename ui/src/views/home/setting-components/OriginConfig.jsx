import { ref, watch } from 'vue'

const emptyOrigin = () => ({
    server_url_pre: 'https://',
    server_url_after: '',
})

export default {
    props: ['data'],
    setup(props, { expose }) {
        const form = ref(emptyOrigin())

        watch(() => props.data.origin_registry, (value) => {
            const origin = value || {}
            form.value = {
                ...emptyOrigin(),
                server_url_pre: origin.server_url?.match(/^https?:\/\//)?.[0] || 'https://',
                server_url_after: origin.server_url?.replace(/^https?:\/\//, '') || '',
            }
        }, { immediate: true })

        expose({ form })

        return () => <div>
            <div class="mt-10 padding-20" style="background: var(--arc-color-fill-1);">
                <el-form-item label="源站地址" label-width="100px" style="margin-bottom:0;">
                    <el-input
                        modelValue={form.value.server_url_after}
                        onUpdate:modelValue={value => form.value.server_url_after = value}
                        placeholder="请输入源站地址，例如 registry-1.docker.io"
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
            </div>
        </div>
    },
}
