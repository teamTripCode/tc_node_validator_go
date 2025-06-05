@echo off
REM scale.bat - Script de escalado

if "%1"=="" (
    set /p replicas="Ingresa el número de réplicas (22): "
    if "!replicas!"=="" set replicas=22
) else (
    set replicas=%1
)

if %replicas% lss 21 (
    echo ❌ Mínimo 21 delegados requeridos para DPoS
    pause
    exit /b 1
)

if %replicas% gtr 101 (
    echo ❌ Máximo 101 delegados permitidos
    pause
    exit /b 1
)

echo ⚖️ Escalando delegados a %replicas% réplicas...

REM Para Kubernetes
kubectl scale deployment dpos-delegates --replicas=%replicas% -n dpos-network

if %errorlevel% equ 0 (
    echo ✅ Escalado completado
    kubectl get deployment dpos-delegates -n dpos-network
) else (
    echo ❌ Error durante el escalado
)

pause