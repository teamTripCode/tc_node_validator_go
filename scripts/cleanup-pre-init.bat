@echo off
chcp 65001 >nul 2>&1
setlocal EnableDelayedExpansion

REM ============================================================================
REM TripCode Blockchain - Cleanup Script
REM Limpia completamente el entorno antes del deployment
REM ============================================================================

echo 🧹 TripCode Blockchain - Cleanup Completo
echo ============================================================================

REM Configuracion
set NAMESPACE=dpos-network
set IMAGE_NAME=foultrip/validator-node
set IMAGE_TAG=latest
set K8S_DIR=../k8s

echo 📋 Iniciando limpieza completa del entorno...

REM Verificar si kubectl esta disponible
kubectl version --client >nul 2>&1
if !errorlevel! neq 0 (
    echo ❌ kubectl no esta disponible
    echo    Saltando limpieza de Kubernetes...
    set kubectl_available=false
) else (
    set kubectl_available=true
)

REM Verificar si minikube esta disponible y corriendo
minikube version >nul 2>&1
if !errorlevel! neq 0 (
    echo ⚠️ Minikube no esta disponible
    echo    Saltando limpieza de Kubernetes...
    set minikube_available=false
    goto :cleanup_docker
)

minikube status >nul 2>&1
if !errorlevel! neq 0 (
    echo ⚠️ Minikube no esta corriendo
    echo    Saltando limpieza de Kubernetes...
    set minikube_available=false
    goto :cleanup_docker
) else (
    set minikube_available=true
)

:cleanup_kubernetes
if "!kubectl_available!"=="false" goto :cleanup_docker

echo 🔄 Fase 1: Limpieza de Kubernetes
echo    Eliminando StatefulSet y Pods...
kubectl scale statefulset validator-node --replicas=0 -n !NAMESPACE! >nul 2>&1
timeout /t 10 /nobreak >nul 2>&1

kubectl delete statefulset validator-node -n !NAMESPACE! --ignore-not-found=true --timeout=60s >nul 2>&1
kubectl delete pods -l app=validator-node -n !NAMESPACE! --force --grace-period=0 >nul 2>&1

echo    Eliminando servicios...
kubectl delete service validator-node -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete service validator-node-external -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete service validator-node-metrics -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete service seed-node-external -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete service seed-node-manual -n !NAMESPACE! --ignore-not-found=true >nul 2>&1

echo    Eliminando ConfigMaps y Secrets...
kubectl delete configmap validator-node-config -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete configmap mcp-config -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete secret validator-node-secrets -n !NAMESPACE! --ignore-not-found=true >nul 2>&1

echo    Eliminando PersistentVolumes y PersistentVolumeClaims...
kubectl delete pvc -l app=validator-node -n !NAMESPACE! --ignore-not-found=true >nul 2>&1
kubectl delete pv validator-node-pv --ignore-not-found=true >nul 2>&1

echo    Eliminando NetworkPolicies...
kubectl delete networkpolicy validator-node-network-policy -n !NAMESPACE! --ignore-not-found=true >nul 2>&1

echo    Eliminando Endpoints...
kubectl delete endpoints seed-node-manual -n !NAMESPACE! --ignore-not-found=true >nul 2>&1

echo    Esperando que todos los recursos se eliminen...
timeout /t 15 /nobreak >nul 2>&1

echo    Eliminando namespace completo...
kubectl delete namespace !NAMESPACE! --ignore-not-found=true --timeout=120s >nul 2>&1

echo ✅ Limpieza de Kubernetes completada

:cleanup_docker
echo 🐳 Fase 2: Limpieza de Docker

REM Verificar si Docker esta disponible
docker version >nul 2>&1
if !errorlevel! neq 0 (
    echo ⚠️ Docker no esta disponible
    goto :cleanup_files
)

echo    Eliminando contenedores relacionados...
for /f "tokens=*" %%i in ('docker ps -a -q --filter "ancestor=!IMAGE_NAME!:!IMAGE_TAG!" 2^>nul') do (
    echo    Eliminando contenedor: %%i
    docker rm -f %%i >nul 2>&1
)

