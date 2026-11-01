# --- Étape 1: Build (Construction) ---
# Utilise une image Golang pour compiler l'application
FROM golang:1.22-alpine AS builder

# Définir le répertoire de travail
WORKDIR /app

# Copier les fichiers de modules Go
# (Vous devez avoir un go.mod ; exécutez 'go mod init mon-backend' si ce n'est pas fait)
COPY go.mod go.sum* ./

# Télécharger les dépendances
RUN go mod download

# Copier le code source
COPY *.go .

# Compiler l'application
# -o /app/server : Spécifie le binaire de sortie
# CGO_ENABLED=0 : Désactive CGO pour une compilation statique
# -ldflags="-w -s" : Réduit la taille du binaire
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/server .

# --- Étape 2: Run (Exécution) ---
# Utilise une image Alpine minimale pour l'exécution
FROM alpine:3.18

# Définir le répertoire de travail
WORKDIR /app

RUN apk --no-cache add curl
RUN curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.11.3/clusterctl-linux-amd64 -o clusterctl && \
    chmod +x clusterctl && \
    mv clusterctl /usr/local/bin/clusterctl

# Copier le binaire compilé depuis l'étape "builder"
COPY --from=builder /app/server .

# Exposer le port 8080 (sur lequel le serveur Go écoute)
EXPOSE 8080

# Commande pour exécuter le serveur
# (Utilisation de /app/server au lieu de ./server pour un chemin absolu)
CMD ["/app/server"]
