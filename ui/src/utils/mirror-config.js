export const MIRROR_CONFIG_TYPES = [
    {
        label: 'K3s registries.yaml',
        value: 'k3s',
        path: '保存到 /etc/rancher/k3s/registries.yaml',
        apply: 'sudo systemctl restart k3s',
    },
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
        label: 'Containerd config v2（1.5–1.7）',
        value: 'containerd',
        path: '合并 config_path，并按注释保存 /etc/containerd/certs.d/*/hosts.toml',
        apply: 'sudo systemctl restart containerd',
    },
    {
        label: 'Containerd config v3（2.x）',
        value: 'containerd2',
        path: '合并 config_path，并按注释保存 /etc/containerd/certs.d/*/hosts.toml',
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
const isHttpUrl = (value = '') => /^http:\/\//i.test(value.trim());
const quoteToml = (value = '') => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');

export const registryHost = (originUrl = '') => {
    const value = trimSlash(originUrl.trim());
    if (!value) return '';
    try {
        return new URL(value.includes('://') ? value : `https://${value}`).host.toLowerCase();
    } catch (e) {
        return withoutProtocol(value).split('/')[0].toLowerCase();
    }
};

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
        originLocation: withoutProtocol(originUrl),
        mirrors,
    }));
};

const registryNamespace = (originUrl) => {
    return registryHost(originUrl);
};

const generateDocker = (sources) => {
    return JSON.stringify({
        'registry-mirrors': sources.map((item) => trimSlash(item.url)),
    }, null, 2);
};

const generatePodman = (sources) => {
    const groups = groupSourcesByOrigin(sources);
    if (!groups.length) return '';
    const registries = groups.map(({ originUrl, originLocation, mirrors }) => {
        const mirrorConfig = mirrors.map((item) => `[[registry.mirror]]
location = "${quoteToml(withoutProtocol(item.url))}"
insecure = ${isHttpUrl(item.url)}`).join('\n\n');
        return `[[registry]]
prefix = "${quoteToml(originLocation)}"
location = "${quoteToml(originLocation)}"
insecure = ${isHttpUrl(originUrl)}

${mirrorConfig}`;
    }).join('\n\n');
    return registries;
};

const generateHostsToml = (sources) => {
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

const generateContainerd = (sources, majorVersion) => {
    const hostsConfig = generateHostsToml(sources);
    if (!hostsConfig) return '';
    const plugin = majorVersion >= 2
        ? 'io.containerd.cri.v1.images'
        : 'io.containerd.grpc.v1.cri';
    return `# 1. 将以下片段合并到 /etc/containerd/config.toml
[plugins."${plugin}".registry]
  config_path = "/etc/containerd/certs.d"

# 2. 将以下内容按文件注释分别保存
${hostsConfig}`;
};

const generateNerdctl = (sources) => generateHostsToml(sources);

const generateK3s = (sources) => {
    const groups = new Map();
    sources.forEach((source) => {
        const namespace = registryNamespace(source.origin?.trim() || '');
        if (!namespace) return;
        if (!groups.has(namespace)) groups.set(namespace, []);
        groups.get(namespace).push(source);
    });
    if (!groups.size) return '';

    const registries = Array.from(groups, ([namespace, mirrors]) => {
        const endpoints = mirrors
            .map((item) => `      - ${JSON.stringify(trimSlash(item.url))}`)
            .join('\n');
        return `  ${JSON.stringify(namespace)}:
    endpoint:
${endpoints}`;
    }).join('\n');
    return `mirrors:
${registries}`;
};

export const generateMirrorConfig = (type, sources) => {
    if (!sources.length) return '';
    const generators = {
        docker: generateDocker,
        podman: generatePodman,
        containerd: (items) => generateContainerd(items, 1),
        containerd2: (items) => generateContainerd(items, 2),
        nerdctl: generateNerdctl,
        k3s: generateK3s,
    };
    return (generators[type] || generateDocker)(sources);
};
