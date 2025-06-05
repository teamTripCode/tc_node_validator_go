Write-Host "🧹 Limpiando despliegues..." -ForegroundColor Green

# Limpiar servicios locales
Write-Host "🔧 Deteniendo servicios locales..." -ForegroundColor Yellow
try {
    docker-compose down -v 2>$null
    docker system prune -f
    Write-Host "✅ Servicios locales limpiados" -ForegroundColor Green
} catch {
    Write-Host "ℹ️ No había servicios locales para limpiar" -ForegroundColor Yellow
}

# Limpiar Minikube si está corriendo
try {
    $minikubeStatus = minikube status 2>$null
    if ($minikubeStatus -like "*Running*") {
        Write-Host "🔧 Limpiando Minikube..." -ForegroundColor Yellow
        kubectl delete namespace dpos-network --ignore-not-found=true
        
        # Configurar entorno Docker de Minikube para limpiar imágenes
        & minikube docker-env | Invoke-Expression
        docker rmi dpos-node:latest 2>$null
        
        Write-Host "✅ Minikube limpiado" -ForegroundColor Green
    }
} catch {
    Write-Host "ℹ️ Minikube no estaba corriendo" -ForegroundColor Yellow
}

Write-Host "✅ Limpieza completada" -ForegroundColor Green
Read-Host "Presiona Enter para continuar"