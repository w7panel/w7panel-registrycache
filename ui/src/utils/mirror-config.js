export const MIRROR_CONFIG_TYPES = [
    {
        label: 'Docker daemon.json',
        value: 'docker',
        path: '合并到 /etc/docker/daemon.json',
        apply: 'sudo systemctl restart docker',
    },
    {
        label: 'Podman registries.conf',
        value: 'podman',
        path: '保存到 /etc/containers/registries.conf.d/w7-mirrors.conf',
        apply: '无需重启，后续 podman pull 自动生效',
    },
    {
        label: 'Containerd config.toml',
        value: 'containerd',
        path: '合并到 /etc/containerd/config.toml',
        apply: 'sudo systemctl restart containerd',
    },
    {
        label: 'Nerdctl hosts.toml',
        value: 'nerdctl',
        path: '按配置中的注释分别保存 hosts.toml',
        apply: '无需重启，后续 nerdctl pull 自动生效',
    },
];

const trimSlash = (value = '') => value.replace(/\/+$/, '');
const withoutProtocol = (value = '') => trimSlash(value).replace(/^https?:\/\//i, '');
const quoteToml = (value = '') => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
const quoteTomlArray = (values) => values.map((value) => `"${quoteToml(value)}"`).join(', ');
const DOCKER_HUB_HOSTS = new Set([
    'docker.io',
    'index.docker.io',
    'registry-1.docker.io',
    'registry.hub.docker.com',
]);

export const registryHost = (originUrl = '') => {
    const value = trimSlash(originUrl.trim());
    if (!value) return '';
    try {
        return new URL(value.includes('://') ? value : `https://${value}`).host.toLowerCase();
    } catch (e) {
        return withoutProtocol(value).split('/')[0].toLowerCase();
    }
};

export const isDockerHubOrigin = (originUrl = '') => DOCKER_HUB_HOSTS.has(registryHost(originUrl));

export const groupMirrorSources = (sources = []) => {
    const groups = new Map();
    sources.forEach((source) => {
        const origin = trimSlash(source.origin?.trim() || '');
        if (!origin) return;
        if (!groups.has(origin)) groups.set(origin, []);
        groups.get(origin).push(source);
    });
    return Array.from(groups, ([origin, mirrors]) => ({ origin, mirrors }));
};

const groupSourcesByOrigin = (sources) => {
    return groupMirrorSources(sources).map(({ origin: originUrl, mirrors }) => ({
        originUrl,
        originLocation: isDockerHubOrigin(originUrl) ? 'docker.io' : withoutProtocol(originUrl),
        mirrors,
    }));
};

const registryNamespace = (originUrl) => {
    return isDockerHubOrigin(originUrl) ? 'docker.io' : registryHost(originUrl);
};

const generateDocker = (sources) => {
    return JSON.stringify({
        'registry-mirrors': sources.map((item) => trimSlash(item.url)),
    }, null, 2);
};

const generatePodman = (sources) => {
    const groups = groupSourcesByOrigin(sources);
    if (!groups.length) return '';
    const registries = groups.map(({ originLocation, mirrors }) => {
        const mirrorConfig = mirrors.map((item) => `[[registry.mirror]]
location = "${quoteToml(withoutProtocol(item.url))}"
insecure = false`).join('\n\n');
        return `[[registry]]
prefix = "${quoteToml(originLocation)}"
location = "${quoteToml(originLocation)}"

${mirrorConfig}`;
    }).join('\n\n');
    return registries;
};

const generateContainerd = (sources) => {
    const groups = groupSourcesByOrigin(sources);
    if (!groups.length) return '';
    const registries = groups.map(({ originUrl, mirrors }) => {
        const namespace = registryNamespace(originUrl);
        const endpoints = quoteTomlArray(mirrors.map((item) => trimSlash(item.url)));
        return `[plugins."io.containerd.grpc.v1.cri".registry.mirrors."${quoteToml(namespace)}"]
  endpoint = [${endpoints}]`;
    }).join('\n\n');
    return `version = 2

${registries}`;
};

const generateNerdctl = (sources) => {
    const groups = groupSourcesByOrigin(sources);
    if (!groups.length) return '';
    return groups.map(({ originUrl, mirrors }) => {
        const hosts = mirrors.map((item) => `[host."${quoteToml(trimSlash(item.url))}"]
  capabilities = ["pull", "resolve"]`).join('\n\n');
        return `# 保存为 /etc/containerd/certs.d/${registryNamespace(originUrl)}/hosts.toml
server = "${quoteToml(originUrl)}"

${hosts}`;
    }).join('\n\n# ------------------------------\n\n');
};

export const generateMirrorConfig = (type, sources) => {
    if (!sources.length) return '';
    const generators = {
        docker: generateDocker,
        podman: generatePodman,
        containerd: generateContainerd,
        nerdctl: generateNerdctl,
    };
    return (generators[type] || generateDocker)(sources);
};
