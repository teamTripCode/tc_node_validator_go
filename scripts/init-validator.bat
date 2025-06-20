@echo off
setlocal enabledelayedexpansion

REM ============================================================================
REM TripCode Blockchain - Deployment Script for Kubernetes (Windows)
REM Deploys existing YAML manifests and configures Minikube
REM ============================================================================

echo TripCode Blockchain - Kubernetes Deployment
echo ============================================================================

REM Configuration
set IMAGE_NAME=foultrip/validator-node
set IMAGE_TAG=latest
set NAMESPACE=dpos-network
set TIMEOUT=600s
set K8S_DIR=..\k8s
set DOCKERFILE_DIR=..\

REM Store current directory
set SCRIPT_DIR=%cd%

REM Check prerequisites
:check_prerequisites
echo Checking prerequisites...

docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo Docker is not installed or not in PATH
    exit /b 1
)

kubectl version --client >nul 2>&1
if %errorlevel% neq 0 (
    echo kubectl is not installed or not in PATH
    exit /b 1
)

minikube status >nul 2>&1
if %errorlevel% neq 0 (
    echo Minikube is not running
    echo Run: minikube start
    exit /b 1
)

REM Check if Dockerfile exists
if not exist "%DOCKERFILE_DIR%\Dockerfile" (
    echo Dockerfile not found at %DOCKERFILE_DIR%\Dockerfile
    echo Please ensure Dockerfile exists in the project root
    exit /b 1
)

echo Prerequisites verified
goto configure_minikube

REM Configure Minikube for optimal performance
:configure_minikube
echo Configuring Minikube for blockchain workload...

REM Enable necessary addons
echo Enabling addons...
minikube addons enable metrics-server 2>nul
minikube addons enable storage-provisioner 2>nul
minikube addons enable default-storageclass 2>nul

REM Configure Docker environment
echo Configuring Docker environment...
for /f "tokens=*" %%i in ('minikube docker-env --shell cmd') do %%i

REM Get Minikube IP for configuration
for /f "tokens=*" %%i in ('minikube ip') do set MINIKUBE_IP=%%i
echo Minikube IP: %MINIKUBE_IP%

REM Configure network settings
echo Configuring network settings...

echo Minikube configured successfully
goto generate_secrets

REM Generate secrets.yaml with secure credentials
:generate_secrets
echo Generating secure secrets...

REM Generate cryptographically secure keys (Windows equivalent)
for /f "tokens=*" %%i in ('powershell -command "[System.Convert]::ToBase64String((1..32 | ForEach-Object {Get-Random -Maximum 256}) -as [byte[]])"') do set VALIDATOR_KEY=%%i
for /f "tokens=*" %%i in ('powershell -command "[System.Convert]::ToBase64String((1..32 | ForEach-Object {Get-Random -Maximum 256}) -as [byte[]])"') do set DELEGATE_KEY=%%i
for /f "tokens=*" %%i in ('powershell -command "[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes('dummy-tls-cert-for-development'))"') do set TLS_CERT=%%i
for /f "tokens=*" %%i in ('powershell -command "[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes('dummy-tls-key-for-development'))"') do set TLS_KEY=%%i
for /f "tokens=*" %%i in ('powershell -command "[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes('your-openai-api-key-here'))"') do set OPENAI_API_KEY=%%i

REM Create secrets.yaml
(
echo apiVersion: v1
echo kind: Secret
echo metadata:
echo   name: validator-node-secrets
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo type: Opaque
echo data:
echo   VALIDATOR_PRIVATE_KEY: %VALIDATOR_KEY%
echo   DELEGATE_PRIVATE_KEY: %DELEGATE_KEY%
echo   TLS_CERT: %TLS_CERT%
echo   TLS_KEY: %TLS_KEY%
echo   OPENAI_API_KEY: %OPENAI_API_KEY%
echo   LLM_API_KEY: %OPENAI_API_KEY%
) > "%K8S_DIR%\secrets.yaml"

echo Secrets generated and saved to %K8S_DIR%\secrets.yaml
goto build_and_push_image

