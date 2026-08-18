import assert from 'node:assert/strict';
import test from 'node:test';

import { generateMirrorConfig } from '../src/utils/mirror-config.js';

test('K3s uses registry hosts without URL schemes as mirror keys', () => {
    const config = generateMirrorConfig('k3s', [
        { origin: 'https://docker.io', url: 'https://mirror.example.com' },
        { origin: 'http://registry.example.com:5000', url: 'http://mirror.local:5001' },
    ]);

    assert.match(config, /"docker\.io":/);
    assert.match(config, /"registry\.example\.com:5000":/);
    assert.doesNotMatch(config, /^\s+"https?:\/\/[^\n]+":$/m);
});

test('Podman marks HTTP origins and mirrors as insecure', () => {
    const config = generateMirrorConfig('podman', [
        { origin: 'http://registry.example.com:5000', url: 'http://mirror.local:5001' },
        { origin: 'http://registry.example.com:5000', url: 'https://mirror.example.com' },
    ]);

    assert.match(config, /location = "registry\.example\.com:5000"\ninsecure = true/);
    assert.match(config, /location = "mirror\.local:5001"\ninsecure = true/);
    assert.match(config, /location = "mirror\.example\.com"\ninsecure = false/);
});

test('Containerd config v2 uses certs.d without emitting a root config version', () => {
    const config = generateMirrorConfig('containerd', [
        { origin: 'https://docker.io', url: 'https://mirror.example.com' },
    ]);

    assert.match(config, /\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\]/);
    assert.match(config, /config_path = "\/etc\/containerd\/certs\.d"/);
    assert.match(config, /\/etc\/containerd\/certs\.d\/docker\.io\/hosts\.toml/);
    assert.doesNotMatch(config, /^version\s*=/m);
});

test('Containerd config v3 uses the CRI images plugin path', () => {
    const config = generateMirrorConfig('containerd2', [
        { origin: 'https://docker.io', url: 'https://mirror.example.com' },
    ]);

    assert.match(config, /\[plugins\."io\.containerd\.cri\.v1\.images"\.registry\]/);
    assert.match(config, /config_path = "\/etc\/containerd\/certs\.d"/);
    assert.doesNotMatch(config, /io\.containerd\.grpc\.v1\.cri/);
});
