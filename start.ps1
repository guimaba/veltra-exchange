# Script de inicialização única (Windows / PowerShell).
# Sobe todo o sistema com um comando: docker compose build + up.
#
# Uso:
#   .\start.ps1            # sobe em foreground (Ctrl+C para parar)
#   .\start.ps1 -Detached  # sobe em background
#   .\start.ps1 -Rebuild   # força rebuild de todas as imagens
#   .\start.ps1 -Clean     # derruba tudo e apaga volumes (perde dados do MariaDB)

param(
    [switch]$Detached,
    [switch]$Rebuild,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

# Verifica que o Docker está disponível
try {
    docker version --format "{{.Server.Version}}" | Out-Null
} catch {
    Write-Host "ERRO: Docker nao encontrado ou daemon nao esta rodando." -ForegroundColor Red
    Write-Host "Instale o Docker Desktop: https://www.docker.com/products/docker-desktop/" -ForegroundColor Yellow
    exit 1
}

if ($Clean) {
    Write-Host "Limpando containers e volumes..." -ForegroundColor Yellow
    docker compose down -v
    Write-Host "Limpeza concluida." -ForegroundColor Green
    exit 0
}

$buildArgs = @()
if ($Rebuild) { $buildArgs += "--no-cache" }

Write-Host "Fazendo build das imagens..." -ForegroundColor Cyan
docker compose build @buildArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "Subindo servicos..." -ForegroundColor Cyan
if ($Detached) {
    docker compose up -d
    Write-Host ""
    Write-Host "Sistema rodando em background." -ForegroundColor Green
    Write-Host "  Aplicacao Flutter Web ..... http://localhost:8080"
    Write-Host "  Painel RabbitMQ ........... http://localhost:15672 (admin/admin)"
    Write-Host ""
    Write-Host "Comandos uteis:"
    Write-Host "  docker compose logs -f          # ver logs"
    Write-Host "  docker compose ps               # ver status"
    Write-Host "  docker compose down             # parar"
    Write-Host "  .\start.ps1 -Clean              # parar e apagar volumes"
} else {
    docker compose up
}
