import { ref, onMounted } from 'vue'
import { k8sproxy } from '@/utils/api'

export default {
    setup() {
        const domainParse = ref({
            exist: false,
            type: 'A',
            cname: '',
            ips: '',
        })

        const init = () => {
            const namespaceActive = 'default'
            k8sproxy.get('/api/v1/namespaces/' + namespaceActive + '/configmaps/domain-parse', {
                baseURL: '',
                noAlert: true,
            }).then(res => {
                domainParse.value = {
                    exist: true,
                    type: res.data?.data?.type || 'A',
                    cname: res.data?.data?.cname || '',
                    ips: res.data?.data?.ips || '',
                }
            }).catch(() => {})
        }

        onMounted(() => {
            init()
        })

        return () => (
            <div>
                {domainParse.value.exist && (
                    <el-alert
                        title="添加域名后，请在您持有域名的DNS解析后台添加对应的域名解析记录："
                        type="primary"
                        show-icon
                        closable={false}
                        class="mt-20"
                    >
                        <span>记录类型：</span>
                        <span>{domainParse.value.type}，</span>
                        <span>记录值：</span>
                        <span>{domainParse.value.type === 'A' ? domainParse.value.ips : domainParse.value.cname}</span>
                        {domainParse.value.type === 'A' && domainParse.value.ips.includes(',') && (
                            <span>（IP任选一个，解析功能支持也可添加多条记录）</span>
                        )}
                    </el-alert>
                )}
            </div>
        )
    }
}
