@echo off
setlocal EnableDelayedExpansion

REM ============================================================================
REM TripCode Blockchain - Cleanup Script Completo 2025
REM Limpia TODOS los recursos para una instalación completamente limpia
REM ============================================================================

echo 🧹 TripCode Blockchain - Cleanup Completo
echo ============================================================================

REM Configuración
set IMAGE_NAME=foultrip/validator-node
set IMAGE_TAG=latest
set NAMESPACE=dpos-network
set K8S_DIR=../k8s

echo 🚨 ADVERTENCIA: Este script eliminará TODOS los recursos del blockchain
echo    - Namespace: %NAMESPACE%
echo    - Pods, Services, StatefulSets, PVCs
echo    - Imágenes Docker locales
echo    - Archivos YAML generados
echo    - Datos persistentes
echo.
echo ¿Deseas continuar? (S/N)
set /p CONFIRM="> "

if /i not "%CONFIRM%"=="S" (
    echo ❌ Operación cancelada
    pause
    exit /b 0
)

echo ============================================================================
echo 🚀 Iniciando limpieza completa...
echo ============================================================================

REM 1. Verificar prerequisites
echo 📋 Fase 1: Verificando herramientas
call :check_prerequisites
if errorlevel 1 exit /b 1

REM 2. Limpiar recursos de Kubernetes
echo 🧹 Fase 2: Limpiando recursos de Kubernetes
call :cleanup_kubernetes
if errorlevel 1 exit /b 1

REM 3. Limpiar imágenes Docker
echo 🐳 Fase 3: Limpiando imágenes Docker
call :cleanup_docker

REM 4. Limpiar archivos locales
echo 📁 Fase 4: Limpiando archivos locales
call :cleanup_files

REM 5. Limpiar datos persistentes
echo 💾 Fase 5: Limpiando datos persistentes
call :cleanup_persistent_data

REM 6. Verificar limpieza
echo ✅ Fase 6: Verificando limpieza
call :verify_cleanup

echo ============================================================================
echo ✅ Limpieza completada exitosamente
echo 🚀 Ahora puedes ejecutar el script de deployment para una instalación limpia
echo ============================================================================
pause
exit /b 0

REM ============================================================================
REM FUNCIONES
REM ============================================================================

:check_prerequisites
echo    📋 Verificando herramientas requeridas...

REM Verificar kubectl
kubectl version --client >nul 2>&1
if errorlevel 1 (
    echo ❌ kubectl no está instalado o no está en PATH
    echo    El cleanup de Kubernetes será omitido
    set KUBECTL_AVAILABLE=false
) else (
    echo ✅ kubectl disponible
    set KUBECTL_AVAILABLE=true
)

REM Verificar Docker
docker --version >nul 2>&1
if errorlevel 1 (
    echo ❌ Docker no está instalado o no está en PATH
    echo    El cleanup de Docker será omitido
    set DOCKER_AVAILABLE=false
) else (
    echo ✅ Docker disponible
    set DOCKER_AVAILABLE=true
)

REM Verificar Minikube
minikube status >nul 2>&1
if errorlevel 1 (
    echo ⚠️  Minikube no está ejecutándose
    echo    Algunos recursos de Kubernetes podrían no eliminarse
    set MINIKUBE_AVAILABLE=false
) else (
    echo ✅ Minikube disponible
    set MINIKUBE_AVAILABLE=true
)

echo ✅ Verificación de prerequisites completada
exit /b 0

:cleanup_kubernetes
if "%KUBECTL_AVAILABLE%"=="false" (
    echo ⚠️  kubectl no disponible, omitiendo limpieza de Kubernetes
    exit /b 0
)

echo    🧹 Eliminando namespace y todos sus recursos...

REM Escalar StatefulSet a 0 réplicas primero
echo    📉 Escalando StatefulSet a 0 réplicas...
kubectl scale statefulset validator-node --replicas=0 -n %NAMESPACE% --timeout=60s 2>nul

