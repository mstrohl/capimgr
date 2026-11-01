import {
  siApachecloudstack, siCanonical, siDigitalocean, siDocker, siGooglecloud,
  siHelm, siHetzner, siHuawei, siIonos, siK3s, siKubernetes, siMaas,
  siNutanix, siOpennebula, siOpenstack, siProxmox, siRancher, siScaleway,
  siTalos, siVmware, siVultr,
} from 'simple-icons';

const logos = {
  'canonical-kubernetes': siCanonical,
  cloudstack: siApachecloudstack,
  digitalocean: siDigitalocean,
  docker: siDocker,
  gcp: siGooglecloud,
  helm: siHelm,
  hetzner: siHetzner,
  huawei: siHuawei,
  'ionoscloud-ionoscloud': siIonos,
  kubeadm: siKubernetes,
  'kubeadm-bootstrap': siKubernetes,
  'kubekey-k3s': siK3s,
  maas: siMaas,
  nutanix: siNutanix,
  opennebula: siOpennebula,
  openstack: siOpenstack,
  proxmox: siProxmox,
  'rancher-fleet': siRancher,
  rke2: siRancher,
  scaleway: siScaleway,
  talos: siTalos,
  'talos-bootstrap': siTalos,
  'vultr-vultr': siVultr,
  vsphere: siVmware,
};

export function getProviderLogo(id) {
  return logos[(id || '').toLowerCase()];
}
