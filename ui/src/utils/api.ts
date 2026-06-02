import axios from 'axios';

const K8S_PROXY_PREFIX = '/k8s-proxy';
const PANEL_API_PREFIX = '/panel-api/v1';

export const k8sproxy = {
    get: (path: string, config?: any) => axios.get(`${K8S_PROXY_PREFIX}${path}`, config),
    post: (path: string, data?: any, config?: any) => axios.post(`${K8S_PROXY_PREFIX}${path}`, data, config),
    patch: (path: string, data?: any, config?: any) => axios.patch(`${K8S_PROXY_PREFIX}${path}`, data, config),
    put: (path: string, data?: any, config?: any) => axios.put(`${K8S_PROXY_PREFIX}${path}`, data, config),
    delete: (path: string, config?: any) => axios.delete(`${K8S_PROXY_PREFIX}${path}`, config),
};

export const panelApi = {
    get: (path: string, config?: any) => axios.get(`${PANEL_API_PREFIX}${path}`, config),
    post: (path: string, data?: any, config?: any) => axios.post(`${PANEL_API_PREFIX}${path}`, data, config),
    patch: (path: string, data?: any, config?: any) => axios.patch(`${PANEL_API_PREFIX}${path}`, data, config),
    put: (path: string, data?: any, config?: any) => axios.put(`${PANEL_API_PREFIX}${path}`, data, config),
    delete: (path: string, config?: any) => axios.delete(`${PANEL_API_PREFIX}${path}`, config),
};
