import React, { useState, useEffect } from 'react';

// --- Icônes SVG pour les fournisseurs (Providers) ---
// Ces icônes sont utilisées pour rendre l'interface plus professionnelle.

const AWSIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-16 h-16 mx-auto">
    <path d="M11.996 15.228a3.155 3.155 0 01-1.04-.182 4.07 4.07 0 01-.976-.506c-.3-.21-.57-.45-.806-.72l.466.673c.18.26.33.528.45.804.12.277.18.561.18.852a2.38 2.38 0 01-.39 1.28c-.24.38-.57.69-.97.91-.4.22-.85.33-1.35.33-.57 0-1.08-.13-1.54-.4-.46-.27-.83-.63-1.12-1.08-.29-.45-.43-.98-.43-1.58 0-.6.15-1.13.44-1.6.29-.47.68-.86 1.16-1.18.49-.32 1.05-.5 1.69-.5.43 0 .83.07 1.2.2.37.13.7.3 1.01.5l-1.63-3.92-3.87-.01.9 2.15c.16.4.24.8.24 1.21 0 .5-.12.94-.36 1.32a2.4 2.4 0 01-1 .92c-.42.21-.9.31-1.44.31-.6 0-1.14-.13-1.61-.4-.47-.27-.86-.63-1.16-1.09-.3-.46-.45-.99-.45-1.6 0-.6.15-1.14.44-1.62.29-.48.68-.88 1.17-1.19.49-.31 1.05-.47 1.69-.47.45 0 .88.08 1.28.24.4.16.77.38 1.1.68l-1.18 2.82c-.1.24-.15.48-.15.73 0 .34.07.63.2.88.13.25.3.45.52.62.22.17.47.25.76.25.32 0 .6-.09.83-.28.23-.19.4-.42.52-.72l1.62-3.88h3.33L11.996 15.228zM21.05 15.1c0 .64-.15 1.2-.46 1.67-.3.47-.7.86-1.18 1.17-.48.31-1.03.47-1.64.47-.62 0-1.17-.16-1.65-.47-.48-.31-.88-.7-1.18-1.17-.3-.47-.46-1.03-.46-1.67 0-.65.15-1.2.46-1.67.3-.47.7-.86 1.18-1.17.48-.31 1.03-.47 1.65-.47.61 0 1.16.16 1.64.47.48.31.87.7 1.18 1.17.3.47.46 1.02.46 1.67zm-2.14 0c0-.36-.06-.68-.19-.97a1.6 1.6 0 00-.53-.69c-.23-.16-.5-.24-.8-.24s-.57.08-.8.24c-.23.16-.42.38-.53.69-.13.29-.19.61-.19.97s.06.68.19.97c.11.29.3.51.53.69.23.16.5.24.8.24s.57-.08.8-.24c.23-.16.42-.38.53-.69.13-.29.19-.61.19-.97z" fill="#FF9900"/>
    <path d="M19.34 6.68c-.3-.27-.64-.49-.99-.67-.36-.18-.74-.32-1.14-.42-.4-.1-.8-.14-1.22-.14-.7 0-1.34.13-1.92.4-.58.27-1.08.62-1.5 1.06-.42.44-.75.95-1 1.52-.25.57-.37 1.18-.37 1.83 0 .7.13 1.34.4 1.92.27.58.62 1.08 1.06 1.5.44.42.95.75 1.52 1 .57.25 1.18.37 1.83.37.5 0 .99-.08 1.45-.25.47-.17.9-.39 1.3-.67l.34.5c-.4.3-.85.55-1.33.74-.48.19-1 .33-1.53.41-.53.08-1.07.12-1.62.12-.9 0-1.74-.15-2.52-.46-.78-.31-1.46-.73-2.04-1.27s-1.03-1.18-1.37-1.91c-.34-.73-.51-1.51-.51-2.34 0-.85.17-1.64.52-2.38.35-.74.8-1.39 1.37-1.94.57-.55 1.25-1 2.04-1.34.79-.34 1.63-.51 2.52-.51.5 0 1 .05 1.5.14.5.09.98.23 1.44.42l-.4.6z" fill="#232F3E"/>
  </svg>
);

const AzureIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-16 h-16 mx-auto">
    <path d="M5.62 6.71l6.13 11.85 3.32-5.35-6.1-11.84-3.35 5.34z" fill="#0078D4"/>
    <path d="M12.27 19.29l6.1-11.84-3.34-5.34-6.13 11.85 3.37 5.33z" fill="#0078D4"/>
    <path d="M5.62 6.71l-2.34-.73 8.47 13.31 2.39.73L5.62 6.71z" fill="#50E6FF"/>
  </svg>
);

const GCPIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="w-16 h-16 mx-auto">
    <path d="M12.001 9.87c.21 0 .42.02.62.05 1.24.2 2.3.7 3.2 1.48 1.05.9 1.74 2.1 1.95 3.44.09.5.13 1.01.13 1.52 0 .6-.06 1.19-.18 1.77h-5.74c.02-1.04.53-1.98 1.32-2.61.6-.47 1.34-.76 2.14-.84-.2-.58-.52-1.1-.94-1.53-.42-.42-.93-.75-1.5-1v-.28z" fill="#FBBC05"/>
    <path d="M11.99 18.1c-1.78 0-3.36-.6-4.6-1.65-.96-.78-1.68-1.77-2.07-2.9-.27-.78-.4-1.6-.4-2.45 0-1.18.23-2.3.66-3.32.5-.9 1.16-1.7 1.96-2.35.79-.65 1.7-1.16 2.7-1.5.85-.3 1.75-.46 2.65-.46.9 0 1.78.15 2.6.45.83.3 1.58.73 2.24 1.28l-1.92 1.92c-.38-.3-.82-.54-1.3-.72-.48-.18-.98-.27-1.5-.27-.8 0-1.54.2-2.18.58-.64.38-1.16.9-1.52 1.55-.37.65-.55 1.4-.55 2.25 0 .8.18 1.55.54 2.2s.87 1.18 1.5 1.58c.64.4 1.36.6 2.15.6.5 0 1-.08 1.45-.25.46-.17.88-.42 1.26-.75l1.9 1.9c-.7.63-1.5 1.13-2.38 1.5-2.04.88-4.2.9-6.26.04z" fill="#EA4335"/>
    <path d="M21.1 12.18c0-1.02-.1-2.02-.28-3H12v3.7h5.1c-.2.78-.6 1.45-1.15 2l1.9 1.9c1.08-1 1.84-2.38 2.25-4.02z" fill="#4285F4"/>
    <path d="M12.001 9.87v3.7l5.1-.02c-.2.78-.6 1.45-1.15 2l1.9 1.9c1.08-1 1.84-2.38 2.25-4.02.08-.3.14-.6.18-.9v-2.68h-8.28z" fill="#4285F4"/>
  </svg>
);

const KubernetesIcon = () => (
  <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z" fill="#326CE5"/>
    <path d="M12 4.1C7.64 4.1 4.1 7.64 4.1 12s3.54 7.9 7.9 7.9 7.9-3.54 7.9-7.9-3.54-7.9-7.9-7.9zm0 14.2c-3.47 0-6.3-2.83-6.3-6.3s2.83-6.3 6.3-6.3 6.3 2.83 6.3 6.3-2.83 6.3-6.3 6.3z" fill="#326CE5"/>
    <path d="M12 10.5l-3.5 2 3.5 2 3.5-2-3.5-2zm-4.5 2.5L12 15.5l4.5-2.5L12 10.5 7.5 13z" fill="#326CE5"/>
    <path d="M12 6l-5 3v6l5 3 5-3V9l-5-3zm-3.5 3.75L12 11.5l3.5-1.75L12 8l-3.5 1.75zm0 3.5L12 15l3.5-1.75L12 11.5l-3.5 1.75z" fill="#326CE5"/>
  </svg>
);

// --- Données Mock (simulées) ---

// !! SUPPRIMÉ: Les données MOCK ne sont plus la source de vérité
// const MOCK_PROVIDERS = [
//   { id: 'aws', name: 'Amazon Web Services', icon: <AWSIcon />, status: 'installé', version: 'v2.4.1' },
// ...
// ];