echo    Eliminando imagenes locales...
docker rmi !IMAGE_NAME!:!IMAGE_TAG! --force >nul 2>&1
docker rmi !IMAGE_NAME!:latest --force >nul 2>&1

echo    Limpiando imagenes huerfanas...
docker image prune -f >nul 2>&1

echo    Limpiando volumenes no utilizados...
docker volume prune -f >nul 2>&1

echo    Limpiando redes no utilizadas...
docker network prune -f >nul 2>&1

echo ✅ Limpieza de Docker completada

:cleanup_files
echo 📁 Fase 3: Limpieza de archivos generados
if exist "!K8S_DIR!\secrets.yaml" (
    echo    Eliminando secrets.yaml generado...
    del "!K8S_DIR!\secrets.yaml" >nul 2>&1
)

REM Limpiar logs temporales si existen
if exist "*.log" (
    echo    Eliminando logs temporales...
    del "*.log" >nul 2>&1
)

REM Limpiar archivos temporales de deployment
if exist "deployment-*.tmp" (
    echo    Eliminando archivos temporales...
    del "deployment-*.tmp" >nul 2>&1
)

echo ✅ Limpieza de archivos completada

:cleanup_minikube_cache
echo 🔧 Fase 4: Limpieza de cache de Minikube

if "!minikube_available!"=="false" (
    echo ⚠️ Minikube no esta disponible
    goto :verify_cleanup
)

echo    Limpiando cache de imagenes de Minikube...
minikube cache delete !IMAGE_NAME!:!IMAGE_TAG! >nul 2>&1
if !errorlevel! neq 0 (
    echo    No hay cache para limpiar
)

echo    Limpiando volumenes de Minikube...
minikube ssh "sudo rm -rf /mnt/data/validator-node/*" >nul 2>&1
if !errorlevel! neq 0 (
    echo    No hay volumenes para limpiar
)

echo ✅ Limpieza de cache de Minikube completada

:verify_cleanup
echo 🔍 Fase 5: Verificacion de limpieza

if "!kubectl_available!"=="true" (
    echo    Verificando pods eliminados...
    set POD_COUNT=0
    for /f "tokens=*" %%i in ('kubectl get pods -n !NAMESPACE! --no-headers 2^>nul ^| find /c /v ""') do set POD_COUNT=%%i
    if !POD_COUNT! gtr 0 (
        echo ⚠️ Aun hay !POD_COUNT! pods en el namespace
    ) else (
        echo ✅ No hay pods restantes
    )

    echo    Verificando namespace...
    kubectl get namespace !NAMESPACE! >nul 2>&1
    if !errorlevel! equ 0 (
        echo ⚠️ Namespace aun existe
    ) else (
        echo ✅ Namespace eliminado completamente
    )
)

REM Verificar Docker solo si esta disponible
docker version >nul 2>&1
if !errorlevel! equ 0 (
    echo    Verificando imagenes Docker...
    docker images !IMAGE_NAME! --format "table {{.Repository}}:{{.Tag}}" 2>nul | findstr !IMAGE_TAG! >nul
    if !errorlevel! neq 0 (
        echo ✅ Imagenes Docker eliminadas
    ) else (
        echo ⚠️ Algunas imagenes Docker aun existen
    )
)

:show_final_status
echo ============================================================================
echo 📊 Estado final del sistema:
echo.
echo 🏠 Minikube:
minikube status 2>nul
if !errorlevel! neq 0 (
    echo    Minikube no esta corriendo
)
echo.

echo 🐳 Docker:
docker system df 2>nul
if !errorlevel! neq 0 (
    echo    Docker no esta disponible
)
echo.

echo 📁 Archivos K8s:
if exist "!K8S_DIR!\*.yaml" (
    echo    Manifiestos disponibles:
    dir /B "!K8S_DIR!\*.yaml" 2>nul
) else (
    echo    No hay manifiestos disponibles
)
echo.

echo ============================================================================
echo ✅ Limpieza completa terminada
echo.
echo 🚀 Ahora puedes ejecutar el script de deployment:
echo    ./init-validator-node-ia.bat
echo ============================================================================

echo Presiona cualquier tecla para continuar...
pause >nul
exit /b 0