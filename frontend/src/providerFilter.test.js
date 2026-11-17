import test from 'node:test';
import assert from 'node:assert/strict';
import { filterProviders } from './providerFilter.js';

const providers = [
  { id: 'in-cluster', name: 'In-Cluster IPAM', category: 'ipam', status: 'active' },
  { id: 'talos', name: 'Talos', category: 'controlplane', status: 'active' },
  { id: 'old', name: 'Old IPAM', category: 'ipam', status: 'archived' },
];

test('combines case-insensitive name or ID search with category and archive filters', () => {
  assert.deepEqual(filterProviders(providers, 'ipam', '  IN-CLUSTER ').map((p) => p.id), ['in-cluster']);
  assert.deepEqual(filterProviders(providers, 'ipam', 'IPAM').map((p) => p.id), ['in-cluster']);
  assert.deepEqual(filterProviders(providers, 'archived', 'ipam').map((p) => p.id), ['old']);
  assert.deepEqual(filterProviders(providers, 'all', 'missing'), []);
});
