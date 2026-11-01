import test from 'node:test';
import assert from 'node:assert/strict';
import { getProviderLogo } from './providerLogos.js';

test('associe les identifiants du catalogue aux marques attendues', () => {
  for (const [id, title] of [
    ['docker', 'Docker'],
    ['gcp', 'Google Cloud'],
    ['proxmox', 'Proxmox'],
    ['openstack', 'OpenStack'],
    ['talos-bootstrap', 'Talos'],
    ['kubeadm-bootstrap', 'Kubernetes'],
    ['vsphere', 'VMware'],
    ['ionoscloud-ionoscloud', 'Ionos'],
    ['vultr-vultr', 'Vultr'],
  ]) {
    assert.equal(getProviderLogo(id)?.title, title, id);
    assert.ok(getProviderLogo(id).path, id);
  }
});

test('les providers inconnus ne reçoivent pas le logo d’un autre provider', () => {
  assert.equal(getProviderLogo('my-docker-provider'), undefined);
  assert.equal(getProviderLogo(''), undefined);
  assert.equal(getProviderLogo('DOCKER').title, 'Docker');
});
