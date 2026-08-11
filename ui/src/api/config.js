import axios from 'axios';

const quietConfig = {
    noAlert: true,
};

const GLOBAL_GROUP = 'global';

export const responseData = (response, fallback = {}) =>
    response?.data?.data ?? fallback;

const getGlobalSetting = (quiet = false) =>
    axios.post(
        '/api/setting/get',
        { group: GLOBAL_GROUP },
        quiet ? quietConfig : undefined,
    );

const saveGlobalSetting = (setting, overrides = {}) =>
    axios.post('/api/setting/set', {
        group: GLOBAL_GROUP,
        cache_storage_registry: repositoryToCacheRegistry(
            cacheRegistryToRepository(setting.cache_registry),
        ),
        repository_cache_rules: setting.cache_rules || [],
        registry_sources: setting.registry_sources || [],
        extra: setting.extra || {},
        ...overrides,
    });

const getGlobalExtra = async (key, quiet = false) => {
    const response = await getGlobalSetting(quiet);
    const setting = responseData(response);
    return {
        data: {
            data: setting?.extra?.[key] || {},
        },
    };
};

const saveGlobalExtra = async (key, data) => {
    const response = await getGlobalSetting(true);
    const setting = responseData(response);
    return saveGlobalSetting(setting, {
        extra: {
            ...(setting.extra || {}),
            [key]: data,
        },
    });
};

const normalizeStoragePath = (value = '/') => {
    const path = value.trim() || '/';
    return path.startsWith('/') ? path : `/${path}`;
};

export const cacheRegistryToRepository = (registry = {}) => {
    const storagePath = registry.cache_namespace_prefix
        ? normalizeStoragePath(registry.cache_namespace_prefix)
        : '/';
    let repositoryUrl = (registry.server_url || '').replace(/\/+$/, '');
    if (storagePath !== '/' && repositoryUrl.endsWith(storagePath)) {
        repositoryUrl = repositoryUrl.slice(0, -storagePath.length).replace(/\/+$/, '');
    }
    return {
        repository_url: repositoryUrl,
        storage_path: storagePath,
        username: registry.username || '',
        password: registry.password || '',
    };
};

export const repositoryToCacheRegistry = (repository = {}) => {
    const repositoryUrl = (repository.repository_url || '').replace(/\/+$/, '');
    const storagePath = normalizeStoragePath(repository.storage_path);
    return {
        server_url: storagePath === '/'
            ? repositoryUrl
            : `${repositoryUrl}${storagePath}`,
        username: repository.username || '',
        password: repository.password || '',
        cache_namespace_prefix: '',
    };
};

export const getPublicSiteList = (quiet = false) =>
    axios.post('/api/setting/common-list', {}, quiet ? quietConfig : undefined);

export const getGlobalCacheRepository = async (quiet = false) => {
    const response = await getGlobalSetting(quiet);
    const setting = responseData(response);
    return {
        data: {
            data: cacheRegistryToRepository(setting.cache_registry),
        },
    };
};

export const saveGlobalCacheRepository = async (data) => {
    const response = await getGlobalSetting(true);
    const setting = responseData(response);
    return saveGlobalSetting(setting, {
        cache_storage_registry: repositoryToCacheRegistry(data),
    });
};

export const getPageSetting = (quiet = false) =>
    getGlobalExtra('page_setting', quiet);

export const savePageSetting = (data) =>
    saveGlobalExtra('page_setting', data);
