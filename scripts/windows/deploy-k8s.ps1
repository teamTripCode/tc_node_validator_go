Write-Host "📊 Monitoreando red DPoS..." -ForegroundColor Green

# Función para verificar servicio
function Test-Service {
    param($Port, $Name)
    
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:$Port/health" -TimeoutSec 3 -ErrorAction Stop
        Write-Host "✅ $Name (puerto $Port) - OK" -ForegroundColor Green
    }
    catch {
        Write-Host "❌ $Name (puerto $Port) - ERROR" -ForegroundColor Red
    }
}

# Verificar servicios locales
$dockerServices = docker-compose ps 2>$null
if ($dockerServices -like "*Up*") {
    Write-Host "🔍 Verificando servicios locales..." -ForegroundColor Yellow
    
    Test-Service -Port 3001 -Name "Nodo Principal"
    
    for ($i = 3002; $i -le 3023; $i++) {
        Test-Service -Port $i -Name "Delegado $($i - 3001)"
    }
} else {
    Write-Host "ℹ️ No hay servicios locales corriendo" -ForegroundColor Yellow
}

Write-Host ""

# Verificar servicios de Kubernetes
try {
    $k8sPods = kubectl get pods -n dpos-network 2>$null
    if ($k8sPods) {
        Write-Host "🔍 Estado de pods en Kubernetes:" -ForegroundColor Yellow
        kubectl get pods -n dpos-network
        
        Write-Host ""
        Write-Host "📊 Uso de recursos:" -ForegroundColor Yellow
        kubectl top pods -n dpos-network 2>$null
    }
} catch {
    Write-Host "ℹ️ No hay servicios en Kubernetes corriendo" -ForegroundColor Yellow
}

Read-Host "Presiona Enter para continuar"