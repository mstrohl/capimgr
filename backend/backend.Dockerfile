FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/server .

FROM alpine:3.20
ARG TARGETARCH=amd64
ARG CLUSTERCTL_VERSION=v1.11.3
WORKDIR /app
RUN apk --no-cache add curl ca-certificates && \
    curl -fsSL -o /usr/local/bin/clusterctl \
      "https://github.com/kubernetes-sigs/cluster-api/releases/download/${CLUSTERCTL_VERSION}/clusterctl-linux-${TARGETARCH}" && \
    chmod +x /usr/local/bin/clusterctl && \
    apk del curl
COPY --from=builder /app/server .
COPY openapi.yaml .
COPY swagger-ui ./swagger-ui
EXPOSE 8080
CMD ["/app/server"]
