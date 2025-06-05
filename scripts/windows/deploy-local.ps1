Write-Host "🚀 Desplegando red DPoS localmente..." -ForegroundColor Green

# Verificar que Docker esté corriendo
try {
    docker info | Out-Null
}
catch {
    Write-Host "❌ Docker no está corriendo. Inicia Docker Desktop primero." -ForegroundColor Red
    Read-Host "Presiona Enter para continuar"
    exit 1
}

# Construir imagen si no existe
$imageExists = docker images | Select-String "dpos-node"
if (!$imageExists) {
    Write-Host "📦 Construyendo imagen Docker..." -ForegroundColor Yellow
    docker build -t dpos-node:latest .
}

# Levantar servicios
Write-Host "🔧 Iniciando servicios..." -ForegroundColor Yellow
try {
    docker-compose up -d
    
    # Esperar que se inicien
    Write-Host "⏳ Esperando que los servicios se inicien..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
    
    # Verificar estado
    Write-Host "✅ Verificando estado de los servicios..." -ForegroundColor Green
    docker-compose ps
    
    Write-Host ""
    Write-Host "🌐 Servicios disponibles:" -ForegroundColor Cyan
    Write-Host "  - Nodo principal: http://localhost:3001" -ForegroundColor White
    Write-Host "  - Delegados: http://localhost:3002 a http://localhost:3023" -ForegroundColor White
    Write-Host ""
    Write-Host "Usa .\monitor.ps1 para verificar el estado de los servicios" -ForegroundColor Yellow
}
catch {
    Write-Host "❌ Error iniciando servicios" -ForegroundColor Red
}

Read-Host "Presiona Enter para continuar"