REM Esperar a que los pods se terminen
echo    ⏳ Esperando terminación de pods...
timeout /t 30 /nobreak >nul 2>&1

REM Eliminar StatefulSet forzadamente
echo    🗑️  Eliminando StatefulSet...
kubectl delete statefulset validator-node -n %NAMESPACE% --force --grace-period=0 --timeout=60s 2>nul

REM Eliminar todos los pods forzadamente
echo    🗑️  Eliminando pods forzadamente...
kubectl delete pods -l app=validator-node -n %NAMESPACE% --force --grace-period=0 2>nul

REM Eliminar PVCs
echo    💾 Eliminando PersistentVolumeClaims...
kubectl delete pvc -l app=validator-node -n %NAMESPACE% --timeout=60s 2>nul

REM Eliminar servicios
echo    🌐 Eliminando servicios...
kubectl delete service validator-node validator-node-external validator-node-metrics seed-node-external seed-node-manual -n %NAMESPACE% --timeout=30s 2>nul

REM Eliminar ConfigMaps y Secrets
echo    🔐 Eliminando ConfigMaps y Secrets...
kubectl delete configmap validator-node-config -n %NAMESPACE% --timeout=30s 2>nul
kubectl delete secret validator-node-secrets -n %NAMESPACE% --timeout=30s 2>nul

REM Eliminar NetworkPolicies
echo    🛡️  Eliminando NetworkPolicies...
kubectl delete networkpolicy validator-node-network-policy -n %NAMESPACE% --timeout=30s 2>nul

REM Eliminar HPA
echo    📊 Eliminando HorizontalPodAutoscaler...
kubectl delete hpa validator-node-hpa -n %NAMESPACE% --timeout=30s 2>nul

REM Eliminar PersistentVolumes
echo    💿 Eliminando PersistentVolumes...
kubectl delete pv validator-node-pv --timeout=60s 2>nul

REM Eliminar Endpoints manuales
echo    🔗 Eliminando Endpoints...
kubectl delete endpoints seed-node-manual -n %NAMESPACE% --timeout=30s 2>nul

REM Eliminar el namespace completo
echo    🗂️  Eliminando namespace %NAMESPACE%...
kubectl delete namespace %NAMESPACE% --timeout=120s 2>nul

REM Verificar que no queden recursos
echo    🔍 Verificando eliminación...
timeout /t 10 /nobreak >nul 2>&1

kubectl get namespace %NAMESPACE% >nul 2>&1
if not errorlevel 1 (
    echo ⚠️  El namespace %NAMESPACE% aún existe, forzando eliminación...
    kubectl patch namespace %NAMESPACE% -p "{\"metadata\":{\"finalizers\":[]}}" --type=merge 2>nul
    kubectl delete namespace %NAMESPACE% --force --grace-period=0 2>nul
)

echo ✅ Limpieza de Kubernetes completada
exit /b 0

:cleanup_docker
if "%DOCKER_AVAILABLE%"=="false" (
    echo ⚠️  Docker no disponible, omitiendo limpieza de Docker
    exit /b 0
)

echo    🐳 Eliminando imágenes Docker locales...

REM Detener y eliminar contenedores relacionados
echo    🛑 Deteniendo contenedores relacionados...
for /f "tokens=1" %%i in ('docker ps -a --filter "ancestor=%IMAGE_NAME%" --format "{{.ID}}" 2^>nul') do (
    echo    Deteniendo contenedor %%i...
    docker stop %%i >nul 2>&1
    docker rm %%i >nul 2>&1
)

REM Eliminar imágenes del proyecto
echo    🗑️  Eliminando imágenes %IMAGE_NAME%...
docker rmi %IMAGE_NAME%:%IMAGE_TAG% --force >nul 2>&1
docker rmi %IMAGE_NAME%:latest --force >nul 2>&1

REM Limpiar imágenes huérfanas y cache
echo    🧹 Limpiando imágenes huérfanas y cache...
docker image prune -f >nul 2>&1
docker builder prune -f >nul 2>&1

