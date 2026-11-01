import React, { useState, useEffect } from 'react';

// --- Icônes SVG pour les fournisseurs (Providers) ---
const AWSIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-12 h-12 mx-auto">
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14H9v-2h2v2zm0-4H9V7h2v5z" fill="#FF9900"/>
    <circle cx="12" cy="12" r="9" stroke="#FF9900" strokeWidth="2"/>
  </svg>
);

const AzureIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-12 h-12 mx-auto">
    <path d="M5.62 6.71l6.13 11.85 3.32-5.35-6.1-11.84-3.35 5.34z" fill="#0078D4"/>
    <path d="M12.27 19.29l6.1-11.84-3.34-5.34-6.13 11.85 3.37 5.33z" fill="#0078D4"/>
    <path d="M5.62 6.71l-2.34-.73 8.47 13.31 2.39.73L5.62 6.71z" fill="#50E6FF"/>
  </svg>
);

const GCPIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-12 h-12 mx-auto">
    <circle cx="12" cy="12" r="9" stroke="#4285F4" strokeWidth="2"/>
    <path d="M12 7v5l3 3" stroke="#EA4335" strokeWidth="2" strokeLinecap="round"/>
  </svg>
);

const DockerIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-12 h-12 mx-auto">
    <path d="M22 13.5c-.5-.3-1.4-.4-2-.2-.3-.8-1-1.3-2-1.3-.2 0-.4 0-.6.1-.5-1.2-1.7-2.1-3.2-2.1H14V8h2V6h-2V4h-2v2h-2V4H8v2H6v2h2v2H3c-.6 0-1 .4-1 1 0 4.4 3.6 8 8 8h6c3.9 0 7-3.1 7-7 0-.5-.4-.9-1-.5z" fill="#2496ED"/>
  </svg>
);

const KubernetesIcon = () => (
  <svg className="w-6 h-6 text-blue-400" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 2L2 7.5v9L12 22l10-5.5v-9L12 2zm0 2.2l7.5 4.1-3 1.6-4.5-2.4-4.5 2.4-3-1.6L12 4.2zm-8 6.4l3 1.6v3.3l-3-1.6v-3.3zm5 4.3v-2.7l3 1.6 3-1.6v2.7l-3 1.6-3-1.6zm8-1v-3.3l3-1.6v3.3l-3 1.6z"/>
  </svg>
);

