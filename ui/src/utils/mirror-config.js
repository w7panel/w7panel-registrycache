export const MIRROR_CONFIG_TYPES = [
    { label: 'Docker daemon.json', value: 'docker' },
    { label: 'Podman registries.conf', value: 'podman' },
    { label: 'Containerd config.toml', value: 'containerd' },
    { label: 'Nerdctl hosts.toml', value: 'nerdctl' },
];

const trimSlash = (value = '') => value.replace(/\/+$/, '');
const withoutProtocol = (value = '') => trimSlash(value).replace(/^https?:\/\//i, '');
const quoteToml = (value = '') => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');

const generateDocker = (sources) => JSON.stringify({
    'registry-mirrors': sources.map((item) => trimSlash(item.url)),
}, null, 2);

const generatePodman = (sources) => {
    const mirrors = sources.map((item) => `[[registry.mirror]]
location = "${quoteToml(withoutProtocol(item.url))}"
insecure = false`).join('\n\n');
    return `unqualified-search-registries = ["docker.io"]

[[registry]]
prefix = "docker.io"
location = "docker.io"

${mirrors}`;
};

const generateContainerd = (sources) => {
    const endpoints = sources.map((item) => `"${quoteToml(trimSlash(item.url))}"`).join(', ');
    return `version = 2

[plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
  endpoint = [${endpoints}]`;
};

const generateNerdctl = (sources) => {
    const hosts = sources.map((item) => `[host."${quoteToml(trimSlash(item.url))}"]
  capabilities = ["pull", "resolve"]`).join('\n\n');
    return `# 保存为 /etc/containerd/certs.d/docker.io/hosts.toml
server = "https://registry-1.docker.io"

${hosts}`;
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
