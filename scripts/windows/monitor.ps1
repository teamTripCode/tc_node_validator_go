@echo off
REM monitor.bat - Script de monitoreo

echo 📊 Monitoreando red DPoS...

REM Función para verificar servicio local
:check_local_service
set port=%1
set name=%2

curl -s http://localhost:%port%/health >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ %name% (puerto %port%) - OK
) else (
    echo ❌ %name% (puerto %port%) - ERROR
)
goto :eof

REM Verificar si hay servicios locales
docker-compose ps | findstr "Up" >nul
if %errorlevel% equ 0 (
    echo 🔍 Verificando servicios locales...
    call :check_local_service 3001 "Nodo Principal"
    
    for /l %%i in (3002,1,3023) do (
        call :check_local_service %%i "Delegado"
    )
) else (
    echo ℹ️ No hay servicios locales corriendo
)

echo.

REM Verificar servicios de Kubernetes
kubectl get pods -n dpos-network >nul 2>&1
if %errorlevel% equ 0 (
    echo 🔍 Estado de pods en Kubernetes:
    kubectl get pods -n dpos-network
    echo.
    echo 📊 Uso de recursos:
    kubectl top pods -n dpos-network 2>nul || echo "Metrics server no disponible"
) else (
    echo ℹ️ No hay servicios en Kubernetes corriendo
)

pause