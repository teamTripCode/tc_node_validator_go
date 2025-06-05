@echo off
REM status.bat - Mostrar estado general

echo 🔍 Estado de servicios DPoS

echo.
echo === SERVICIOS LOCALES ===
docker-compose ps 2>nul || echo No hay servicios locales

echo.
echo === SERVICIOS KUBERNETES ===
kubectl get all -n dpos-network 2>nul || echo No hay servicios en Kubernetes

echo.
echo === INFORMACIÓN DEL SISTEMA ===
echo Docker version:
docker --version

echo.
echo Docker Compose version:
docker-compose --version

echo.
echo Kubectl version:
kubectl version --client --short 2>nul || echo No disponible

echo.
echo Minikube version:
minikube version --short 2>nul || echo No disponible

echo.
echo Minikube status:
minikube status 2>nul || echo No corriendo

pause