const ProviderIcon = ({ id }) => {
  const lower = (id || '').toLowerCase();
  if (lower.includes('aws')) return <AWSIcon />;
  if (lower.includes('azure')) return <AzureIcon />;
  if (lower.includes('gcp') || lower.includes('google')) return <GCPIcon />;
  if (lower.includes('docker')) return <DockerIcon />;
  return <KubernetesIcon />;
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
const JobModal = ({ jobName, onClose, apiUrl }) => {
  const [jobStatus, setJobStatus] = useState(null);
  const [logs, setLogs] = useState('Chargement des logs...');

  useEffect(() => {
    if (!jobName) return;

    const fetchStatus = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/jobs/${jobName}/status`);
        if (res.ok) {
          const data = await res.json();
          setJobStatus(data);
        }
      } catch (err) {
        console.error(err);
      }
    };

    const fetchLogs = async () => {
      try {
        const res = await fetch(`${apiUrl}/api/jobs/${jobName}/logs`);
        if (res.ok) {
          const text = await res.text();
          setLogs(text || 'Aucun log pour le moment...');
        }
      } catch (err) {
        console.error(err);
      }
    };

    fetchStatus();
    fetchLogs();
    const interval = setInterval(() => {
      fetchStatus();
      fetchLogs();
    }, 2500);

    return () => clearInterval(interval);
  }, [jobName, apiUrl]);

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div className="bg-gray-900 border border-gray-700 rounded-2xl max-w-3xl w-full p-6 text-white shadow-2xl flex flex-col max-h-[85vh]">
        <div className="flex justify-between items-center pb-4 border-b border-gray-800">
          <div>
            <h3 className="text-lg font-bold">Suivi du Job : {jobName}</h3>
            <p className="text-xs text-gray-400">Statut: <span className="font-semibold text-blue-400">{jobStatus?.status || 'Vérification...'}</span></p>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-white text-xl">✕</button>
        </div>
        <div className="flex-1 my-4 overflow-hidden flex flex-col">
          <label className="text-xs font-semibold text-gray-400 mb-2">Logs d'exécution clusterctl :</label>
          <pre className="flex-1 bg-black/80 rounded-xl p-4 font-mono text-xs text-green-400 overflow-y-auto whitespace-pre-wrap leading-relaxed border border-gray-800">
            {logs}
          </pre>
        </div>
        <div className="flex justify-end pt-2">
          <button onClick={onClose} className="px-5 py-2 bg-gray-800 hover:bg-gray-700 rounded-xl text-sm font-medium transition">
            Fermer
          </button>
        </div>
      </div>
    </div>
  );
};

// --- Composant Principal de l'Application ---
export default function App() {
  const [view, setView] = useState('dashboard'); // dashboard, clusters, providers, crds, create
  const [toast, setToast] = useState(null);
  const [activeJob, setActiveJob] = useState(null);

  const [clusters, setClusters] = useState([]);
  const [providers, setProviders] = useState([]);
  const [crds, setCrds] = useState([]);
  const [selectedCRD, setSelectedCRD] = useState(null);
  const [crdSchema, setCrdSchema] = useState(null);
  const [loadingSchema, setLoadingSchema] = useState(false);

  // Configuration dynamique de l'URL Backend
  const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8085';

  const showToast = (message, type = 'success') => setToast({ message, type });

  const fetchClusters = async () => {
    try {
      const response = await fetch(`${API_URL}/api/clusters`);
      if (response.ok) {
        const data = await response.json();
        setClusters(data || []);
      }
    } catch (error) {
      console.error(error);
    }
  };

  const fetchProviders = async () => {
    try {
      const response = await fetch(`${API_URL}/api/providers`);
      if (response.ok) {
        const data = await response.json();
        setProviders(data || []);
      }
    } catch (error) {
      console.error(error);
    }
  };

  const fetchCRDs = async () => {
    try {
      const response = await fetch(`${API_URL}/api/crds`);
      if (response.ok) {
        const data = await response.json();
        setCrds(data || []);
      }
    } catch (error) {
      console.error(error);
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
        showToast(`Impossible de charger le schéma pour ${crdName}`, 'error');
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
        showToast(`Installation initiée (Job: ${data.jobName})`);
        setActiveJob(data.jobName);
        fetchProviders();
        fetchCRDs();
      } else {
        showToast(data.message || 'Erreur lors de l’installation', 'error');
      }
    } catch (err) {
      showToast(err.message, 'error');
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
            onInstall={handleInstallProvider}
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
            onDelete={async (id) => {
              if (window.confirm("Confirmer la suppression du cluster ?")) {
                await fetch(`${API_URL}/api/clusters/${id}`, { method: 'DELETE' });
                showToast("Suppression en cours...");
                fetchClusters();
              }
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
      {activeJob && <JobModal jobName={activeJob} apiUrl={API_URL} onClose={() => setActiveJob(null)} />}

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
          <NavItem icon="📊" text="Tableau de bord" active={view === 'dashboard'} onClick={() => setView('dashboard')} />
          <NavItem icon="📦" text="Providers CAPI" count={providers.length} active={view === 'providers'} onClick={() => setView('providers')} />
          <NavItem icon="🧩" text="CRDs des Providers" count={crds.length} active={view === 'crds'} onClick={() => setView('crds')} />
          <NavItem icon="☸️" text="Clusters Enfants" count={clusters.length} active={view === 'clusters'} onClick={() => setView('clusters')} />
        </nav>

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
const DashboardView = ({ clusterCount, nodeCount, providersInstalled, totalProviders, crdsCount, onNavigate }) => (
  <div>
    <div className="mb-8">
      <h2 className="text-2xl font-bold text-white tracking-tight">Vue d'ensemble</h2>
      <p className="text-gray-400 text-sm mt-1">Supervision de votre cluster de management et des composants CAPI</p>
    </div>

    <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
      <StatCard title="Providers Installés" value={`${providersInstalled} / ${totalProviders}`} subtitle="Actifs sur le cluster" icon="📦" onClick={() => onNavigate('providers')} />
      <StatCard title="CRDs Introspectées" value={crdsCount} subtitle="Schémas configurables" icon="🧩" onClick={() => onNavigate('crds')} />
      <StatCard title="Clusters Enfants" value={clusterCount} subtitle="Workload clusters" icon="☸️" onClick={() => onNavigate('clusters')} />
      <StatCard title="Nœuds Gérés" value={nodeCount} subtitle="Total CP + Workers" icon="🖥️" />
    </div>

    <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
      <h3 className="text-base font-semibold text-white mb-3">Capacités Actives</h3>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-blue-400 font-semibold mb-1">✓ Découverte Dynamique</div>
          <p className="text-gray-400 text-xs">Catalogue Cluster API synchronisé avec plus de 40 providers et gestion des versions.</p>
        </div>
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-emerald-400 font-semibold mb-1">✓ Introspection des CRDs</div>
          <p className="text-gray-400 text-xs">Extraction automatique des schémas OpenAPI v3 pour générer des formulaires à la volée.</p>
        </div>
        <div className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
          <div className="text-purple-400 font-semibold mb-1">✓ Jobs Asynchrones</div>
          <p className="text-gray-400 text-xs">Installation et désinstallation non-bloquante via K8s Jobs avec suivi des logs en temps réel.</p>
        </div>
      </div>
    </div>
  </div>
);

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
const ProviderCatalogView = ({ providers, onInstall, onRefresh }) => {
  const [selectedCategory, setSelectedCategory] = useState('all');
  const [installingProvider, setInstallingProvider] = useState(null);
  const [customVersion, setCustomVersion] = useState('');

  const categories = [
    { id: 'all', label: 'Tous' },
    { id: 'infrastructure', label: 'Infrastructure' },
    { id: 'controlplane', label: 'Control Plane' },
    { id: 'bootstrap', label: 'Bootstrap' },
    { id: 'core', label: 'Core' }
  ];

  const filtered = providers.filter(p => selectedCategory === 'all' || p.category === selectedCategory);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-white tracking-tight">Catalogue des Providers CAPI</h2>
          <p className="text-gray-400 text-sm mt-1">Gérez l'ensemble des providers supportés et leurs versions</p>
        </div>
        <button onClick={onRefresh} className="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-xs rounded-xl font-medium transition flex items-center space-x-2">
          <span>🔄</span>
          <span>Actualiser</span>
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
          <div key={p.id} className="bg-gray-900 border border-gray-800 rounded-2xl p-5 flex flex-col justify-between hover:border-gray-700 transition">
            <div>
              <div className="flex items-start justify-between mb-4">
                <div className="p-3 bg-gray-950 border border-gray-800 rounded-xl">
                  <ProviderIcon id={p.id} />
                </div>
                <div className="flex flex-col items-end space-y-1">
                  <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                    p.installed ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-gray-800 text-gray-400'
                  }`}>
                    {p.installed ? '✓ Installé' : 'Disponible'}
                  </span>
                  <span className="text-[10px] text-gray-500 uppercase tracking-wider">{p.category}</span>
                </div>
              </div>

              <h3 className="text-base font-bold text-white">{p.name}</h3>
              <p className="text-xs text-gray-400 mt-1">Version: <span className="font-mono text-blue-400 font-semibold">{p.version || 'latest'}</span></p>

              {/* CRDs associées */}
              {p.crds && p.crds.length > 0 && (
                <div className="mt-4 pt-3 border-t border-gray-800">
                  <div className="text-[11px] font-medium text-gray-400 mb-1.5">CRDs actives ({p.crds.length}) :</div>
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
                <button disabled className="w-full py-2 bg-emerald-950/40 text-emerald-400 text-xs font-semibold rounded-xl border border-emerald-800/40 cursor-default">
                  ✓ Composant Opérationnel
                </button>
              ) : (
                <button
                  onClick={() => {
                    setInstallingProvider(p);
                    setCustomVersion(p.version || '');
                  }}
                  className="w-full py-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded-xl transition shadow-md shadow-blue-600/20"
                >
                  Installer ce Provider
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
            <h3 className="text-lg font-bold mb-1">Installer {installingProvider.name}</h3>
            <p className="text-xs text-gray-400 mb-4">Un Job Kubernetes exécutera `clusterctl init` pour déployer ce provider.</p>

            <label className="block text-xs font-semibold text-gray-300 mb-1">Version cible :</label>
            <input 
              type="text"
              value={customVersion}
              onChange={(e) => setCustomVersion(e.target.value)}
              placeholder="ex: v2.4.0 (ou laisser vide pour version par défaut)"
              className="w-full bg-gray-950 border border-gray-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-blue-500 mb-5"
            />

            <div className="flex justify-end space-x-3">
              <button 
                onClick={() => setInstallingProvider(null)}
                className="px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-xl text-xs font-medium transition"
              >
                Annuler
              </button>
              <button 
                onClick={() => {
                  const id = installingProvider.id;
                  setInstallingProvider(null);
                  onInstall(id, customVersion);
                }}
                className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded-xl text-xs font-medium transition font-semibold"
              >
                Lancer l'installation
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
  return (
    <div>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-white tracking-tight">CRDs Introspectées & Schémas OpenAPI v3</h2>
        <p className="text-gray-400 text-sm mt-1">Schémas extraits dynamiquement depuis Kubernetes pour alimenter les formulaires de configuration</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Liste des CRDs */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-4 h-[75vh] flex flex-col">
          <h3 className="text-xs font-bold text-gray-400 uppercase tracking-wider mb-3 px-2">Ressources Détectées ({crds.length})</h3>
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
            <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">Chargement du schéma OpenAPI v3...</div>
          ) : schemaData ? (
            <div className="flex-1 flex flex-col overflow-hidden">
              <div className="pb-4 border-b border-gray-800 mb-4 flex justify-between items-start">
                <div>
                  <h3 className="text-lg font-bold text-white">{schemaData.kind}</h3>
                  <p className="text-xs text-blue-400 font-mono mt-0.5">apiVersion: {schemaData.apiVersion}</p>
                </div>
                <span className="text-xs px-2.5 py-1 rounded-full bg-blue-500/20 text-blue-400 border border-blue-500/30">
                  Sous-schéma .spec normalisé
                </span>
              </div>

              {/* Aperçu des propriétés configurables */}
              <div className="flex-1 overflow-y-auto space-y-4 pr-2">
                <div className="text-xs font-semibold text-gray-400 uppercase tracking-wider">Champs de configuration détectés :</div>
                {schemaData.schema?.properties ? (
                  Object.entries(schemaData.schema.properties).map(([field, def]) => (
                    <div key={field} className="bg-gray-950 p-4 rounded-xl border border-gray-800/80">
                      <div className="flex items-center justify-between mb-1">
                        <span className="font-mono text-sm font-bold text-white">{field}</span>
                        <div className="flex space-x-2 items-center">
                          <span className="text-[10px] px-2 py-0.5 bg-gray-800 rounded text-purple-400 font-mono">
                            {def.type || 'object'}
                          </span>
                          {schemaData.schema.required?.includes(field) && (
                            <span className="text-[10px] px-2 py-0.5 bg-red-950/60 text-red-400 rounded border border-red-800/40">
                              Requis
                            </span>
                          )}
                        </div>
                      </div>
                      <p className="text-xs text-gray-400 mt-1 leading-relaxed">{def.description || 'Aucune description disponible.'}</p>
                      {def.enum && (
                        <div className="mt-2 text-xs text-gray-500 flex items-center space-x-1">
                          <span className="text-gray-400">Valeurs autorisées :</span>
                          <span className="font-mono text-blue-400">{def.enum.join(', ')}</span>
                        </div>
                      )}
                    </div>
                  ))
                ) : (
                  <pre className="bg-black/60 p-4 rounded-xl text-xs font-mono text-green-400 overflow-x-auto">
                    {JSON.stringify(schemaData.schema, null, 2)}
                  </pre>
                )}
              </div>
            </div>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-gray-500 text-sm">
              <span className="text-3xl mb-2">👈</span>
              <span>Sélectionnez une CRD dans la liste pour inspecter son schéma de formulaire.</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

// --- Vue 4: Liste des Clusters ---
const ClusterListView = ({ clusters, onDelete }) => (
  <div>
    <div className="mb-6">
      <h2 className="text-2xl font-bold text-white tracking-tight">Clusters Kubernetes Enfants</h2>
      <p className="text-gray-400 text-sm mt-1">Clusters provisionnés et gérés via Cluster API</p>
    </div>

    {clusters.length === 0 ? (
      <div className="bg-gray-900 border border-gray-800 rounded-2xl p-12 text-center text-gray-500">
        <span className="text-4xl block mb-3">☸️</span>
        <p className="text-base font-medium text-gray-400">Aucun cluster enfant déployé pour l'instant.</p>
        <p className="text-xs text-gray-500 mt-1">Installez un provider d'infrastructure pour provisionner votre premier cluster.</p>
      </div>
    ) : (
      <div className="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden shadow-xl">
        <table className="min-w-full divide-y divide-gray-800 text-left text-sm">
          <thead className="bg-gray-950 text-gray-400 text-xs uppercase tracking-wider font-semibold">
            <tr>
              <th className="px-6 py-4">Nom du Cluster</th>
              <th className="px-6 py-4">Statut</th>
              <th className="px-6 py-4">Infrastructure</th>
              <th className="px-6 py-4">Version K8s</th>
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
                <td className="px-6 py-4 text-right">
                  <button onClick={() => onDelete(c.id)} className="text-red-400 hover:text-red-300 font-medium text-xs">
                    Supprimer
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )}
  </div>
);
