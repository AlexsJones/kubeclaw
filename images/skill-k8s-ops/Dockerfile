# Skill sidecar: k8s-ops
# Provides kubectl for Kubernetes operations skills.
# Runs as non-root (UID 1000) with a read-only rootfs.
FROM bitnami/kubectl:1.31 AS kubectl

FROM alpine:3.20

# Install minimal utilities the agent might need
RUN apk add --no-cache \
    bash \
    curl \
    jq \
    && adduser -D -u 1000 agent

COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl

USER 1000
WORKDIR /workspace

# The agent controller sets the command; default to sleep so the
# sidecar stays alive for the lifetime of the agent run.
CMD ["sleep", "infinity"]
