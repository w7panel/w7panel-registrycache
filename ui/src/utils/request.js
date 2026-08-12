import axios from 'axios'
import { ElNotification } from 'element-plus'

// 请求拦截器
axios.defaults.baseURL = window?.$wujie?.props?.url || '';

axios.interceptors.request.use(
    (config) => {
        config.headers = config.headers || {};

        const oauthToken = window.$wujie?.props?.OAUTH_TOKEN
            || window.$wujie?.props?.oauthToken
            || import.meta.env.VITE_OAUTH_TOKEN;
        if (oauthToken) {
            config.headers.Authorization = /^bearer\s+/i.test(oauthToken)
                ? oauthToken
                : `Bearer ${oauthToken}`;
        }

        const paneltoken = config.customToken || window.$wujie?.props?.paneltoken;
        if (paneltoken) {
            config.headers.authorizationx = 'bearer ' + paneltoken;
        }
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
        if (error.config?.noAlert){
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