// const MOCK_CLUSTERS = [
//   { id: 'prod-eu-1', name: 'prod-cluster-eu-west-1', provider: 'aws', status: 'En cours', k8sVersion: 'v1.29.3', nodes: 52, controlPlane: '3 x m5.xlarge', created: 'Il y a 2 mois' },
// ...
// ];

// --- Composant Toast (Notification) ---
// Pour simuler les retours d'action sans utiliser 'alert()'
const Toast = ({ message, type, onClose }) => {
  useEffect(() => {
    const timer = setTimeout(onClose, 3000);
    return () => clearTimeout(timer);
  }, [onClose]);

  const bgColor = type === 'error' ? 'bg-red-600' : 'bg-green-600';

  return (
    <div className={`fixed bottom-5 right-5 ${bgColor} text-white px-6 py-3 rounded-lg shadow-xl animate-bounce`}>
      {message}
    </div>
  );
};


// --- Composant Principal de l'Application ---

export default function App() {
  const [view, setView] = useState('dashboard'); // dashboard, clusters, providers, create
  const [toast, setToast] = useState(null); // { message, type }
  
  // !! MODIFIÉ: Initialiser l'état à vide
  const [clusters, setClusters] = useState([]);
  const [providers, setProviders] = useState([]);

  // !! AJOUTÉ: URL du Backend
  const API_URL = 'http://localhost:8080';

  // !! AJOUTÉ: Fonctions pour appeler l'API
  const fetchClusters = async () => {
    try {
      const response = await fetch(`${API_URL}/api/clusters`);
      if (!response.ok) throw new Error('Erreur lors du chargement des clusters');
      // S'assurer que c'est un tableau même si data est null (ce que Go peut faire)
      const data = await response.json();
      setClusters(data || []); 
    } catch (error) {
      console.error(error);
      showToast(error.message, 'error');
    }
  };

  const fetchProviders = async () => {
    try {
      const response = await fetch(`${API_URL}/api/providers`);
      if (!response.ok) throw new Error('Erreur lors du chargement des providers');
      const data = await response.json();
      setProviders(data || []);
    } catch (error) {
      console.error(error);
      showToast(error.message, 'error');
    }
  };

  // !! AJOUTÉ: useEffect pour charger les données au démarrage
  useEffect(() => {
    fetchClusters();
    fetchProviders();
  }, []); // Le tableau vide signifie "exécuter une fois au montage"

  const showToast = (message, type = 'success') => {
    setToast({ message, type });
  };
  
  // !! MODIFIÉ: addCluster pour appeler l'API POST
  const addCluster = async (newClusterFormData) => {
    // Le backend attend 'nodes' pour les workerNodes,
    // et les autres champs du formulaire
    const payload = {
        ...newClusterFormData,
        nodes: newClusterFormData.workerNodes 
    };
    
    try {
      const response = await fetch(`${API_URL}/api/clusters`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
         throw new Error('Échec de la création du cluster');
      }

      // const newClusterFromApi = await response.json(); // Le backend renvoie le cluster créé

      showToast('Cluster en cours de création !');
      setView('clusters');
      fetchClusters(); // Recharger la liste des clusters depuis le backend
      
    } catch (error) {
      console.error(error);
      showToast(error.message, 'error');
    }
  };

  // !! MODIFIÉ: deleteCluster pour appeler l'API DELETE
  const deleteCluster = async (clusterId) => {
    // J'ai gardé le window.confirm de votre code original
    if (window.confirm("Êtes-vous sûr de vouloir supprimer ce cluster ? Cette action est irréversible.")) {
      try {
        const response = await fetch(`${API_URL}/api/clusters/${clusterId}`, {
          method: 'DELETE',
        });

        if (!response.ok) {
          throw new Error('Échec de la suppression du cluster');
        }
        
        showToast('Cluster marqué pour suppression.');
        fetchClusters(); // Recharger la liste des clusters depuis le backend

      } catch (error) {
        console.error(error);
        showToast(error.message, 'error');
      }
    }
  };

  const renderView = () => {
    switch (view) {
      case 'dashboard':
        return <DashboardView clusterCount={clusters.length} nodeCount={clusters.reduce((acc, c) => acc + (c.status !== 'Erreur' ? c.nodes : 0), 0)} providersInstalled={providers.filter(p => p.status === 'installé').length} />;
      case 'clusters':
        return <ClusterListView clusters={clusters} onDelete={deleteCluster} onViewDetails={() => showToast('Navigation vers les détails (simulée).')} />;
      case 'providers':
        return <ProviderCatalogView providers={providers} onInstall={() => showToast('Installation du provider (simulée)...')} />;
      case 'create':
        return <CreateClusterView providers={providers.filter(p => p.status === 'installé')} onSubmit={addCluster} onCancel={() => setView('clusters')} />;
      default:
        return <DashboardView />;
    }
  };

  return (
    <div className="flex h-screen bg-gray-100 font-inter">
      {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
      
      {/* --- Barre Latérale (Sidebar) --- */}
      <aside className="w-64 bg-gray-900 text-gray-300 flex flex-col p-4">
        <div className="flex items-center space-x-2 mb-8 p-2">
          <KubernetesIcon />
          <span className="text-xl font-bold text-white">CAPI Dashboard</span>
        </div>
        <nav className="flex-1 flex flex-col space-y-2">
          <NavItem icon="📊" text="Tableau de bord" active={view === 'dashboard'} onClick={() => setView('dashboard')} />
          <NavItem icon="🧩" text="Clusters" active={view === 'clusters'} onClick={() => setView('clusters')} />
          <NavItem icon="📦" text="Catalogue des Providers" active={view === 'providers'} onClick={() => setView('providers')} />
        </nav>
        <div className="mt-auto">
          <button 
            onClick={() => setView('create')}
            className="w-full bg-blue-600 text-white py-3 px-4 rounded-lg font-semibold hover:bg-blue-700 transition duration-200 shadow-md flex items-center justify-center space-x-2">
            <span>🚀</span>
            <span>Créer un Cluster</span>
          </button>
        </div>
      </aside>

      {/* --- Contenu Principal --- */}
      <main className="flex-1 overflow-y-auto p-8">
        {renderView()}
      </main>
    </div>
  );
}

// --- Composant de Navigation ---
const NavItem = ({ icon, text, active, onClick }) => (
  <button
    onClick={onClick}
    className={`flex items-center space-x-3 p-3 rounded-lg transition duration-200 w-full text-left ${
      active
        ? 'bg-blue-600 text-white shadow-lg'
        : 'hover:bg-gray-800 hover:text-white'
    }`}
  >
    <span className="text-xl">{icon}</span>
    <span className="font-medium">{text}</span>
  </button>
);

// --- Vue: Tableau de bord ---
const DashboardView = ({ clusterCount, nodeCount, providersInstalled }) => (
  <>
    <h1 className="text-3xl font-bold text-gray-900 mb-6">Tableau de bord</h1>
    <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
      <StatCard title="Clusters Gérés" value={clusterCount} unit="clusters" />
      <StatCard title="Nœuds Actifs (Total)" value={nodeCount} unit="nœuds" />
      <StatCard title="Providers Installés" value={providersInstalled} unit="providers" />
    </div>
    
    <div className="mt-10 bg-white p-6 rounded-lg shadow-lg">
      <h2 className="text-xl font-semibold mb-4">Bienvenue sur votre Dashboard Cluster API</h2>
      <p className="text-gray-700">
        Utilisez le menu de gauche pour naviguer, consulter vos clusters existants, 
        gérer les providers d'infrastructure ou déployer de nouveaux clusters Kubernetes.
      </p>
    </div>
  </>
);

const StatCard = ({ title, value, unit }) => (
  <div className="bg-white p-6 rounded-lg shadow-lg border border-gray-200">
    <h3 className="text-sm font-medium text-gray-500 uppercase tracking-wider">{title}</h3>
    <div className="mt-2 flex items-baseline space-x-2">
      <span className="text-4xl font-bold text-blue-600">{value}</span>
      <span className="text-gray-600">{unit}</span>
    </div>
  </div>
);

// --- Vue: Catalogue des Providers ---
const ProviderCatalogView = ({ providers, onInstall }) => (
  <>
    <h1 className="text-3xl font-bold text-gray-900 mb-6">Catalogue des Providers</h1>
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {providers.map(p => (
        <ProviderCard key={p.id} provider={p} onInstall={onInstall} />
      ))}
    </div>
  </>
);

const ProviderCard = ({ provider, onInstall }) => {
  const getStatusChip = (status) => {
    switch(status) {
      case 'installé':
        return <span className="px-3 py-1 bg-green-200 text-green-800 rounded-full text-xs font-medium">Installé</span>;
      case 'disponible':
        return <span className="px-3 py-1 bg-blue-200 text-blue-800 rounded-full text-xs font-medium">Disponible</span>;
      default:
        return <span className="px-3 py-1 bg-gray-200 text-gray-800 rounded-full text-xs font-medium">Non-installé</span>;
    }
  };

  return (
    <div className="bg-white p-6 rounded-lg shadow-lg border border-gray-200 flex flex-col">
      <div className="flex-shrink-0 mb-4">{provider.icon}</div>
      <h2 className="text-xl font-semibold text-gray-900">{provider.name}</h2>
      <p className="text-gray-500 mb-4">Version: {provider.version}</p>
      <div className="flex justify-between items-center mb-6">
        {getStatusChip(provider.status)}
      </div>
      <button 
        onClick={() => onInstall(provider.id)}
        disabled={provider.status === 'installé'}
        className={`w-full mt-auto px-4 py-2 rounded-lg font-semibold transition duration-200 ${
          provider.status === 'installé'
            ? 'bg-gray-300 text-gray-500 cursor-not-allowed'
            : 'bg-blue-600 text-white hover:bg-blue-700'
        }`}
      >
        {provider.status === 'installé' ? 'Déjà Installé' : 'Installer'}
      </button>
    </div>
  );
};

// --- Vue: Liste des Clusters ---
const ClusterListView = ({ clusters, onDelete, onViewDetails }) => (
  <>
    <h1 className="text-3xl font-bold text-gray-900 mb-6">Clusters Kubernetes</h1>
    <div className="bg-white shadow-lg rounded-lg overflow-hidden border border-gray-200">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Nom du Cluster</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Statut</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Provider</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Version K8s</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Nœuds</th>
            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {clusters.map(c => <ClusterRow key={c.id} cluster={c} onDelete={onDelete} onViewDetails={onViewDetails} />)}
        </tbody>
      </table>
    </div>
  </>
);

const ClusterRow = ({ cluster, onDelete, onViewDetails }) => {
  const getStatusIndicator = (status) => {
    switch (status) {
      case 'En cours':
        return <span className="w-3 h-3 bg-green-500 rounded-full animate-pulse"></span>;
      case 'Provisionnement':
        return <span className="w-3 h-3 bg-yellow-500 rounded-full animate-pulse"></span>;
      case 'Erreur':
        return <span className="w-3 h-3 bg-red-500 rounded-full"></span>;
      default:
        return <span className="w-3 h-3 bg-gray-400 rounded-full"></span>;
    }
  };

  const providerIcon = (provider) => {
    if (provider === 'aws') return <AWSIcon />;
    if (provider === 'azure') return <AzureIcon />;
    if (provider === 'gcp') return <GCPIcon />;
    return <KubernetesIcon />;
  }

  return (
    <tr>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="font-medium text-gray-900">{cluster.name}</div>
        <div className="text-sm text-gray-500">Créé: {cluster.created}</div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="flex items-center space-x-2">
          {getStatusIndicator(cluster.status)}
          <span className="text-sm font-medium text-gray-800">{cluster.status}</span>
        </div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="w-8 h-8 transform scale-75 -ml-2">{providerIcon(cluster.provider)}</div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">{cluster.k8sVersion}</td>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-600">{cluster.nodes}</td>
      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
        <button onClick={() => onViewDetails(cluster.id)} className="text-blue-600 hover:text-blue-900">Détails</button>
        <button onClick={() => onDelete(cluster.id)} className="text-red-600 hover:text-red-900">Supprimer</button>
      </td>
    </tr>
  );
};


// --- Vue: Création de Cluster ---
const CreateClusterView = ({ providers, onSubmit, onCancel }) => {
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({
    provider: providers.length > 0 ? providers[0].id : '',
    name: 'mon-nouveau-cluster',
    k8sVersion: 'v1.30.0',
    controlPlaneNodes: 3,
    controlPlaneType: 'm5.xlarge', // Type par défaut pour AWS (simulé)
    workerNodes: 3,
    workerType: 'm5.xlarge',
  });

  const handleChange = (e) => {
    const { name, value, type } = e.target;
    setFormData(prev => ({ ...prev, [name]: type === 'number' ? parseInt(value) : value }));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    onSubmit(formData);
  };
  
  const providerTypes = {
    aws: ['m5.xlarge', 'm5.2xlarge', 't3.medium', 't3.large'],
    azure: ['Standard_D4s_v3', 'Standard_D8s_v3', 'Standard_B2ms'],
    gcp: ['n1-standard-2', 'n1-standard-4', 'e2-medium']
  };
  
  const currentInstanceTypes = providerTypes[formData.provider] || [];

  return (
    <form onSubmit={handleSubmit} className="max-w-2xl mx-auto bg-white p-8 rounded-lg shadow-lg border border-gray-200">
      <h1 className="text-3xl font-bold text-gray-900 mb-8">Créer un nouveau Cluster</h1>

      {/* --- Section 1: Configuration Générale --- */}
      <section className="mb-8">
        <h2 className="text-xl font-semibold mb-4 border-b pb-2">1. Configuration Générale</h2>
        
        <label className="block text-sm font-medium text-gray-700 mb-2">Nom du Cluster</label>
        <input 
          type="text" 
          name="name"
          value={formData.name}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500" 
        />

        <label className="block text-sm font-medium text-gray-700 mt-4 mb-2">Provider d'Infrastructure</label>
        <select 
          name="provider" 
          value={formData.provider}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        >
          {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
        
        <label className="block text-sm font-medium text-gray-700 mt-4 mb-2">Version Kubernetes</label>
        <select 
          name="k8sVersion" 
          value={formData.k8sVersion}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        >
          <option>v1.30.0</option>
          <option>v1.29.3</option>
          <option>v1.28.5</option>
        </select>
      </section>

      {/* --- Section 2: Control Plane --- */}
      <section className="mb-8">
        <h2 className="text-xl font-semibold mb-4 border-b pb-2">2. Control Plane</h2>
        
        <label className="block text-sm font-medium text-gray-700 mb-2">Nombre de Nœuds (HA)</label>
        <select 
          name="controlPlaneNodes" 
          value={formData.controlPlaneNodes}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        >
          <option value={1}>1 (Non-HA)</option>
          <option value={3}>3 (Recommandé)</option>
          <option value={5}>5</option>
        </select>
        
        <label className="block text-sm font-medium text-gray-700 mt-4 mb-2">Type d'Instance</label>
        <select 
          name="controlPlaneType" 
          value={formData.controlPlaneType}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        >
          {currentInstanceTypes.map(type => <option key={type} value={type}>{type}</option>)}
        </select>
      </section>

      {/* --- Section 3: Pool de Workers (MachineDeployment) --- */}
      <section className="mb-8">
        <h2 className="text-xl font-semibold mb-4 border-b pb-2">3. Pool de Workers (default-pool)</h2>
        
        <label className="block text-sm font-medium text-gray-700 mb-2">Nombre de Nœuds (Initial)</label>
        <input 
          type="number"
          name="workerNodes"
          min="1"
          max="100"
          value={formData.workerNodes}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500" 
        />
        
        <label className="block text-sm font-medium text-gray-700 mt-4 mb-2">Type d'Instance</label>
        <select 
          name="workerType" 
          value={formData.workerType}
          onChange={handleChange}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
        >
          {currentInstanceTypes.map(type => <option key={type} value={type}>{type}</option>)}
        </select>
        {/* Dans une vraie app, on ajouterait ici l'autoscaling, les labels, taints, etc. */}
      </section>

      {/* --- Actions --- */}
      <div className="flex justify-end space-x-4">
        <button 
          type="button" 
          onClick={onCancel}
          className="px-6 py-2 bg-gray-200 text-gray-800 rounded-lg font-semibold hover:bg-gray-300 transition duration-200"
        >
          Annuler
        </button>
        <button 
          type="submit"
          className="px-6 py-2 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 transition duration-200 shadow-md"
        >
          🚀 Déployer le Cluster
        </button>
      </div>
    </form>
  );
};


