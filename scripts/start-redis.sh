#!/usr/bin/env bash
set -euo pipefail

CONTAINER="vectoreologist-redis"
IMAGE="redis:7-alpine"
PORT="${REDIS_PORT:-6379}"
MAX_MEMORY="${REDIS_MAXMEM:-2gb}"

# ── helpers ───────────────────────────────────────────────────────────────────

die()  { echo "error: $*" >&2; exit 1; }
info() { echo "  $*"; }

# ── preflight ─────────────────────────────────────────────────────────────────

command -v docker >/dev/null 2>&1 || die "Docker is not installed. Install it from https://docs.docker.com/get-docker/"

docker info >/dev/null 2>&1 || die "Docker daemon is not running. Start Docker and try again."

# ── already running? ──────────────────────────────────────────────────────────

if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    info "Redis container '${CONTAINER}' is already running."
    info "URL: redis://localhost:${PORT}"
    exit 0
fi

# ── stopped but exists? ───────────────────────────────────────────────────────

if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    info "Starting existing container '${CONTAINER}'…"
    docker start "${CONTAINER}" >/dev/null
else
    # ── fresh install ─────────────────────────────────────────────────────────
    info "Pulling ${IMAGE}…"
    docker pull "${IMAGE}" >/dev/null

    info "Creating container '${CONTAINER}' on port ${PORT}…"
    docker run -d \
        --name "${CONTAINER}" \
        --restart unless-stopped \
        -p "${PORT}:6379" \
        -v vectoreologist-redis-data:/data \
        "${IMAGE}" \
        redis-server \
            --maxmemory "${MAX_MEMORY}" \
            --maxmemory-policy allkeys-lru \
            --save 60 1 \
        >/dev/null
fi

# ── wait for ready ────────────────────────────────────────────────────────────

info "Waiting for Redis to accept connections…"
for i in $(seq 1 20); do
    if docker exec "${CONTAINER}" redis-cli ping 2>/dev/null | grep -q PONG; then
        echo ""
        echo "✓ Redis is ready"
        echo "  URL:       redis://localhost:${PORT}"
        echo "  Container: ${CONTAINER}"
        echo "  Memory:    ${MAX_MEMORY}"
        echo ""
        echo "  Run vectoreologist with:"
        echo "    ./vectoreologist --collection <name> --redis-url redis://localhost:${PORT}"
        echo ""
        echo "  To stop:   docker stop ${CONTAINER}"
        echo "  To remove: docker rm -v ${CONTAINER}"
        exit 0
    fi
    sleep 0.5
done

die "Redis did not become ready in time. Check: docker logs ${CONTAINER}"
