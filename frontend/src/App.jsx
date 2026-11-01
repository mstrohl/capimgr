import React, { useState, useEffect, useRef } from 'react';
import { dump as dumpYaml, load as loadYaml } from 'js-yaml';
import { getProviderLogo } from './providerLogos';
import awsLogo from './assets/provider-logos/aws.svg';
import azureLogo from './assets/provider-logos/azure.svg';
import { useLanguage } from './LanguageContext';

const KubernetesIcon = () => (
  <svg className="w-6 h-6 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 2L2 7.5v9L12 22l10-5.5v-9L12 2zm0 2.2l7.5 4.1-3 1.6-4.5-2.4-4.5 2.4-3-1.6L12 4.2zm-8 6.4l3 1.6v3.3l-3-1.6v-3.3zm5 4.3v-2.7l3 1.6 3-1.6v2.7l-3 1.6-3-1.6zm8-1v-3.3l3-1.6v3.3l-3 1.6z"/>
  </svg>
);

const ProviderIcon = ({ id }) => {
  const lower = (id || '').toLowerCase();
  const image = { aws: awsLogo, azure: azureLogo }[lower];
  const logo = getProviderLogo(lower);
  return (
    <div aria-hidden="true" className={`p-3 border rounded-xl ${image || logo ? 'bg-white border-gray-200' : 'bg-gray-950 border-gray-800'}`}>
      {image ? <img src={image} alt="" className="w-12 h-12 object-contain" /> :
        logo ? <svg viewBox="0 0 24 24" className="w-12 h-12" fill={`#${logo.hex}`}><path d={logo.path} /></svg> :
          <KubernetesIcon />}
    </div>
  );
};

// --- Composant Toast (Notification) ---
const Toast = ({ message, type, onClose }) => {
  useEffect(() => {
    const timer = setTimeout(onClose, 4000);
    return () => clearTimeout(timer);
  }, [onClose]);

  const bgColor = type === 'error' ? 'bg-red-600' : 'bg-emerald-600';

  return (
    <div className={`fixed bottom-5 right-5 ${bgColor} text-white px-6 py-3 rounded-xl shadow-2xl z-50 flex items-center space-x-3 text-sm font-medium animate-fade-in`}>
      <span>{type === 'error' ? '⚠️' : '✅'}</span>
      <span>{message}</span>
      <button onClick={onClose} className="ml-2 hover:opacity-75">✕</button>
    </div>
  );
};

// --- Composant Modal pour Suivi de Job (Logs & Statut) ---
const JobModal = ({ jobName, onClose, apiUrl, onComplete }) => {
  const { t } = useLanguage();
  const [jobStatus, setJobStatus] = useState(null);
  const [logs, setLogs] = useState('');
  const [logState, setLogState] = useState('loading');
  const [jobError, setJobError] = useState('');
  const completionReported = useRef(false);

  useEffect(() => {
    if (!jobName) return;

    const fetchStatus = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/jobs/${jobName}/status`);
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        setJobStatus(data);
        setJobError('');
        if (['Succeeded', 'Failed'].includes(data.status) && !completionReported.current) {
          completionReported.current = true;
          onComplete?.(data);
        }
      } catch (err) {
        setJobError(err.message);
      }
    };

    const fetchLogs = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/jobs/${jobName}/logs`);
        if (res.status === 409) {
          setLogState('pending');
          return;
        }
        if (!res.ok) throw new Error(await res.text());
        const text = await res.text();
        setLogs(text);
        setLogState('loaded');
      } catch (err) {
        setLogs(err.message);
        setLogState('error');
      }
    };

    fetchStatus();
    fetchLogs();
    const interval = setInterval(() => {
      fetchStatus();
      fetchLogs();
    }, 2500);

    return () => clearInterval(interval);
  }, [jobName, apiUrl, onComplete]);

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl max-w-3xl w-full p-6 text-white shadow-2xl flex flex-col max-h-[85vh]">
        <div className="flex justify-between items-center pb-4 border-b border-gray-800">
          <div>
            <h3 className="text-lg font-bold">{t('Job status:', 'Suivi du Job :')} {jobName}</h3>
            <p className="text-xs text-gray-400">{t('Status:', 'Statut:')} <span className="font-semibold text-blue-400">{jobStatus?.status || t('Checking...', 'Vérification...')}</span></p>
            {jobError && <p role="alert" className="mt-1 text-xs text-red-400">{jobError}</p>}
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-white text-xl">✕</button>
        </div>
        <div className="flex-1 my-4 overflow-hidden flex flex-col">
          <label className="text-xs font-semibold text-gray-400 mb-2">{t('clusterctl execution logs:', "Logs d'exécution clusterctl :")}</label>
          <pre className="flex-1 bg-black/80 rounded-xl p-4 font-mono text-xs text-green-400 overflow-y-auto whitespace-pre-wrap leading-relaxed border border-gray-800">
            {logState === 'loading' ? t('Loading logs...', 'Chargement des logs...') :
              logState === 'pending' ? t('The job pod has not started yet.', 'Le Pod du Job n’a pas encore démarré.') :
                logState === 'error' ? `${t('Failed to read logs:', 'Erreur de lecture des logs :')} ${logs}` :
                  logs || t('No logs yet...', 'Aucun log pour le moment...')}
          </pre>
        </div>
        <div className="flex justify-end pt-2">
          <button onClick={onClose} className="px-5 py-2 bg-gray-800 hover:bg-gray-700 rounded-xl text-sm font-medium transition">
            {t('Close', 'Fermer')}
          </button>
        </div>
      </div>
    </div>
  );
};

