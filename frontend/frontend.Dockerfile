# --- Étape 1: Build (Construction) ---
# Utilise une image Node.js pour construire l'application React
FROM node:20-alpine AS builder

# Définir le répertoire de travail
WORKDIR /app

# Copier package.json et package-lock.json
COPY package.json package-lock.json* ./

# Installer les dépendances
# Note: Si vous n'avez pas de package-lock.json, utilisez 'npm install'
RUN npm ci

# Copier le reste du code source
# (Assurez-vous que capi-dashboard.jsx est dans src/App.jsx)
COPY . .

# Construire l'application pour la production
RUN npm run build

# --- Étape 2: Serve (Service) ---
# Utilise une image Nginx légère pour servir les fichiers statiques
FROM nginx:1.25-alpine

# Copier les fichiers statiques construits de l'étape "builder"
# Le dossier 'dist' est la sortie par défaut de 'vite build'
COPY --from=builder /app/dist /usr/share/nginx/html

# Copier la configuration Nginx personnalisée
# Ceci est crucial pour que le routage React (SPA) fonctionne
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Exposer le port 80 (par défaut pour Nginx)
EXPOSE 80

# Commande pour démarrer Nginx
CMD ["nginx", "-g", "daemon off;"]
