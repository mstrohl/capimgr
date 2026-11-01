FROM alpine:3.20
ARG TARGETARCH=amd64
ARG CLUSTERCTL_VERSION=v1.11.3
RUN apk --no-cache add curl ca-certificates && \
    curl -fsSL -o /usr/local/bin/clusterctl \
      "https://github.com/kubernetes-sigs/cluster-api/releases/download/${CLUSTERCTL_VERSION}/clusterctl-linux-${TARGETARCH}" && \
    chmod +x /usr/local/bin/clusterctl && \
    apk del curl && \
    clusterctl version
ENTRYPOINT []
CMD ["clusterctl", "version"]
