import { ref, watch } from 'vue'
import DomainParseAlert from './domainParseAlert'
import { Plus, Delete } from '@element-plus/icons-vue'

export default {
    props: ['data'],
    setup(props, { expose }) {
        const form = ref({
            domains: [],
        })

        watch(() => props.data, (v) => {
            if (!v) return
            form.value = {
                ...form.value,
                ...v,
            }
            if (!form.value.domains || form.value.domains.length === 0) {
                form.value.domains = [{
                    url_after: '',
                    auto_https: true,
                }]
            }
        }, {
            immediate: true
        })

        const addDomain = () => {
            form.value.domains.push({
                url_after: '',
                auto_https: true,
            })
        }

        const removeDomain = (index) => {
            form.value.domains.splice(index, 1)
        }

        expose({ form })

        return () => (<>
            <DomainParseAlert />
            <el-form
                model={form.value}
                label-width="auto"
                class="padding-20"
            >
                <el-form-item label="域名">
                    <div class="df df-c" style="flex:1;">
                        {form.value.domains.map((item, index) => (
                            <div key={index} class="df ai-c mb-10">
                                <el-input
                                    modelValue={item.url_after}
                                    onUpdate:modelValue={v => item.url_after = v}
                                    placeholder="请输入域名"
                                    v-slots={{
                                        prepend: () => (
                                            <span>{item.auto_https ? 'https://' : 'http://'}</span>
                                        )
                                    }}
                                />
                                <el-checkbox
                                    modelValue={item.auto_https}
                                    onUpdate:modelValue={v => item.auto_https = v}
                                    class="ml-10 df-s0"
                                >自动https</el-checkbox>
                                {form.value.domains.length > 1 && (
                                    <el-icon
                                        class="ml-10 cursor c-red df-s0"
                                        onClick={() => removeDomain(index)}
                                    >
                                        <Delete />
                                    </el-icon>
                                )}
                            </div>
                        ))}
                        <el-button
                            type="primary"
                            plain
                            onClick={addDomain}
                            class="w-per-100 mt-10"
                            v-slots={{
                                icon: () => <el-icon><Plus /></el-icon>
                            }}
                        >添加域名</el-button>
                    </div>
                </el-form-item>
            </el-form>
        </>)
    }
}
