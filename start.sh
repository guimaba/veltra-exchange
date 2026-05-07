#!/usr/bin/env bash
# Script de inicialização única (Linux / macOS / WSL).
# Sobe todo o sistema com um comando: docker compose build + up.
#
# Uso:
#   ./start.sh              # sobe em foreground (Ctrl+C para parar)
#   ./start.sh --detached   # sobe em background
#   ./start.sh --rebuild    # força rebuild de todas as imagens
#   ./start.sh --clean      # derruba tudo e apaga volumes (perde dados)

set -euo pipefail

DETACHED=0
REBUILD=0
CLEAN=0

for arg in "$@"; do
  case "$arg" in
    --detached|-d) DETACHED=1 ;;
    --rebuild)     REBUILD=1 ;;
    --clean)       CLEAN=1 ;;
    -h|--help)
      grep -E '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "Argumento desconhecido: $arg" >&2
      exit 1
      ;;
  esac
done

if ! command -v docker >/dev/null 2>&1; then
  echo "ERRO: Docker nao encontrado." >&2
  echo "Instale: https://www.docker.com/products/docker-desktop/" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "ERRO: Docker daemon nao esta rodando." >&2
  exit 1
fi

if [ "$CLEAN" -eq 1 ]; then
  echo "Limpando containers e volumes..."
  docker compose down -v
  echo "Limpeza concluida."
  exit 0
fi

BUILD_ARGS=()
if [ "$REBUILD" -eq 1 ]; then
  BUILD_ARGS+=("--no-cache")
fi

echo "Fazendo build das imagens..."
docker compose build "${BUILD_ARGS[@]}"

echo
echo "Subindo servicos..."
if [ "$DETACHED" -eq 1 ]; then
  docker compose up -d
  echo
  echo "Sistema rodando em background."
  echo "  Aplicacao Flutter Web ..... http://localhost:8080"
  echo "  Painel RabbitMQ ........... http://localhost:15672 (admin/admin)"
  echo
  echo "Comandos uteis:"
  echo "  docker compose logs -f          # ver logs"
  echo "  docker compose ps               # ver status"
  echo "  docker compose down             # parar"
  echo "  ./start.sh --clean              # parar e apagar volumes"
else
  docker compose up
fi