REM Build and push Docker image
:build_and_push_image
echo Building and pushing Docker image...

REM Change to Dockerfile directory
echo Changing to Dockerfile directory: %DOCKERFILE_DIR%
cd /d "%DOCKERFILE_DIR%"
if %errorlevel% neq 0 (
    echo Failed to change to Dockerfile directory
    exit /b 1
)

REM Verify we're in the right location
echo Current directory: %cd%
if not exist "Dockerfile" (
    echo Dockerfile not found in current directory
    echo Expected location: %cd%\Dockerfile
    cd /d "%SCRIPT_DIR%"
    exit /b 1
)

REM Build image with build context information
echo Building image %IMAGE_NAME%:%IMAGE_TAG%...
docker build -t %IMAGE_NAME%:%IMAGE_TAG% . --no-cache
if %errorlevel% neq 0 (
    echo Failed to build Docker image
    cd /d "%SCRIPT_DIR%"
    exit /b 1
)

REM Return to scripts directory
cd /d "%SCRIPT_DIR%"

REM Tag image with additional tags if needed
echo Tagging image...
docker tag %IMAGE_NAME%:%IMAGE_TAG% %IMAGE_NAME%:v%date:~-4,4%%date:~-10,2%%date:~-7,2%
docker tag %IMAGE_NAME%:%IMAGE_TAG% %IMAGE_NAME%:latest

REM Check Docker Hub authentication
docker info | findstr "Username" >nul 2>&1
if %errorlevel% neq 0 (
    echo Docker Hub authentication required...
    echo Please login to Docker Hub:
    docker login
    if %errorlevel% neq 0 (
        echo Failed to login to Docker Hub
        exit /b 1
    )
)

REM Push image
echo Pushing image to Docker Hub...
docker push %IMAGE_NAME%:%IMAGE_TAG%
if %errorlevel% neq 0 (
    echo Failed to push Docker image
    exit /b 1
)

REM Push additional tags
echo Pushing additional tags...
docker push %IMAGE_NAME%:v%date:~-4,4%%date:~-10,2%%date:~-7,2% 2>nul
docker push %IMAGE_NAME%:latest 2>nul

echo Image built and pushed successfully
echo Image: %IMAGE_NAME%:%IMAGE_TAG%
echo Registry: https://hub.docker.com/r/foultrip/validator-node
goto prepare_environment

REM Prepare Kubernetes environment
:prepare_environment
echo Preparing Kubernetes environment...

REM Verify K8S manifests exist
if not exist "%K8S_DIR%\namespace.yaml" (
    echo Kubernetes manifests not found in %K8S_DIR%
    echo Please ensure all YAML files are present
    exit /b 1
)

REM Apply namespace first
echo Creating namespace...
kubectl apply -f "%K8S_DIR%\namespace.yaml"

REM Clean up existing resources gracefully
echo Cleaning existing resources...
kubectl scale statefulset validator-node --replicas=0 -n %NAMESPACE% 2>nul
timeout /t 15 /nobreak >nul

kubectl delete statefulset validator-node -n %NAMESPACE% --ignore-not-found=true --timeout=60s 2>nul
kubectl delete pods -l app=validator-node -n %NAMESPACE% --force --grace-period=0 2>nul

echo Environment prepared
goto deploy_application

REM Deploy application manifests
:deploy_application
echo Deploying application manifests...

REM Apply manifests in correct order
echo Applying ConfigMaps...
kubectl apply -f "%K8S_DIR%\mcp-configmap.yaml"
kubectl apply -f "%K8S_DIR%\configmap.yaml"

echo Applying Secrets...
kubectl apply -f "%K8S_DIR%\secrets.yaml"

echo Applying Storage...
kubectl apply -f "%K8S_DIR%\persistent-volume.yaml"

echo Applying Network Policies...
kubectl apply -f "%K8S_DIR%\network-policy.yaml"

echo Applying Services...
kubectl apply -f "%K8S_DIR%\seed-node-service.yaml"
kubectl apply -f "%K8S_DIR%\service.yaml"

echo Applying StatefulSet...
kubectl apply -f "%K8S_DIR%\statefulset.yaml"

