Write-Host "🚀 Configurando DPoS Validator Node en Windows..." -ForegroundColor Green

# Verificar requisitos
function Test-Prerequisites {
    Write-Host "🔍 Verificando requisitos..." -ForegroundColor Yellow
    
    # Verificar Docker
    try {
        $dockerVersion = docker --version
        Write-Host "✅ Docker encontrado: $dockerVersion" -ForegroundColor Green
    }
    catch {
        Write-Host "❌ Docker no está instalado o no está en PATH" -ForegroundColor Red
        Write-Host "Instala Docker Desktop desde: https://www.docker.com/products/docker-desktop" -ForegroundColor Yellow
        Read-Host "Presiona Enter para continuar"
        exit 1
    }
    
    # Verificar Docker Compose
    try {
        $composeVersion = docker-compose --version
        Write-Host "✅ Docker Compose encontrado: $composeVersion" -ForegroundColor Green
    }
    catch {
        Write-Host "❌ Docker Compose no está disponible" -ForegroundColor Red
        Read-Host "Presiona Enter para continuar"
        exit 1
    }
}

# Crear estructura de carpetas
function New-ProjectStructure {
    Write-Host "📁 Creando estructura de carpetas..." -ForegroundColor Yellow
    
    $folders = @(
        "k8s", "k8s\main", "k8s\delegates", "k8s\rbac", "k8s\network", "k8s\monitoring",
        "scripts", "config", "helm", "backups"
    )
    
    foreach ($folder in $folders) {
        if (!(Test-Path $folder)) {
            New-Item -ItemType Directory -Path $folder -Force | Out-Null
        }
    }
    
    Write-Host "✅ Estructura de carpetas creada" -ForegroundColor Green
}

# Construir imagen Docker
function Build-DockerImage {
    Write-Host "📦 Construyendo imagen Docker..." -ForegroundColor Yellow
    
    try {
        docker build -t dpos-node:latest .
        Write-Host "✅ Imagen Docker construida exitosamente" -ForegroundColor Green
    }
    catch {
        Write-Host "❌ Error construyendo imagen Docker" -ForegroundColor Red
        Read-Host "Presiona Enter para continuar"
        exit 1
    }
}

# Ejecutar configuración
Test-Prerequisites
New-ProjectStructure
Build-DockerImage

Write-Host ""
Write-Host "🎉 Configuración completa!" -ForegroundColor Green
Write-Host ""
Write-Host "Comandos disponibles:" -ForegroundColor Cyan
Write-Host "  .\deploy-local.ps1    - Desplegar con Docker Compose" -ForegroundColor White
Write-Host "  .\deploy-k8s.ps1      - Desplegar en Minikube" -ForegroundColor White
Write-Host "  .\monitor.ps1         - Monitorear servicios" -ForegroundColor White
Write-Host "  .\cleanup.ps1         - Limpiar todo" -ForegroundColor White
Write-Host ""
Read-Host "Presiona Enter para continuar"