// --- Composant Principal de l'Application ---
export default function App() {
  const { language, changeLanguage, t } = useLanguage();
  const [view, setView] = useState('dashboard'); // dashboard, clusters, providers, crds, create
  const [toast, setToast] = useState(null);
  const [activeJob, setActiveJob] = useState(null);

  const [clusters, setClusters] = useState([]);
  const [providers, setProviders] = useState([]);
  const [crds, setCrds] = useState([]);
  const [selectedCRD, setSelectedCRD] = useState(null);
  const [crdSchema, setCrdSchema] = useState(null);
  const [loadingSchema, setLoadingSchema] = useState(false);
  const [selectedCluster, setSelectedCluster] = useState(null);
  const [loadingClusters, setLoadingClusters] = useState(false);

  // Configuration dynamique de l'URL Backend
  const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8085';

  const showToast = (message, type = 'success') => setToast({ message, type });

  const fetchClusters = async () => {
    setLoadingClusters(true);
    try {
      const response = await fetch(`${API_URL}/api/clusters`);
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      setClusters(data || []);
    } catch (error) {
      showToast(t(`Unable to load clusters: ${error.message}`, `Chargement des clusters impossible : ${error.message}`), 'error');
    } finally {
      setLoadingClusters(false);
    }
  };

  const fetchProviders = async () => {
    try {
      const response = await fetch(`${API_URL}/api/providers`);
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      setProviders(data || []);
    } catch (error) {
      showToast(t(`Unable to load providers: ${error.message}`, `Chargement des providers impossible : ${error.message}`), 'error');
    }
  };

  const fetchCRDs = async () => {
    try {
      const response = await fetch(`${API_URL}/api/crds`);
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      setCrds(data || []);
    } catch (error) {
      showToast(t(`Unable to load CRDs: ${error.message}`, `Chargement des CRDs impossible : ${error.message}`), 'error');
    }
  };

  const loadCRDSchema = async (crdName) => {
    setSelectedCRD(crdName);
    setLoadingSchema(true);
    try {
      const response = await fetch(`${API_URL}/api/crds/${crdName}/schema`);
      if (response.ok) {
        const data = await response.json();
        setCrdSchema(data);
      } else {
        showToast(t(`Unable to load schema for ${crdName}`, `Impossible de charger le schéma pour ${crdName}`), 'error');
      }
    } catch (error) {
      showToast(error.message, 'error');
    } finally {
      setLoadingSchema(false);
    }
  };

  useEffect(() => {
    fetchClusters();
    fetchProviders();
    fetchCRDs();
  }, []);

  const handleInstallProvider = async (providerId, customVersion) => {
    try {
      const payload = customVersion ? { version: customVersion } : {};
      const response = await fetch(`${API_URL}/api/providers/${providerId}/install`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      const data = await response.json();
      if (response.ok) {
        showToast(t(`Installation started (Job: ${data.jobName})`, `Installation initiée (Job: ${data.jobName})`));
        setActiveJob(data.jobName);
        fetchProviders();
        fetchCRDs();
      } else {
        showToast(data.message || t('Installation failed', 'Erreur lors de l’installation'), 'error');
      }
    } catch (err) {
      showToast(err.message, 'error');
    }
  };

  const handleDeleteProvider = async (providerId, includeCRD) => {
    const confirmation = includeCRD
      ? t(`Removing ${providerId} CRDs will also delete associated custom resources. Continue?`, `La purge des CRDs de ${providerId} supprimera également les ressources personnalisées associées. Continuer ?`)
      : t(`Uninstall ${providerId} without removing its CRDs?`, `Désinstaller le provider ${providerId} sans supprimer ses CRDs ?`);
    if (!window.confirm(confirmation)) return;
    try {
      const suffix = includeCRD ? '?includeCRD=true' : '';
      const response = await fetch(`${API_URL}/api/providers/${providerId}${suffix}`, { method: 'DELETE' });
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      showToast(t(`Uninstallation started (Job: ${data.jobName})`, `Désinstallation initiée (Job: ${data.jobName})`));
      setActiveJob(data.jobName);
      fetchProviders();
      fetchCRDs();
    } catch (error) {
      showToast(error.message, 'error');
    }
  };

  const renderView = () => {
    switch (view) {
      case 'dashboard':
        return (
          <DashboardView 
            clusterCount={clusters.length} 
            nodeCount={clusters.reduce((acc, c) => acc + (c.status !== 'Erreur' ? (c.nodes || 0) : 0), 0)} 
            providersInstalled={providers.filter(p => p.installed).length}
            totalProviders={providers.length}
            crdsCount={crds.length}
            onNavigate={setView}
          />
        );
      case 'providers':
        return (
          <ProviderCatalogView 
            providers={providers} 
            apiUrl={API_URL}
            onInstall={handleInstallProvider}
            onDelete={handleDeleteProvider}
            onRefresh={() => { fetchProviders(); fetchCRDs(); }}
          />
        );
      case 'crds':
        return (
          <CRDExplorerView 
            crds={crds} 
            selectedCRD={selectedCRD}
            schemaData={crdSchema}
            loading={loadingSchema}
            onSelectCRD={loadCRDSchema}
          />
        );
      case 'clusters':
        return (
          <ClusterListView 
            clusters={clusters} 
            onRefresh={fetchClusters}
            loading={loadingClusters}
            onDetails={setSelectedCluster}
            onDelete={async (id) => {
              if (window.confirm(t('Delete this cluster?', 'Confirmer la suppression du cluster ?'))) {
                try {
                  const response = await fetch(`${API_URL}/api/clusters/${id}`, { method: 'DELETE' });
                  if (!response.ok) throw new Error(await response.text());
                  showToast(t('Deletion requested...', 'Suppression en cours...'));
                  fetchClusters();
                } catch (error) {
                  showToast(error.message, 'error');
                }
              }
            }} 
          />
        );
      case 'create':
        return (
          <ClusterCreateView
            providers={providers}
            apiUrl={API_URL}
            showToast={showToast}
            onCancel={() => setView('clusters')}
            onCreated={(cluster) => {
              showToast(t(`Cluster ${cluster.name} created.`, `Cluster ${cluster.name} créé.`));
              setView('clusters');
              fetchClusters();
            }}
          />
        );
      default:
        return <DashboardView clusterCount={clusters.length} nodeCount={0} providersInstalled={0} totalProviders={0} crdsCount={0} onNavigate={setView} />;
    }
  };

  return (
    <div className="flex h-screen bg-gray-950 text-gray-100 font-inter">
      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
      {activeJob && (
        <JobModal
          jobName={activeJob}
          apiUrl={API_URL}
          onClose={() => setActiveJob(null)}
          onComplete={() => { fetchProviders(); fetchCRDs(); fetchClusters(); }}
        />
      )}
      {selectedCluster && (
        <ClusterDetailModal
          cluster={selectedCluster}
          apiUrl={API_URL}
          onClose={() => setSelectedCluster(null)}
          onScaled={fetchClusters}
          showToast={showToast}
        />
      )}

      {/* --- Sidebar Navigation --- */}
      <aside className="w-64 bg-gray-900 border-r border-gray-800 flex flex-col p-5">
        <div className="flex items-center space-x-3 mb-8 px-2">
          <div className="p-2 bg-blue-600/20 border border-blue-500/30 rounded-xl">
            <KubernetesIcon />
          </div>
          <div>
            <h1 className="text-base font-bold text-white tracking-wide">capimgr</h1>
            <p className="text-xs text-gray-400">Cluster API Manager</p>
          </div>
        </div>

        <nav className="flex-1 space-y-1.5">
          <NavItem icon="📊" text={t('Dashboard', 'Tableau de bord')} active={view === 'dashboard'} onClick={() => setView('dashboard')} />
          <NavItem icon="📦" text={t('CAPI providers', 'Providers CAPI')} count={providers.length} active={view === 'providers'} onClick={() => setView('providers')} />
          <NavItem icon="🧩" text={t('Provider CRDs', 'CRDs des Providers')} count={crds.length} active={view === 'crds'} onClick={() => setView('crds')} />
          <NavItem icon="☸️" text={t('Workload clusters', 'Clusters Enfants')} count={clusters.length} active={view === 'clusters'} onClick={() => setView('clusters')} />
          <NavItem icon="➕" text={t('Create a cluster', 'Créer un cluster')} active={view === 'create'} onClick={() => setView('create')} />
          <a href={`${API_URL}/api/docs/`} target="_blank" rel="noopener noreferrer" className="flex items-center space-x-3 w-full px-4 py-3 rounded-xl text-sm font-medium text-gray-400 hover:bg-gray-800/60 hover:text-gray-200 transition duration-150">
            <span className="text-lg" aria-hidden="true">📖</span>
            <span>{t('API documentation (Swagger)', 'Documentation API (Swagger)')}</span>
          </a>
        </nav>

        <label className="mb-4 block text-xs text-gray-400">
          {t('Language', 'Langue')}
          <select aria-label={t('Language', 'Langue')} value={language} onChange={(event) => changeLanguage(event.target.value)} className="mt-1 w-full rounded-lg border border-gray-700 bg-gray-950 px-3 py-2 text-sm text-white">
            <option value="en">English</option>
            <option value="fr">Français</option>
          </select>
        </label>
        <div className="pt-4 border-t border-gray-800 text-xs text-gray-400 flex items-center justify-between">
          <span className="flex items-center space-x-2">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
            <span>API: {API_URL}</span>
          </span>
        </div>
      </aside>

      {/* --- Main Content Area --- */}
      <main className="flex-1 overflow-y-auto p-8">
        {renderView()}
      </main>
    </div>
  );
}

const NavItem = ({ icon, text, count, active, onClick }) => (
  <button
    onClick={onClick}
    className={`flex items-center justify-between w-full px-4 py-3 rounded-xl transition duration-150 text-sm font-medium ${
      active
        ? 'bg-blue-600 text-white shadow-lg shadow-blue-500/20'
        : 'text-gray-400 hover:bg-gray-800/60 hover:text-gray-200'
    }`}
  >
    <div className="flex items-center space-x-3">
      <span className="text-lg">{icon}</span>
      <span>{text}</span>
    </div>
    {count !== undefined && count > 0 && (
      <span className={`text-xs px-2 py-0.5 rounded-full ${active ? 'bg-blue-700 text-white' : 'bg-gray-800 text-gray-300'}`}>
        {count}
      </span>
    )}
  </button>
);

// --- Vue 1: Dashboard ---
const DashboardView = ({ clusterCount, nodeCount, providersInstalled, totalProviders, crdsCount, onNavigate }) => {
  const { t } = useLanguage();
  return <div>
    <div className="mb-8">
      <h2 className="text-2xl font-bold text-white tracking-tight">{t('Overview', "Vue d'ensemble")}</h2>
      <p className="text-gray-400 text-sm mt-1">{t('Monitor your management cluster and CAPI components', 'Supervision de votre cluster de management et des composants CAPI')}</p>
    </div>

    <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
      <StatCard title={t('Installed providers', 'Providers Installés')} value={`${providersInstalled} / ${totalProviders}`} subtitle={t('Active on the cluster', 'Actifs sur le cluster')} icon="📦" onClick={() => onNavigate('providers')} />
      <StatCard title={t('Discovered CRDs', 'CRDs Introspectées')} value={crdsCount} subtitle={t('Configurable schemas', 'Schémas configurables')} icon="🧩" onClick={() => onNavigate('crds')} />
      <StatCard title={t('Workload clusters', 'Clusters Enfants')} value={clusterCount} subtitle="Workload clusters" icon="☸️" onClick={() => onNavigate('clusters')} />
      <StatCard title={t('Managed nodes', 'Nœuds Gérés')} value={nodeCount} subtitle="Total CP + Workers" icon="🖥️" />
    </div>

    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 className="text-base font-semibold text-white mb-3">{t('Active capabilities', 'Capacités Actives')}</h3>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-blue-400 font-semibold mb-1">✓ {t('Dynamic discovery', 'Découverte Dynamique')}</div>
          <p className="text-gray-400 text-xs">{t('Cluster API catalog with more than 40 providers and version management.', 'Catalogue Cluster API synchronisé avec plus de 40 providers et gestion des versions.')}</p>
        </div>
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-emerald-400 font-semibold mb-1">✓ {t('CRD introspection', 'Introspection des CRDs')}</div>
          <p className="text-gray-400 text-xs">{t('Automatically extract OpenAPI v3 schemas to generate forms.', 'Extraction automatique des schémas OpenAPI v3 pour générer des formulaires à la volée.')}</p>
        </div>
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-purple-400 font-semibold mb-1">✓ {t('Asynchronous jobs', 'Jobs Asynchrones')}</div>
          <p className="text-gray-400 text-xs">{t('Non-blocking installation and removal using Kubernetes jobs with live logs.', 'Installation et désinstallation non-bloquante via K8s Jobs avec suivi des logs en temps réel.')}</p>
        </div>
      </div>
    </div>
  </div>;
};

const StatCard = ({ title, value, subtitle, icon, onClick }) => (
  <div onClick={onClick} className={`bg-gray-900 border border-gray-800 rounded-2xl p-5 hover:border-blue-500/50 transition cursor-pointer`}>
    <div className="flex items-center justify-between text-gray-400 text-xs font-medium uppercase tracking-wider mb-2">
      <span>{title}</span>
      <span className="text-lg">{icon}</span>
    </div>
    <div className="text-3xl font-extrabold text-white">{value}</div>
    <div className="text-xs text-gray-500 mt-1">{subtitle}</div>
  </div>
);

// --- Vue 2: Catalogue des Providers ---
const ProviderCatalogView = ({ providers, apiUrl, onInstall, onDelete, onRefresh }) => {
  const { t, language } = useLanguage();
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [installingProvider, setInstallingProvider] = useState(null);
  const [customVersion, setCustomVersion] = useState('');
  const [versionInfo, setVersionInfo] = useState({ loading: false, versions: [], error: '' });

  useEffect(() => {
    if (!installingProvider) return undefined;
    let cancelled = false;
    setVersionInfo({ loading: true, versions: [], error: '' });
    fetch(`${apiUrl}/api/providers/${installingProvider.id}/versions`)
      .then((response) => (response.ok ? response.json() : Promise.reject(new Error(`HTTP ${response.status}`))))
      .then((data) => {
        if (cancelled) return;
        setVersionInfo({ loading: false, versions: data.versions || [], error: '' });
        setCustomVersion(data.default || '');
      })
      .catch((error) => {
        if (!cancelled) setVersionInfo({ loading: false, versions: [], error: t(`Version list unavailable (${error.message})`, `Liste des versions indisponible (${error.message})`) });
      });
    return () => { cancelled = true; };
  }, [installingProvider, apiUrl, language]);

  const categories = [
    { id: 'all', label: t('All', 'Tous') },
    { id: 'infrastructure', label: 'Infrastructure' },
    { id: 'controlplane', label: 'Control Plane' },
    { id: 'bootstrap', label: 'Bootstrap' },
    { id: 'core', label: 'Core' },
    { id: 'archived', label: t('Archived', 'Archivés') }
  ];

  const isArchived = (p) => p.status === 'archived';
  const filtered = providers.filter(p => {
    if (selectedCategory === 'archived') return isArchived(p);
    if (selectedCategory === 'all') return true;
    return p.category === selectedCategory && !isArchived(p);
  });

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-white tracking-tight">{t('CAPI provider catalog', 'Catalogue des Providers CAPI')}</h2>
          <p className="text-gray-400 text-sm mt-1">{t('Manage supported providers and their versions', "Gérez l'ensemble des providers supportés et leurs versions")}</p>
        </div>
        <button onClick={onRefresh} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-xs rounded-xl font-medium transition flex items-center space-x-2">
          <span>🔄</span>
          <span>{t('Refresh', 'Actualiser')}</span>
        </button>
      </div>

      {/* Filtres par catégorie */}
      <div className="flex space-x-2 mb-6 overflow-x-auto pb-2">
        {categories.map(c => (
          <button
            key={c.id}
            onClick={() => setSelectedCategory(c.id)}
            className={`px-4 py-1.5 rounded-xl text-xs font-semibold transition ${
              selectedCategory === c.id
                ? 'bg-blue-600 text-white shadow-md shadow-blue-500/20'
                : 'bg-gray-900 text-gray-400 hover:bg-gray-800'
            }`}
          >
            {c.label}
          </button>
        ))}
      </div>

      {/* Grille des cartes de providers */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filtered.map(p => (
          <div key={p.id} className={`bg-gray-900 border border-gray-800 rounded-2xl p-5 flex flex-col justify-between hover:border-gray-700 transition ${isArchived(p) ? 'opacity-50 grayscale' : ''}`}>
            <div>
              <div className="flex items-start justify-between mb-4">
                <ProviderIcon id={p.id} />
                <div className="flex flex-col items-end space-y-1">
                  <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                    p.installed ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400'
                  }`}>
                    {p.installed ? t('✓ Installed', '✓ Installé') : isArchived(p) ? t('Archived', 'Archivé') : t('Available', 'Disponible')}
                  </span>
                  <span className="text-[10px] text-gray-500 uppercase tracking-wider">{p.category}</span>
                </div>
              </div>

              <h3 className="text-base font-bold text-white">{p.name}</h3>
              <p className="text-xs text-gray-400 mt-1">Version: <span className="font-mono text-blue-400 font-semibold">{p.version || 'latest'}</span></p>

              {/* CRDs associées */}
              {p.crds && p.crds.length > 0 && (
                <div className="mt-4 pt-3 border-t border-gray-800">
                  <div className="text-[11px] font-medium text-gray-400 mb-1.5">{t('Active CRDs', 'CRDs actives')} ({p.crds.length}) :</div>
                  <div className="flex flex-wrap gap-1">
                    {p.crds.map(crd => (
                      <span key={crd} className="text-[10px] bg-gray-800 px-2 py-0.5 rounded text-gray-300 font-mono">
                        {crd.split('.')[0]}
                      </span>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="mt-5 pt-4 border-t border-gray-800">
              {p.installed ? (
                <div className="space-y-2">
                  <div className="w-full py-2 bg-emerald-950/40 text-center text-emerald-400 text-xs font-semibold rounded-xl border border-emerald-800/40">
                    ✓ {t('Component operational', 'Composant Opérationnel')}
                  </div>
                  <button onClick={() => onDelete(p.id, false)} className="w-full py-2 bg-gray-800 hover:bg-gray-700 text-gray-200 text-xs font-semibold rounded-xl">
                    {t('Uninstall (preserve CRDs)', 'Désinstaller (préserver les CRDs)')}
                  </button>
                  <button onClick={() => onDelete(p.id, true)} className="w-full py-2 bg-red-950/50 hover:bg-red-900/60 text-red-300 text-xs font-semibold rounded-xl border border-red-900/50">
                    {t('Uninstall and remove CRDs…', 'Désinstaller et purger les CRDs…')}
                  </button>
                </div>
              ) : isArchived(p) ? (
                <div className="w-full py-2 bg-gray-800 text-center text-gray-500 text-xs font-semibold rounded-xl cursor-not-allowed">
                  {t('Archived — installation unavailable', 'Archivé — installation impossible')}
                </div>
              ) : (
                <button
                  onClick={() => {
                    setInstallingProvider(p);
                    setCustomVersion(p.version || '');
                  }}
                  className="w-full py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-xl transition shadow-md shadow-blue-600/20"
                >
                  {t('Install provider', 'Installer ce Provider')}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Modal d'installation avec sélection de version */}
      {installingProvider && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-2xl max-w-md w-full p-6 text-white shadow-2xl">
            <h3 className="text-lg font-bold mb-1">{t('Install', 'Installer')} {installingProvider.name}</h3>
            <p className="text-xs text-gray-400 mb-4">{t('A Kubernetes job will run `clusterctl init` to deploy this provider.', 'Un Job Kubernetes exécutera `clusterctl init` pour déployer ce provider.')}</p>

            <label className="block text-xs font-semibold text-gray-300 mb-1">{t('Target version:', 'Version cible :')}</label>
            {versionInfo.versions.length > 0 ? (
              <select
                value={customVersion}
                onChange={(e) => setCustomVersion(e.target.value)}
                className="w-full bg-gray-950 border border-gray-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 mb-5"
              >
                {versionInfo.versions.map((v) => (
                  <option key={v.version} value={v.version}>
                    {v.version}{v.prerelease ? t(' (pre-release)', ' (pré-version)') : ''}
                  </option>
                ))}
              </select>
            ) : (
              <>
                <input 
                  type="text"
                  value={customVersion}
                  onChange={(e) => setCustomVersion(e.target.value)}
                  placeholder={versionInfo.loading ? t('Loading versions…', 'Chargement des versions…') : t('e.g. v2.4.0 (or leave blank for latest)', 'ex: v2.4.0 (ou laisser vide pour la dernière)')}
                  className="w-full bg-gray-950 border border-gray-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 mb-2"
                />
                {versionInfo.error && <p className="text-xs text-amber-400 mb-3">{versionInfo.error} — {t('manual entry is available.', 'saisie manuelle possible.')}</p>}
              </>
            )}

            <div className="flex justify-end space-x-3">
              <button 
                onClick={() => setInstallingProvider(null)}
                className="px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-xl text-xs font-medium transition"
              >
                {t('Cancel', 'Annuler')}
              </button>
              <button 
                onClick={() => {
                  const id = installingProvider.id;
                  setInstallingProvider(null);
                  onInstall(id, customVersion);
                }}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-xl text-xs font-medium transition font-semibold"
              >
                {t('Start installation', "Lancer l'installation")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

// --- Vue 3: Explorateur de CRDs & Schémas Dynamiques ---
const CRDExplorerView = ({ crds, selectedCRD, schemaData, loading, onSelectCRD }) => {
  const { t } = useLanguage();
  const [resource, setResource] = useState(null);
  const [yamlText, setYamlText] = useState('');
  const [yamlError, setYamlError] = useState('');
  const [activeTab, setActiveTab] = useState('form');

  useEffect(() => {
    if (!schemaData) {
      setResource(null);
      setYamlText('');
      setYamlError('');
      return;
    }
    const nextResource = {
      apiVersion: schemaData.apiVersion,
      kind: schemaData.kind,
      metadata: { name: '' },
      spec: schemaDefaults(schemaData.schema)
    };
    setResource(nextResource);
    setYamlText(dumpYaml(nextResource, { noRefs: true, lineWidth: -1 }));
    setYamlError('');
    setActiveTab('form');
  }, [schemaData]);

  useEffect(() => {
    if (resource) setYamlText(dumpYaml(resource, { noRefs: true, lineWidth: -1 }));
  }, [resource]);

  const updateResource = (path, value) => {
    setResource((current) => setAtPath(current, path, value));
    setYamlError('');
  };

  const updateYaml = (value) => {
    setYamlText(value);
    try {
      const parsed = loadYaml(value);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(t('The YAML document must be a Kubernetes object.', 'Le document YAML doit être un objet Kubernetes.'));
      }
      setResource(parsed);
      setYamlError('');
    } catch (error) {
      setYamlError(error.message);
    }
  };

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-white tracking-tight">{t('Discovered CRDs & OpenAPI v3 schemas', 'CRDs Introspectées & Schémas OpenAPI v3')}</h2>
        <p className="text-gray-400 text-sm mt-1">{t('Schemas dynamically extracted from Kubernetes to generate configuration forms', 'Schémas extraits dynamiquement depuis Kubernetes pour alimenter les formulaires de configuration')}</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Liste des CRDs */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-4 h-[75vh] flex flex-col">
          <h3 className="text-xs font-bold text-gray-400 uppercase tracking-wider mb-3 px-2">{t('Discovered resources', 'Ressources Détectées')} ({crds.length})</h3>
          <div className="flex-1 overflow-y-auto space-y-1 pr-1">
            {crds.map(c => (
              <button
                key={c.name}
                onClick={() => onSelectCRD(c.name)}
                className={`w-full text-left p-3 rounded-xl transition text-xs flex flex-col ${
                  selectedCRD === c.name
                    ? 'bg-blue-600/20 border border-blue-500/40 text-blue-300'
                    : 'hover:bg-gray-800/60 text-gray-300 border border-transparent'
                }`}
              >
                <div className="flex justify-between items-center mb-1">
                  <span className="font-bold text-white">{c.kind}</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">{c.provider}</span>
                </div>
                <span className="text-[10px] text-gray-500 truncate">{c.name}</span>
              </button>
            ))}
          </div>
        </div>

        {/* Détail du schéma sélectionné */}
        <div className="lg:col-span-2 bg-gray-900 border border-gray-800 rounded-2xl p-6 h-[75vh] flex flex-col">
          {loading ? (
            <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">{t('Loading OpenAPI v3 schema...', 'Chargement du schéma OpenAPI v3...')}</div>
          ) : schemaData && resource ? (
            <div className="flex-1 flex flex-col overflow-hidden">
              <div className="pb-4 border-b border-gray-800 mb-4 flex justify-between items-start">
                <div>
                  <h3 className="text-lg font-bold text-white">{schemaData.kind}</h3>
                  <p className="text-xs text-blue-400 font-mono mt-0.5">apiVersion: {schemaData.apiVersion}</p>
                </div>
                <span className="text-xs px-2.5 py-1 rounded-full bg-blue-500/20 text-blue-400 border border-blue-500/30">
                  {t('Dynamic form', 'Formulaire dynamique')}
                </span>
              </div>

              <div className="flex gap-2 mb-4">
                <button onClick={() => setActiveTab('form')} className={`px-3 py-1.5 rounded-lg text-xs ${activeTab === 'form' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-300'}`}>{t('Form', 'Formulaire')}</button>
                <button onClick={() => setActiveTab('yaml')} className={`px-3 py-1.5 rounded-lg text-xs ${activeTab === 'yaml' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-300'}`}>YAML</button>
              </div>
              {activeTab === 'form' ? (
                <div className="flex-1 overflow-y-auto space-y-5 pr-2">
                  <label className="block">
                    <span className="text-xs text-gray-300">{t('Resource name', 'Nom de la ressource')}</span>
                    <input
                      value={resource.metadata?.name || ''}
                      onChange={(event) => updateResource(['metadata', 'name'], event.target.value)}
                      className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white"
                      placeholder="nom-ressource"
                    />
                  </label>
                  <CRDFormRenderer
                    schema={schemaData.schema}
                    value={resource.spec || {}}
                    path={['spec']}
                    onChange={updateResource}
                  />
                </div>
              ) : (
                <div className="flex-1 min-h-0 flex flex-col">
                  <textarea
                    aria-label={t('Kubernetes resource YAML editor', 'Éditeur YAML de la ressource Kubernetes')}
                    value={yamlText}
                    onChange={(event) => updateYaml(event.target.value)}
                    spellCheck="false"
                    className="flex-1 min-h-0 bg-black/70 border border-gray-700 rounded-xl p-4 font-mono text-xs text-green-300 resize-none"
                  />
                  {yamlError && <p role="alert" className="mt-2 text-xs text-red-400">{yamlError}</p>}
                </div>
              )}
            </div>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-gray-500 text-sm">
              <span className="text-3xl mb-2">👈</span>
              <span>{t('Select a CRD to inspect its form schema.', 'Sélectionnez une CRD dans la liste pour inspecter son schéma de formulaire.')}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

function schemaDefaults(schema) {
  if (schema?.default !== undefined) return schema.default;
  if (Array.isArray(schema?.enum) && schema.enum.length) return schema.enum[0];
  if (schema?.type === 'object' || schema?.properties) {
    return Object.fromEntries(
      Object.entries(schema.properties || {}).map(([key, child]) => [key, schemaDefaults(child)])
    );
  }
  if (schema?.type === 'array') return [];
  if (schema?.type === 'boolean') return false;
  if (schema?.type === 'integer' || schema?.type === 'number') return 0;
  return '';
}

function setAtPath(source, path, value) {
  const result = Array.isArray(source) ? [...source] : { ...(source || {}) };
  const [key, ...remaining] = path;
  if (remaining.length === 0) {
    result[key] = value;
  } else {
    result[key] = setAtPath(result[key], remaining, value);
  }
  return result;
}

const CRDFormRenderer = ({ schema, value, path, onChange }) => {
  const { t } = useLanguage();
  const properties = schema?.properties || {};
  const required = schema?.required || [];
  return (
    <div className="space-y-4">
      {schema?.description && <p className="text-xs text-gray-400">{schema.description}</p>}
      {Object.entries(properties).map(([name, definition]) => {
        const fieldPath = [...path, name];
        const fieldValue = value?.[name] ?? schemaDefaults(definition);
        const label = (
          <span className="text-xs font-semibold text-gray-200">
            {name}{required.includes(name) && <span className="text-red-400"> *</span>}
          </span>
        );
        const help = definition.description && <p className="mt-1 text-[11px] text-gray-500">{definition.description}</p>;

        if (definition.type === 'object' || definition.properties) {
          return (
            <details key={name} open className="bg-gray-950 border border-gray-800 rounded-xl p-4">
              <summary className="cursor-pointer">{label}{help}</summary>
              <div className="mt-4 pl-3 border-l border-gray-800">
                <CRDFormRenderer schema={definition} value={fieldValue || {}} path={fieldPath} onChange={onChange} />
              </div>
            </details>
          );
        }

        if (definition.type === 'array') {
          const items = Array.isArray(fieldValue) ? fieldValue : [];
          return (
            <div key={name} className="bg-gray-950 border border-gray-800 rounded-xl p-4">
              {label}{help}
              <div className="mt-3 space-y-3">
                {items.map((item, index) => (
                  <div key={`${name}-${index}`} className="flex gap-2 items-start">
                    <div className="flex-1">
                      <CRDArrayItem
                        schema={definition.items || { type: 'string' }}
                        value={item}
                        path={[...fieldPath, index]}
                        onChange={onChange}
                      />
                    </div>
                    <button
                      type="button"
                      onClick={() => onChange(fieldPath, items.filter((_, itemIndex) => itemIndex !== index))}
                      className="px-2 py-1 text-xs text-red-400 hover:bg-red-950 rounded"
                    >{t('Remove', 'Supprimer')}</button>
                  </div>
                ))}
                <button
                  type="button"
                  onClick={() => onChange(fieldPath, [...items, schemaDefaults(definition.items || { type: 'string' })])}
                  className="px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 rounded-lg"
                >+ {t('Add', 'Ajouter')}</button>
              </div>
            </div>
          );
        }

        return (
          <label key={name} className="block bg-gray-950 border border-gray-800 rounded-xl p-4">
            {label}
            {definition.enum ? (
            <select required={required.includes(name)} value={fieldValue} onChange={(event) => onChange(fieldPath, definition.enum.find((option) => String(option) === event.target.value))} className="mt-2 w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white">
                {definition.enum.map((option) => <option key={String(option)} value={option}>{String(option)}</option>)}
              </select>
            ) : definition.type === 'boolean' ? (
            <input type="checkbox" checked={Boolean(fieldValue)} onChange={(event) => onChange(fieldPath, event.target.checked)} className="ml-3 accent-blue-500" />
            ) : (
              <input
                type={definition.type === 'integer' || definition.type === 'number' ? 'number' : 'text'}
              required={required.includes(name)}
                step={definition.type === 'integer' ? 1 : 'any'}
                value={fieldValue ?? ''}
                min={definition.minimum}
                max={definition.maximum}
                pattern={definition.pattern}
                onChange={(event) => onChange(fieldPath, definition.type === 'integer' ? (event.target.value === '' ? '' : Number.parseInt(event.target.value, 10)) : definition.type === 'number' ? (event.target.value === '' ? '' : Number(event.target.value)) : event.target.value)}
                className="mt-2 w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white"
              />
            )}
            {help}
          </label>
        );
      })}
    </div>
  );
};

const CRDArrayItem = ({ schema, value, path, onChange }) => {
  if (schema.type === 'object' || schema.properties) {
    return <CRDFormRenderer schema={schema} value={value || {}} path={path} onChange={onChange} />;
  }
  if (schema.enum) {
    return <select value={value} onChange={(event) => onChange(path, schema.enum.find((option) => String(option) === event.target.value))} className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white">{schema.enum.map((option) => <option key={String(option)} value={option}>{String(option)}</option>)}</select>;
  }
  if (schema.type === 'boolean') {
    return <input type="checkbox" checked={Boolean(value)} onChange={(event) => onChange(path, event.target.checked)} className="mt-2 accent-blue-500" />;
  }
  return <input type={schema.type === 'integer' || schema.type === 'number' ? 'number' : 'text'} value={value ?? ''} onChange={(event) => onChange(path, schema.type === 'integer' ? (event.target.value === '' ? '' : Number.parseInt(event.target.value, 10)) : schema.type === 'number' ? (event.target.value === '' ? '' : Number(event.target.value)) : event.target.value)} className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white" />;
};

// --- Vue 4: Liste des Clusters ---
const ClusterListView = ({ clusters, onDetails, onDelete, onRefresh, loading }) => {
  const { t } = useLanguage();
  return <div>
    <div className="mb-6 flex items-start justify-between gap-4">
      <div>
        <h2 className="text-2xl font-bold text-white tracking-tight">{t('Kubernetes workload clusters', 'Clusters Kubernetes Enfants')}</h2>
        <p className="text-gray-400 text-sm mt-1">{t('Clusters provisioned and managed by Cluster API', 'Clusters provisionnés et gérés via Cluster API')}</p>
      </div>
      <button type="button" onClick={onRefresh} disabled={loading} className="rounded-lg bg-gray-800 px-4 py-2 text-sm disabled:opacity-50">
        {loading ? t('Refreshing…', 'Actualisation…') : t('Refresh', 'Actualiser')}
      </button>
    </div>

    {loading && clusters.length === 0 ? <p className="text-sm text-gray-400">{t('Loading clusters…', 'Chargement des clusters…')}</p> : clusters.length === 0 ? (
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-12 text-center text-gray-500">
        <span className="text-4xl block mb-3">☸️</span>
        <p className="text-base font-medium text-gray-400">{t('No workload clusters have been deployed yet.', "Aucun cluster enfant déployé pour l'instant.")}</p>
        <p className="text-xs text-gray-500 mt-1">{t('Install an infrastructure provider to provision your first cluster.', "Installez un provider d'infrastructure pour provisionner votre premier cluster.")}</p>
      </div>
    ) : (
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden shadow-xl">
        <table className="min-w-full divide-y divide-gray-800 text-left text-sm">
          <thead className="bg-gray-950 text-gray-400 text-xs uppercase tracking-wider font-semibold">
            <tr>
              <th className="px-6 py-4">{t('Cluster name', 'Nom du Cluster')}</th>
              <th className="px-6 py-4">{t('Status', 'Statut')}</th>
              <th className="px-6 py-4">Infrastructure</th>
              <th className="px-6 py-4">Version K8s</th>
              <th className="px-6 py-4">{t('Nodes', 'Nœuds')}</th>
              <th className="px-6 py-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800/80 text-gray-300">
            {clusters.map(c => (
              <tr key={c.id} className="hover:bg-gray-800/40 transition">
                <td className="px-6 py-4 font-bold text-white">{c.name}</td>
                <td className="px-6 py-4">
                  <span className="inline-flex items-center space-x-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">
                    <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse"></span>
                    <span>{c.status}</span>
                  </span>
                </td>
                <td className="px-6 py-4 text-gray-400 font-mono text-xs">{c.infrastructureProvider}</td>
                <td className="px-6 py-4 font-mono text-xs text-blue-400">{c.kubernetesVersion || 'N/A'}</td>
                <td className="px-6 py-4">{c.nodes ?? 0}</td>
                <td className="px-6 py-4 text-right">
                  <button onClick={() => onDetails(c)} className="mr-4 text-blue-400 hover:text-blue-300 font-medium text-xs">
                    {t('Details', 'Détails')}
                  </button>
                  <button onClick={() => onDelete(c.id)} className="text-red-400 hover:text-red-300 font-medium text-xs">
                    {t('Delete', 'Supprimer')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )}
  </div>;
};

const ManifestEditor = ({ descriptor, object, apiUrl, onChange }) => {
  const { t, language } = useLanguage();
  const [tab, setTab] = useState('form');
  const [schema, setSchema] = useState(null);
  const [schemaError, setSchemaError] = useState('');
  const [yamlText, setYamlText] = useState('');
  const [yamlError, setYamlError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setSchema(null);
    setSchemaError('');
    fetch(`${apiUrl}/api/crds/${descriptor.crdName}/schema?version=${descriptor.version}`)
      .then((response) => (response.ok ? response.json() : Promise.reject(new Error(t('schema unavailable', 'schéma indisponible')))))
      .then((data) => { if (!cancelled) setSchema(data); })
      .catch((error) => { if (!cancelled) setSchemaError(error.message); });
    return () => { cancelled = true; };
  }, [descriptor.crdName, descriptor.version, apiUrl, language]);

  useEffect(() => {
    if (tab === 'yaml') {
      setYamlText(dumpYaml(object, { noRefs: true }));
      setYamlError('');
    }
  }, [tab, descriptor.id]);

  const specSchema = schema?.schema;
  const metadata = object.metadata || {};

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h4 className="text-lg font-bold text-white">{descriptor.kind}</h4>
          <p className="text-xs text-gray-500">{object.apiVersion}</p>
        </div>
        <div className="flex gap-1 rounded-lg bg-gray-950 p-1 text-xs">
          {['form', 'yaml'].map((name) => (
            <button key={name} type="button" onClick={() => setTab(name)} className={`px-3 py-1 rounded-md ${tab === name ? 'bg-blue-600 text-white' : 'text-gray-400'}`}>
              {name === 'form' ? t('Form', 'Formulaire') : 'YAML'}
            </button>
          ))}
        </div>
      </div>
      {tab === 'yaml' ? (
        <div>
          <textarea
            value={yamlText}
            spellCheck={false}
            onChange={(event) => {
              setYamlText(event.target.value);
              try {
                const parsed = loadYaml(event.target.value);
                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(t('YAML must be an object', 'le YAML doit être un objet'));
                setYamlError('');
                onChange(parsed);
              } catch (error) {
                setYamlError(error.message);
              }
            }}
            className="h-96 w-full rounded-xl border border-gray-700 bg-gray-950 p-3 font-mono text-xs text-gray-100"
          />
          {yamlError && <p className="mt-1 text-xs text-red-400">{yamlError}</p>}
        </div>
      ) : (
        <>
          <label className="block rounded-xl border border-gray-800 bg-gray-950 p-4">
            <span className="text-xs font-semibold text-gray-200">metadata.name <span className="text-red-400">*</span></span>
            <input required value={metadata.name || ''} onChange={(event) => onChange(setAtPath(object, ['metadata', 'name'], event.target.value))} className="mt-2 w-full rounded-lg border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-white" />
          </label>
          {schemaError && <p className="text-xs text-amber-400">{t('Form unavailable', 'Formulaire indisponible')} ({schemaError}) : {t('use the YAML tab.', 'utilisez l’onglet YAML.')}</p>}
          {!schema && !schemaError && <p className="text-xs text-gray-500">{t('Loading schema…', 'Chargement du schéma…')}</p>}
          {specSchema && (
            <CRDFormRenderer schema={specSchema} value={object.spec || {}} path={['spec']} onChange={(path, value) => onChange(setAtPath(object, path, value))} />
          )}
        </>
      )}
    </div>
  );
};

const ClusterManifestsModal = ({ blueprint, clusterName, apiUrl, submitting, onClose, onSubmit }) => {
  const { t, language } = useLanguage();
  const [items, setItems] = useState(() =>
    blueprint.required.map((descriptor) => ({ key: descriptor.id, descriptor, object: descriptor.object }))
  );
  const [selected, setSelected] = useState(items[0]?.key);
  const [toAdd, setToAdd] = useState('');
  const counter = useRef(0);

  const current = items.find((item) => item.key === selected) || items[0];
  const frenchDescriptions = {
    cluster: "Ressource racine : réseau et références vers le control plane et l'infrastructure.",
    'infrastructure-cluster': "Infrastructure du cluster.",
    'control-plane': "Control plane.",
    'control-plane-machine-template': "Gabarit des machines du control plane.",
    workers: "Pool de nœuds workers.",
    'workers-machine-template': "Gabarit des machines workers.",
    'workers-bootstrap': "Configuration de bootstrap des workers.",
  };
  const description = current && language === 'fr' && !current.optional
    ? frenchDescriptions[current.descriptor.id] || current.descriptor.description
    : current?.descriptor.description;
  const addOptional = () => {
    const descriptor = blueprint.optional.find((candidate) => candidate.id === toAdd);
    if (!descriptor) return;
    counter.current += 1;
    const key = `${descriptor.id}-${counter.current}`;
    setItems((existing) => [...existing, { key, descriptor, object: structuredClone(descriptor.object), optional: true }]);
    setSelected(key);
    setToAdd('');
  };
  const removeItem = (key) => {
    setItems((existing) => existing.filter((item) => item.key !== key));
    if (selected === key) setSelected(items[0]?.key);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div className="flex h-[90vh] w-full max-w-6xl flex-col rounded-2xl border border-gray-800 bg-gray-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-gray-800 px-6 py-4">
          <div>
            <h3 className="text-lg font-bold text-white">{t('Cluster manifests', 'Manifests du cluster')} {clusterName}</h3>
            <p className="text-xs text-gray-400">Namespace {blueprint.namespace} — {t('required manifests are preselected.', 'les manifests obligatoires sont présélectionnés.')}</p>
          </div>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-white">✕</button>
        </div>
        <div className="flex min-h-0 flex-1">
          <aside className="w-72 shrink-0 space-y-4 overflow-y-auto border-r border-gray-800 p-4">
            <div>
              <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500">{t('Required', 'Obligatoires')}</p>
              <ul className="space-y-1">
                {items.filter((item) => !item.optional).map((item) => (
                  <li key={item.key}>
                    <button type="button" onClick={() => setSelected(item.key)} className={`w-full rounded-lg px-3 py-2 text-left text-sm ${selected === item.key ? 'bg-blue-600 text-white' : 'bg-gray-950 text-gray-200 hover:bg-gray-800'}`}>
                      🔒 {item.descriptor.title}
                      <span className="block truncate text-[11px] opacity-70">{item.object.metadata?.name}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-gray-500">{t('Optional', 'Optionnels')}</p>
              <ul className="space-y-1">
                {items.filter((item) => item.optional).map((item) => (
                  <li key={item.key} className="flex gap-1">
                    <button type="button" onClick={() => setSelected(item.key)} className={`flex-1 rounded-lg px-3 py-2 text-left text-sm ${selected === item.key ? 'bg-blue-600 text-white' : 'bg-gray-950 text-gray-200 hover:bg-gray-800'}`}>
                      {item.descriptor.title}
                      <span className="block truncate text-[11px] opacity-70">{item.object.metadata?.name}</span>
                    </button>
                    <button type="button" onClick={() => removeItem(item.key)} className="px-2 text-red-400 hover:bg-red-950 rounded" title={t('Remove', 'Retirer')}>✕</button>
                  </li>
                ))}
              </ul>
              <div className="mt-2 flex gap-1">
                <select value={toAdd} onChange={(event) => setToAdd(event.target.value)} className="min-w-0 flex-1 rounded-lg border border-gray-700 bg-gray-950 px-2 py-1.5 text-xs text-white">
                  <option value="">{t('Add…', 'Ajouter…')}</option>
                  {blueprint.optional.map((descriptor) => <option key={descriptor.id} value={descriptor.id}>{descriptor.title}</option>)}
                </select>
                <button type="button" onClick={addOptional} disabled={!toAdd} className="rounded-lg bg-gray-800 px-3 text-sm disabled:opacity-50">+</button>
              </div>
            </div>
          </aside>
          <section className="min-w-0 flex-1 overflow-y-auto p-6">
            {current && (
              <>
                {description && <p className="mb-4 text-xs text-gray-400">{description}</p>}
                <ManifestEditor
                  key={current.key}
                  descriptor={current.descriptor}
                  object={current.object}
                  apiUrl={apiUrl}
                  onChange={(object) => setItems((existing) => existing.map((item) => (item.key === current.key ? { ...item, object } : item)))}
                />
              </>
            )}
          </section>
        </div>
        <div className="flex justify-end gap-3 border-t border-gray-800 px-6 py-4">
          <button type="button" onClick={onClose} className="rounded-lg bg-gray-800 px-4 py-2 text-sm">{t('Back', 'Retour')}</button>
          <button type="button" disabled={submitting} onClick={() => onSubmit(items.map((item) => item.object))} className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold disabled:opacity-50">
            {submitting ? t('Creating…', 'Création…') : t(`Create cluster (${items.length} manifests)`, `Créer le cluster (${items.length} manifests)`)}
          </button>
        </div>
      </div>
    </div>
  );
};

const ClusterCreateView = ({ providers, apiUrl, showToast, onCancel, onCreated }) => {
  const { t } = useLanguage();
  const infraProviders = providers.filter((provider) => provider.category === 'infrastructure' && provider.installed);
  const controlPlaneProviders = providers.filter((provider) => provider.category === 'controlplane' && provider.installed);
  const bootstrapProviders = providers.filter((provider) => provider.category === 'bootstrap' && provider.installed);
  const [form, setForm] = useState({
    name: '',
    kubernetesVersion: 'v1.30.0',
    infrastructureProvider: infraProviders[0]?.id || '',
    controlPlaneProvider: controlPlaneProviders[0]?.id || '',
    bootstrapProvider: bootstrapProviders[0]?.id || '',
    controlPlaneReplicas: 3,
    workerReplicas: 3
  });
  const [submitting, setSubmitting] = useState(false);

  const update = (key, value) => setForm((current) => ({ ...current, [key]: value }));
  const [blueprint, setBlueprint] = useState(null);

  const validate = async (event) => {
    event.preventDefault();
    setSubmitting(true);
    try {
      const params = new URLSearchParams({
        name: form.name,
        kubernetesVersion: form.kubernetesVersion,
        infrastructureProvider: form.infrastructureProvider,
        controlPlaneProvider: form.controlPlaneProvider,
        bootstrapProvider: form.bootstrapProvider,
        controlPlaneReplicas: String(form.controlPlaneReplicas),
        workerReplicas: String(form.workerReplicas)
      });
      const response = await fetch(`${apiUrl}/api/clusters/blueprint?${params}`);
      if (!response.ok) throw new Error(await response.text());
      setBlueprint(await response.json());
    } catch (error) {
      showToast(t(`Unable to prepare manifests: ${error.message}`, `Impossible de préparer les manifests : ${error.message}`), 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const create = async (manifests) => {
    setSubmitting(true);
    try {
      const response = await fetch(`${apiUrl}/api/clusters`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: form.name, kubernetesVersion: form.kubernetesVersion, manifests })
      });
      if (!response.ok) throw new Error(await response.text());
      onCreated(await response.json());
    } catch (error) {
      showToast(t(`Unable to create cluster: ${error.message}`, `Impossible de créer le cluster : ${error.message}`), 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const selectProvider = (label, key, options) => (
    <label className="block">
      <span className="text-xs text-gray-300">{label}</span>
      <select required value={form[key]} onChange={(event) => update(key, event.target.value)} className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white">
        <option value="">{t('Select an installed provider', 'Choisir un provider installé')}</option>
        {options.map((provider) => <option key={provider.id} value={provider.id}>{provider.name} ({provider.id})</option>)}
      </select>
    </label>
  );

  return (
    <div className="max-w-3xl">
      <h2 className="text-2xl font-bold text-white">{t('Create a workload cluster', 'Créer un cluster enfant')}</h2>
      <p className="mt-1 mb-6 text-sm text-gray-400">{t('Enter the parameters and continue to edit the Cluster API manifests.', 'Renseignez les paramètres, puis validez pour éditer les manifests Cluster API à créer.')}</p>
      {(!infraProviders.length || !controlPlaneProviders.length || !bootstrapProviders.length) && (
        <p className="mb-5 rounded-xl border border-amber-700/50 bg-amber-950/30 p-4 text-sm text-amber-300">
          {t('Install an infrastructure, control plane and bootstrap provider before creating a cluster.', 'Installez au moins un provider d’infrastructure, de control plane et de bootstrap avant de créer un cluster.')}
        </p>
      )}
      <form onSubmit={validate} className="space-y-4 rounded-2xl border border-gray-800 bg-gray-900 p-6">
        <label className="block">
          <span className="text-xs text-gray-300">{t('Cluster name', 'Nom du cluster')}</span>
          <input required pattern="[a-z0-9]([-a-z0-9]*[a-z0-9])?" maxLength="58" value={form.name} onChange={(event) => update('name', event.target.value)} className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white" />
        </label>
        <label className="block">
          <span className="text-xs text-gray-300">Version Kubernetes</span>
          <input required value={form.kubernetesVersion} onChange={(event) => update('kubernetesVersion', event.target.value)} className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white" />
        </label>
        {selectProvider(t('Infrastructure provider', 'Provider d’infrastructure'), 'infrastructureProvider', infraProviders)}
        {selectProvider(t('Control plane provider', 'Provider Control Plane'), 'controlPlaneProvider', controlPlaneProviders)}
        {selectProvider(t('Bootstrap provider', 'Provider Bootstrap'), 'bootstrapProvider', bootstrapProviders)}
        <div className="grid grid-cols-2 gap-4">
          <label className="block">
            <span className="text-xs text-gray-300">{t('Control plane replicas', 'Réplicas Control Plane')}</span>
            <input type="number" min="1" value={form.controlPlaneReplicas} onChange={(event) => update('controlPlaneReplicas', event.target.value)} className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white" />
          </label>
          <label className="block">
            <span className="text-xs text-gray-300">{t('Worker replicas', 'Réplicas workers')}</span>
            <input type="number" min="0" value={form.workerReplicas} onChange={(event) => update('workerReplicas', event.target.value)} className="mt-1 w-full bg-gray-950 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white" />
          </label>
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <button type="button" onClick={onCancel} className="px-4 py-2 rounded-lg bg-gray-800 text-sm">{t('Cancel', 'Annuler')}</button>
          <button type="submit" disabled={submitting || !infraProviders.length || !controlPlaneProviders.length || !bootstrapProviders.length} className="px-4 py-2 rounded-lg bg-blue-600 disabled:opacity-50 text-sm font-semibold">
            {submitting ? t('Preparing…', 'Validation…') : t('Continue', 'Valider')}
          </button>
        </div>
      </form>
      {blueprint && (
        <ClusterManifestsModal blueprint={blueprint} clusterName={form.name} apiUrl={apiUrl} submitting={submitting} onClose={() => setBlueprint(null)} onSubmit={create} />
      )}
    </div>
  );
};

const ClusterDetailModal = ({ cluster, apiUrl, onClose, onScaled, showToast }) => {
  const { t } = useLanguage();
  const [detail, setDetail] = useState(null);
  const [machines, setMachines] = useState([]);
  const [workerReplicas, setWorkerReplicas] = useState(3);
  const [kubeconfig, setKubeconfig] = useState(null);
  const [loading, setLoading] = useState(true);
  const [manifests, setManifests] = useState(null);
  const [manifestWarnings, setManifestWarnings] = useState([]);
  const [manifestError, setManifestError] = useState('');
  const [manifestLoading, setManifestLoading] = useState(false);
  const [selectedManifest, setSelectedManifest] = useState('');

  const loadDetails = async () => {
    setLoading(true);
    try {
      const [detailResponse, machinesResponse] = await Promise.all([
        fetch(`${apiUrl}/api/clusters/${cluster.id}`),
        fetch(`${apiUrl}/api/clusters/${cluster.id}/machines`)
      ]);
      if (!detailResponse.ok) throw new Error(await detailResponse.text());
      if (!machinesResponse.ok) throw new Error(await machinesResponse.text());
      const detailData = await detailResponse.json();
      const machineData = await machinesResponse.json();
      setDetail(detailData);
      setMachines(machineData);
      if (detailData.machineDeployments?.length) setWorkerReplicas(detailData.machineDeployments[0].replicas);
    } catch (error) {
      showToast(error.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadDetails(); }, [cluster.id]);

  const loadManifests = async () => {
    setManifestLoading(true);
    setManifestError('');
    try {
      const response = await fetch(`${apiUrl}/api/clusters/${encodeURIComponent(cluster.id)}/manifests`);
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json();
      setManifests(data.manifests);
      setManifestWarnings(data.warnings);
      setSelectedManifest('');
    } catch (error) {
      setManifestError(error.message);
    } finally {
      setManifestLoading(false);
    }
  };

  const scale = async () => {
    try {
      const response = await fetch(`${apiUrl}/api/clusters/${cluster.id}/scale`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workerReplicas: Number(workerReplicas) })
      });
      if (!response.ok) throw new Error(await response.text());
      showToast(t('Worker replicas updated.', 'Réplicas workers mises à jour.'));
      onScaled();
      loadDetails();
    } catch (error) {
      showToast(error.message, 'error');
    }
  };

  const loadKubeconfig = async () => {
    try {
      const response = await fetch(`${apiUrl}/api/clusters/${cluster.id}/kubeconfig`);
      if (!response.ok) throw new Error(await response.text());
      setKubeconfig(await response.json());
    } catch (error) {
      showToast(error.message, 'error');
    }
  };

  const copyKubeconfig = async () => {
    try {
      await navigator.clipboard.writeText(kubeconfig.rawKubeconfig);
      showToast(t('Kubeconfig copied to clipboard.', 'Kubeconfig copié dans le presse-papier.'));
    } catch (error) {
      showToast(t(`Unable to copy: ${error.message}`, `Copie impossible : ${error.message}`), 'error');
    }
  };

  const downloadKubeconfig = () => {
    const blob = new Blob([kubeconfig.rawKubeconfig], { type: 'application/yaml' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${cluster.name}.kubeconfig`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <section role="dialog" aria-modal="true" aria-labelledby="cluster-detail-title" className="max-h-[90vh] w-full max-w-5xl overflow-y-auto rounded-2xl border border-gray-700 bg-gray-900 p-6 text-white">
        <header className="mb-5 flex items-start justify-between border-b border-gray-800 pb-4">
          <div>
            <h2 id="cluster-detail-title" className="text-xl font-bold">{cluster.name}</h2>
            <p className="mt-1 text-xs text-gray-400">{detail?.namespace || cluster.namespace} · {detail?.status || cluster.status}</p>
          </div>
          <button onClick={onClose} aria-label={t('Close', 'Fermer')} className="text-xl text-gray-400 hover:text-white">×</button>
        </header>
        {loading ? <p className="py-8 text-center text-sm text-gray-400">{t('Loading…', 'Chargement…')}</p> : (
          <div className="space-y-6">
            <div className="grid gap-3 sm:grid-cols-3 text-xs">
              <div className="rounded-lg bg-gray-950 p-3">Version : <strong>{detail?.kubernetesVersion || 'N/A'}</strong></div>
              <div className="rounded-lg bg-gray-950 p-3">{t('Control plane ready:', 'Control Plane prêt :')} <strong>{detail?.controlPlaneReady ? t('Yes', 'Oui') : t('No', 'Non')}</strong></div>
              <div className="rounded-lg bg-gray-950 p-3">{t('Infrastructure ready:', 'Infrastructure prête :')} <strong>{detail?.infrastructureReady ? t('Yes', 'Oui') : t('No', 'Non')}</strong></div>
            </div>

            <section>
              <div className="mb-2 flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-sm font-semibold">{t('Cluster manifests', 'Manifests du cluster')}</h3>
                  <p className="mt-1 text-xs text-gray-400">{t('Current CAPI resources, read-only, including their status.', 'Ressources CAPI actuelles, en lecture seule, y compris leur état.')}</p>
                </div>
                <button type="button" onClick={loadManifests} disabled={manifestLoading} className="rounded-lg bg-gray-800 px-3 py-2 text-xs disabled:opacity-50">
                  {manifestLoading ? t('Loading…', 'Chargement…') : manifests ? t('Refresh manifests', 'Actualiser les manifests') : t('View manifests', 'Consulter les manifests')}
                </button>
              </div>
              {manifestError && <p role="alert" className="mb-2 text-xs text-red-400">{t('Unable to read:', 'Lecture impossible :')} {manifestError}</p>}
              {manifestWarnings.length > 0 && (
                <details className="mb-2 rounded-lg border border-amber-700 bg-amber-950/30 p-3 text-xs text-amber-300">
                  <summary>{t(`Incomplete results: ${manifestWarnings.length} resource type(s) unavailable`, `Résultats incomplets : ${manifestWarnings.length} type(s) de ressources inaccessibles`)}</summary>
                  <ul className="mt-2 space-y-1">{manifestWarnings.map((warning) => <li key={warning}>{warning}</li>)}</ul>
                </details>
              )}
              {manifests && (
                <div className="grid gap-3 md:grid-cols-[minmax(12rem,1fr)_3fr]">
                  <div className="max-h-96 space-y-1 overflow-y-auto">
                    {manifests.map((manifest, index) => (
                      <button key={`${manifest.apiVersion}/${manifest.kind}/${manifest.metadata.name}`} type="button" onClick={() => setSelectedManifest(String(index))} className={`block w-full rounded-lg px-3 py-2 text-left text-xs ${String(index) === (selectedManifest || '0') ? 'bg-blue-600' : 'bg-gray-950 hover:bg-gray-800'}`}>
                        <strong>{manifest.kind}</strong><span className="mt-1 block break-all opacity-75">{manifest.metadata.name}</span>
                      </button>
                    ))}
                    {!manifests.length && <p className="text-xs text-gray-400">{t('No accessible resources.', 'Aucune ressource accessible.')}</p>}
                  </div>
                  {manifests.length > 0 && <pre tabIndex="0" aria-label="Manifest YAML" className="max-h-96 overflow-auto rounded-lg bg-gray-950 p-4 text-xs text-gray-200">{dumpYaml(manifests[Number(selectedManifest || 0)], { noRefs: true })}</pre>}
                </div>
              )}
            </section>

            <section>
              <h3 className="mb-2 text-sm font-semibold">{t('Health conditions', 'Conditions de santé')}</h3>
              <div className="space-y-2">
                {(detail?.conditions || []).map((condition) => (
                  <details key={condition.type} className="rounded-lg bg-gray-950 p-3 text-xs">
                    <summary className="cursor-pointer">
                      {condition.type} — {condition.status} {condition.reason ? `(${condition.reason})` : ''}
                    </summary>
                    <p className="mt-2 text-gray-400">{condition.message || t('No message.', 'Aucun message.')}</p>
                    {condition.lastTransitionTime && <p className="mt-1 text-gray-500">{t('Last transition:', 'Dernière transition :')} {condition.lastTransitionTime}</p>}
                  </details>
                ))}
                {!detail?.conditions?.length && <p className="text-xs text-gray-500">{t('No conditions reported.', 'Aucune condition rapportée.')}</p>}
              </div>
            </section>

            <section>
              <h3 className="mb-2 text-sm font-semibold">MachineDeployments</h3>
              <div className="grid gap-2 sm:grid-cols-2">
                {(detail?.machineDeployments || []).map((deployment) => (
                  <div key={deployment.name} className="rounded-lg bg-gray-950 p-3 text-xs">
                    <span className="font-mono">{deployment.name}</span>
                    <span className="ml-3 text-gray-400">{deployment.readyReplicas || 0}/{deployment.replicas || 0} {t('ready', 'prêts')}</span>
                  </div>
                ))}
                {!detail?.machineDeployments?.length && <p className="text-xs text-gray-500">{t('No MachineDeployments available.', 'Aucun MachineDeployment disponible.')}</p>}
              </div>
            </section>

            <section>
              <div className="mb-2 flex items-center justify-between gap-3">
                <h3 className="text-sm font-semibold">Machines ({machines.length})</h3>
                <div className="flex items-center gap-2">
                  <label htmlFor="worker-replicas" className="text-xs text-gray-400">Workers</label>
                  <input id="worker-replicas" type="number" min="0" max="500" value={workerReplicas} onChange={(event) => setWorkerReplicas(event.target.value)} className="w-20 rounded-lg border border-gray-700 bg-gray-950 px-2 py-1 text-xs" />
                  <button onClick={scale} className="rounded-lg bg-blue-600 px-3 py-1.5 text-xs">{t('Scale', 'Redimensionner')}</button>
                </div>
              </div>
              <div className="overflow-x-auto rounded-lg border border-gray-800">
                <table className="min-w-full text-left text-xs">
                  <thead className="bg-gray-950 text-gray-400"><tr><th className="p-3">{t('Name', 'Nom')}</th><th className="p-3">{t('Role', 'Rôle')}</th><th className="p-3">Phase</th><th className="p-3">Version</th><th className="p-3">{t('Internal / external IP', 'IP interne / externe')}</th></tr></thead>
                  <tbody className="divide-y divide-gray-800">
                    {machines.map((machine) => <tr key={machine.name}><td className="p-3 font-mono">{machine.name}</td><td className="p-3">{machine.role}</td><td className="p-3">{machine.phase || 'Unknown'}</td><td className="p-3">{machine.version || 'N/A'}</td><td className="p-3">{machine.internalIP || '—'} / {machine.externalIP || '—'}</td></tr>)}
                    {!machines.length && <tr><td colSpan="5" className="p-4 text-center text-gray-500">{t('No machines yet.', 'Aucune machine pour le moment.')}</td></tr>}
                  </tbody>
                </table>
              </div>
            </section>

            <section className="rounded-xl border border-gray-800 bg-gray-950 p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div><h3 className="text-sm font-semibold">Kubeconfig</h3><p className="mt-1 text-xs text-gray-500">{kubeconfig?.serverEndpoint || t('Not loaded', 'Non chargé')}</p></div>
                {kubeconfig ? (
                  <div className="flex gap-2">
                    <button onClick={copyKubeconfig} className="rounded-lg bg-gray-800 px-3 py-2 text-xs">{t('Copy to clipboard', 'Copier dans le presse-papier')}</button>
                    <button onClick={downloadKubeconfig} className="rounded-lg bg-blue-600 px-3 py-2 text-xs">{t('Download kubeconfig', 'Télécharger le fichier kubeconfig')}</button>
                  </div>
                ) : <button onClick={loadKubeconfig} className="rounded-lg bg-blue-600 px-3 py-2 text-xs">{t('Show kubeconfig', 'Afficher le kubeconfig')}</button>}
              </div>
              {kubeconfig && <textarea readOnly value={kubeconfig.rawKubeconfig} aria-label={t('Kubeconfig contents', 'Contenu du kubeconfig')} className="mt-3 h-40 w-full resize-y rounded-lg border border-gray-800 bg-black p-3 font-mono text-[11px] text-green-300" />}
            </section>
          </div>
        )}
      </section>
    </div>
  );
};
