# Simulation Script for Distributed Blockchain
# This script starts 3 nodes on localhost

$PSScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Definition
if (!$PSScriptRoot) { $PSScriptRoot = Get-Location }

if (!(Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERRO: O comando 'go' não foi encontrado no PATH." -ForegroundColor Red
    Write-Host "Certifique-se de que o Go está instalado e configurado corretamente." -ForegroundColor Yellow
    Pause
    exit
}

$Node1 = "go run cmd/node/main.go -id 1 -port 8001 -peers 2:8002,3:8003"
$Node2 = "go run cmd/node/main.go -id 2 -port 8002 -peers 1:8001,3:8003"
$Node3 = "go run cmd/node/main.go -id 3 -port 8003 -peers 1:8001,2:8002"

Write-Host "Iniciando Nó 1 na porta 8001..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Write-Host 'Iniciando...'; $Node1" -WorkingDirectory $PSScriptRoot

Write-Host "Iniciando Nó 2 na porta 8002..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Write-Host 'Iniciando...'; $Node2" -WorkingDirectory $PSScriptRoot

Write-Host "Iniciando Nó 3 na porta 8003..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Write-Host 'Iniciando...'; $Node3" -WorkingDirectory $PSScriptRoot

Write-Host "Nodes started. After 5 seconds they will elect a leader (should be Node 3)." -ForegroundColor Yellow
Write-Host "To test Bully Algorithm: kill Node 3 process and watch Node 2 become leader." -ForegroundColor Yellow