REM Limpiar volúmenes huérfanos
echo    💾 Limpiando volúmenes huérfanos...
docker volume prune -f >nul 2>&1

REM Limpiar redes huérfanas
echo    🌐 Limpiando redes huérfanas...
docker network prune -f >nul 2>&1

echo ✅ Limpieza de Docker completada
exit /b 0

:cleanup_files
echo    📁 Eliminando archivos YAML generados...

if exist "%K8S_DIR%" (
    echo    🗑️  Eliminando directorio %K8S_DIR%...
    rmdir /S /Q "%K8S_DIR%" 2>nul
    if exist "%K8S_DIR%" (
        echo ⚠️  No se pudo eliminar %K8S_DIR%, eliminando archivos individualmente...
        del /Q "%K8S_DIR%\*.yaml" 2>nul
        del /Q "%K8S_DIR%\*.yml" 2>nul
    )
) else (
    echo    ℹ️  Directorio %K8S_DIR% no existe
)

REM Eliminar archivos temporales
echo    🧹 Eliminando archivos temporales...
del /Q "*.tmp" 2>nul
del /Q "temp_*" 2>nul
del /Q "*.log" 2>nul

echo ✅ Limpieza de archivos completada
exit /b 0

:cleanup_persistent_data
if "%MINIKUBE_AVAILABLE%"=="false" (
    echo ⚠️  Minikube no disponible, omitiendo limpieza de datos persistentes
    exit /b 0
)

echo    💾 Eliminando datos persistentes de Minikube...

REM Eliminar datos del hostPath
echo    🗑️  Eliminando datos del hostPath...
minikube ssh "sudo rm -rf /mnt/data/validator-node" 2>nul

REM Limpiar logs del sistema
echo    📋 Limpiando logs del sistema...
minikube ssh "sudo rm -rf /var/log/validator-node*" 2>nul

REM Limpiar cache de imágenes en Minikube
echo    🐳 Limpiando cache de imágenes en Minikube...
minikube ssh "docker system prune -af" 2>nul

echo ✅ Limpieza de datos persistentes completada
exit /b 0

:verify_cleanup
echo    🔍 Verificando que la limpieza fue exitosa...

REM Verificar que no hay pods del proyecto
if "%KUBECTL_AVAILABLE%"=="true" (
    echo    📋 Verificando pods...
    kubectl get pods -A --field-selector=metadata.namespace=%NAMESPACE% >nul 2>&1
    if not errorlevel 1 (
        echo ⚠️  Aún existen pods del namespace %NAMESPACE%
    ) else (
        echo ✅ No hay pods del proyecto
    )

    REM Verificar que no hay PVCs
    echo    💾 Verificando PVCs...
    kubectl get pvc -A --field-selector=metadata.namespace=%NAMESPACE% >nul 2>&1
    if not errorlevel 1 (
        echo ⚠️  Aún existen PVCs del namespace %NAMESPACE%
    ) else (
        echo ✅ No hay PVCs del proyecto
    )

    REM Verificar que no hay namespace
    echo    🗂️  Verificando namespace...
    kubectl get namespace %NAMESPACE% >nul 2>&1
    if not errorlevel 1 (
        echo ⚠️  El namespace %NAMESPACE% aún existe
    ) else (
        echo ✅ Namespace eliminado correctamente
    )
)

REM Verificar imágenes Docker
if "%DOCKER_AVAILABLE%"=="true" (
    echo    🐳 Verificando imágenes Docker...
    docker images %IMAGE_NAME% --format "{{.Repository}}:{{.Tag}}" | findstr %IMAGE_NAME% >nul 2>&1
    if not errorlevel 1 (
        echo ⚠️  Aún existen imágenes %IMAGE_NAME%
    ) else (
        echo ✅ Imágenes Docker eliminadas
    )
)

REM Verificar archivos
echo    📁 Verificando archivos...
if exist "%K8S_DIR%" (
    echo ⚠️  El directorio %K8S_DIR% aún existe
) else (
    echo ✅ Archivos YAML eliminados
)

echo ✅ Verificación completada
exit /b 0