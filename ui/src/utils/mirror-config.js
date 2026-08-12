export const MIRROR_CONFIG_TYPES = [
    { label: 'Docker daemon.json', value: 'docker' },
    { label: 'Podman registries.conf', value: 'podman' },
    { label: 'Containerd config.toml', value: 'containerd' },
    { label: 'Nerdctl hosts.toml', value: 'nerdctl' },
];

const trimSlash = (value = '') => value.replace(/\/+$/, '');
const withoutProtocol = (value = '') => trimSlash(value).replace(/^https?:\/\//i, '');
const quoteToml = (value = '') => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
const quoteTomlArray = (values) => values.map((value) => `"${quoteToml(value)}"`).join(', ');

const groupSourcesByOrigin = (sources) => {
    const groups = new Map();
    sources.forEach((source) => {
        const originUrl = trimSlash(source.origin?.trim() || '');
        if (!originUrl) return;
        if (!groups.has(originUrl)) groups.set(originUrl, []);
        groups.get(originUrl).push(source);
    });
    return Array.from(groups, ([originUrl, mirrors]) => ({
        originUrl,
        originLocation: withoutProtocol(originUrl),
        mirrors,
    }));
};

const registryNamespace = (originUrl) => {
    try {
        return new URL(originUrl).host;
    } catch (e) {
        return withoutProtocol(originUrl).split('/')[0];
    }
};

const generateDocker = (sources) => JSON.stringify({
    'registry-mirrors': sources.map((item) => trimSlash(item.url)),
}, null, 2);

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
    return `unqualified-search-registries = [${quoteTomlArray(groups.map((item) => item.originLocation))}]

${registries}`;
};

const generateContainerd = (sources) => {
    const groups = groupSourcesByOrigin(sources);
    if (!groups.length) return '';
    const registries = groups.map(({ originLocation, mirrors }) => {
        const endpoints = quoteTomlArray(mirrors.map((item) => trimSlash(item.url)));
        return `[plugins."io.containerd.grpc.v1.cri".registry.mirrors."${quoteToml(originLocation)}"]
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
