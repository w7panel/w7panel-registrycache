import axios from 'axios'
import { ElNotification } from 'element-plus'

// 请求拦截器
axios.defaults.baseURL = window?.$wujie?.props?.url || '';

axios.interceptors.request.use(
    (config) => {
        config.headers = config.headers || {};
        
        // const token = window.$wujie?.props?.OAUTH_TOKEN || 'qdpslkgkok';
        // if(token){
        //     config.headers.Authorization = `Bearer ${token}`;
        // }
        // if(config?.customToken){
        //     config.headers.Authorization = `Bearer ${config.customToken}`;
        // }
        config.headers.authorizationx = 'bearer ' + window.$wujie?.props?.paneltoken;
        // 测试
        // config.headers.authorizationx = 'bearer ' + `eyJhbGciOiJSUzI1NiIsImtpZCI6IkRUc1FoNVR0Q1Z6SDBmc1dMZkxUTmd0MEtZSTlpajZzNXJkX3hrWHloaTQifQ.eyJhdWQiOlsiYWRtaW4iLCJmb3VuZGVyIiwiNDkwMjA5IiwiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiLCJrM3MiXSwiZXhwIjoxNzc4NTc4NTYyLCJpYXQiOjE3Nzg1NzQ5NjIsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiZWU0YmYyMjAtMjEzMS00YWNmLTg4MWItZGY4YzI5OTZiM2UwIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImFkbWluIiwidWlkIjoiMDY4ZTFhOWEtMjE3Yy00NDBlLThjZGUtNTVhMzFkNzQ2YjcyIn19LCJuYmYiOjE3Nzg1NzQ5NjIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmFkbWluIn0.Xnaaj-SthjgJ87okPgmvb5wmvNIfSBjj9WzvpLwAHe_yobfZTI-wiYj3VRtY6KK1FLWDi3iDt4UjLSCKAl-Djq4wAi7c_EgjT8QWRn1psRwIZieJi4ETwbzirU8uiBG3qjhCetkbXrfn6RGxXkJaIr0SjL1Fz4PgeXTEha6nj0p1Qxf2hHMHewhadgqYXSF26UIhirjs7L25R-L0FS4E4Ugivp5p30ob4aAELmnjga6f4AWzxI9fVLi3Vw_PB5VyKMCbnAMn20PeTeblupcsigYvWSKzxkIiI-KGf-v9bX-KCWKlSh0k_HWeW6FJlDQmlWJH2rb2mrf40-hq13hEoQ`;
        return config
    },
    (error) => Promise.reject(error)
)

// 响应拦截器
axios.interceptors.response.use(
    (res) => {
        if(res.status>=200 && res.status<=300){
            return res;
        }
        
        return Promise.reject(new Error(res.message || '请求失败'))
    },
    (error) => {
        if (error.config.noAlert){
            return Promise.reject(error);
        }
        if(typeof error?.response?.data?.error == 'string'){
            ElNotification({
                title: 'Error',
                message: error.response.data.error,
                type: 'error',
            })
        }
        return Promise.reject(error);
    }
)