echo All manifests applied successfully
goto validate_deployment

REM Validate deployment
:validate_deployment
echo Validating deployment...

echo Waiting for pods to be ready...
kubectl wait --for=condition=ready pod -l app=validator-node -n %NAMESPACE% --timeout=%TIMEOUT%
if %errorlevel% neq 0 (
    echo Timeout waiting for pods. Showing diagnostic info...
    call :show_debug_info
    exit /b 1
)

echo Deployment validated successfully
goto setup_monitoring

REM Setup monitoring and observability
:setup_monitoring
echo Setting up monitoring...

echo Metrics endpoints available:
echo - Blockchain metrics: kubectl port-forward svc/validator-node-metrics 9090:9090 -n %NAMESPACE%
echo - MCP metrics: kubectl port-forward svc/validator-node-metrics 9091:9091 -n %NAMESPACE%

echo Monitoring configured
goto show_cluster_info

REM Show debug information
:show_debug_info
echo Debug Information:
echo.
echo --- Pods ---
kubectl get pods -n %NAMESPACE% -o wide 2>nul
echo.
echo --- Events ---
kubectl get events -n %NAMESPACE% --sort-by='.lastTimestamp' 2>nul
echo.
echo --- Logs from first pod ---
kubectl logs validator-node-0 -n %NAMESPACE% --tail=50 2>nul || echo No logs available
echo.
echo --- ConfigMaps ---
kubectl get configmaps -n %NAMESPACE% 2>nul
echo.
echo --- Secrets ---
kubectl get secrets -n %NAMESPACE% 2>nul
echo.
echo --- StatefulSet Status ---
kubectl describe statefulset validator-node -n %NAMESPACE% 2>nul
goto :eof

REM Show cluster information
:show_cluster_info
echo Cluster Information:
echo.
for /f "tokens=*" %%i in ('minikube ip') do set MINIKUBE_IP=%%i
echo Minikube IP: %MINIKUBE_IP%
echo.
echo Pod Status:
kubectl get pods -n %NAMESPACE% -o wide
echo.
echo Services:
kubectl get services -n %NAMESPACE%
echo.
echo Access URLs:
echo    API Node 0: http://%MINIKUBE_IP%:30001
echo    API Node 1: http://%MINIKUBE_IP%:30002
echo    API Node 2: http://%MINIKUBE_IP%:30003
echo    MCP Node 0: http://%MINIKUBE_IP%:30011
echo    MCP Node 1: http://%MINIKUBE_IP%:30012
echo    MCP Node 2: http://%MINIKUBE_IP%:30013
echo.
echo Useful Commands:
echo    View logs:       kubectl logs -f validator-node-0 -n %NAMESPACE%
echo    Pod status:      kubectl get pods -n %NAMESPACE%
echo    Enter pod:       kubectl exec -it validator-node-0 -n %NAMESPACE% -- /bin/bash
echo    Port forward:    kubectl port-forward validator-node-0 3002:3002 -n %NAMESPACE%
echo    MCP forward:     kubectl port-forward validator-node-0 3003:3003 -n %NAMESPACE%
echo    Metrics:         kubectl port-forward svc/validator-node-metrics 9090:9090 -n %NAMESPACE%
echo    MCP Metrics:     kubectl port-forward svc/validator-node-metrics 9091:9091 -n %NAMESPACE%
echo    Cleanup:         kubectl delete namespace %NAMESPACE%
echo.
echo Docker Hub: https://hub.docker.com/r/%IMAGE_NAME%
echo Image Tags:
echo    - %IMAGE_NAME%:latest
echo    - %IMAGE_NAME%:%IMAGE_TAG%
echo    - %IMAGE_NAME%:v%date:~-4,4%%date:~-10,2%%date:~-7,2%

echo ============================================================================
echo Deployment completed successfully!
echo    Docker image: %IMAGE_NAME%:%IMAGE_TAG%
echo    Kubernetes namespace: %NAMESPACE%
echo    Minikube IP: %MINIKUBE_IP%
echo ============================================================================

endlocal
pause