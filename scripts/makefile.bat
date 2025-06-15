@echo off
setlocal EnableDelayedExpansion

REM ============================================================================
REM TripCode Blockchain - Deployment Script
REM Genera TODOS los archivos YAML necesarios automáticamente
REM ============================================================================

echo 🚀 TripCode Blockchain - Deployment Completo
echo ============================================================================

REM Configuración
set IMAGE_NAME=foultrip/validator-node
set IMAGE_TAG=latest
set NAMESPACE=dpos-network
set TIMEOUT=600s
set HEALTH_CHECK_RETRIES=30
set K8S_DIR=../k8s

REM Crear directorio k8s si no existe
if not exist "%K8S_DIR%" mkdir "%K8S_DIR%"

REM Verificar prerequisites
echo 📋 Verificando prerequisites...
call :check_prerequisites
if errorlevel 1 exit /b 1

REM 0. Generar todos los manifiestos YAML
echo 📝 Fase 0: Generando manifiestos YAML
call :generate_manifests
if errorlevel 1 exit /b 1

REM 1. Build y Push con optimizaciones
echo 🏗️  Fase 1: Build y Registry
call :build_and_push_image
if errorlevel 1 exit /b 1

REM 2. Preparar entorno con validaciones
echo 🧹 Fase 2: Preparación del entorno
call :prepare_environment
if errorlevel 1 exit /b 1

REM 3. Gestión segura de secrets
echo 🔐 Fase 3: Gestión de secrets
call :manage_secrets
if errorlevel 1 exit /b 1

REM 4. Deploy con rolling update
echo 🚢 Fase 4: Deployment
call :deploy_application
if errorlevel 1 exit /b 1

REM 5. Validación post-deployment
echo ✅ Fase 5: Validación
call :validate_deployment
if errorlevel 1 exit /b 1

REM 6. Configurar observabilidad
echo 📊 Fase 6: Observabilidad
call :setup_monitoring

echo ============================================================================
echo ✅ Deployment completado exitosamente
call :show_cluster_info
echo ============================================================================
pause
exit /b 0

REM ============================================================================
REM FUNCIONES
REM ============================================================================

:check_prerequisites
echo    Verificando herramientas requeridas...
docker --version >nul 2>&1
if errorlevel 1 (
    echo ❌ Docker no está instalado o no está en PATH
    exit /b 1
)

kubectl version --client >nul 2>&1
if errorlevel 1 (
    echo ❌ kubectl no está instalado o no está en PATH
    exit /b 1
)

minikube status >nul 2>&1
if errorlevel 1 (
    echo ❌ Minikube no está ejecutándose
    echo    Ejecuta: minikube start
    exit /b 1
)

echo ✅ Prerequisites verificados
exit /b 0

:generate_manifests
echo    📝 Generando namespace.yaml...
(
echo apiVersion: v1
echo kind: Namespace
echo metadata:
echo   name: %NAMESPACE%
echo   labels:
echo     name: %NAMESPACE%
echo     app: tripcodechain
echo     environment: development
) > "%K8S_DIR%\namespace.yaml"

echo    📝 Generando configmap.yaml...
for /f %%i in ('minikube ip 2^>nul ^|^| echo "192.168.49.2"') do set MINIKUBE_IP=%%i
(
echo apiVersion: v1
echo kind: ConfigMap
echo metadata:
echo   name: validator-node-config
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo     version: v1.0.0
echo data:
echo   NODE_ENV: "production"
echo   CHAIN_ID: "tripcodechain-mainnet"
echo   DPoS_BLOCK_TIME: "3s"
echo   DPoS_MIN_STAKE: "100000000000000000000"
echo   DPoS_MAX_DELEGATES: "21"
echo   LOG_LEVEL: "info"
echo   P2P_BASE_PORT: "3001"
echo   API_BASE_PORT: "3002"
echo   SEED_NODES: "!MINIKUBE_IP!:3000"
echo   PEER_NODES: "validator-node-0.validator-node.%NAMESPACE%.svc.cluster.local:3001,validator-node-1.validator-node.%NAMESPACE%.svc.cluster.local:3001,validator-node-2.validator-node.%NAMESPACE%.svc.cluster.local:3001"
echo   HEALTH_CHECK_PORT: "3002"
echo   METRICS_PORT: "9090"
) > "%K8S_DIR%\configmap.yaml"

echo    📝 Generando persistent-volume.yaml...
(
echo apiVersion: v1
echo kind: PersistentVolume
echo metadata:
echo   name: validator-node-pv
echo   labels:
echo     app: validator-node
echo spec:
echo   capacity:
echo     storage: 50Gi
echo   accessModes:
echo     - ReadWriteOnce
echo   persistentVolumeReclaimPolicy: Retain
echo   storageClassName: standard
echo   hostPath:
echo     path: /mnt/data/validator-node
) > "%K8S_DIR%\persistent-volume.yaml"

echo    📝 Generando service.yaml...
(
echo # Servicio headless para StatefulSet
echo apiVersion: v1
echo kind: Service
echo metadata:
echo   name: validator-node
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo spec:
echo   clusterIP: None
echo   selector:
echo     app: validator-node
echo   ports:
echo   - name: p2p
echo     port: 3001
echo     targetPort: 3001
echo   - name: api
echo     port: 3002
echo     targetPort: 3002
echo ---
echo # Servicio externo para acceso desde fuera del cluster
echo apiVersion: v1
echo kind: Service
echo metadata:
echo   name: validator-node-external
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo     service-type: external
echo spec:
echo   type: NodePort
echo   selector:
echo     app: validator-node
echo   ports:
echo   - name: api-0
echo     port: 4001
echo     targetPort: 3002
echo     nodePort: 30001
echo   - name: api-1
echo     port: 4002
echo     targetPort: 3002
echo     nodePort: 30002
echo   - name: api-2
echo     port: 4003
echo     targetPort: 3002
echo     nodePort: 30003
echo ---
echo # Servicio para métricas
echo apiVersion: v1
echo kind: Service
echo metadata:
echo   name: validator-node-metrics
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo     metrics: enabled
echo spec:
echo   selector:
echo     app: validator-node
echo   ports:
echo   - name: metrics
echo     port: 9090
echo     targetPort: 9090
) > "%K8S_DIR%\service.yaml"

echo    📝 Generando statefulset.yaml...
(
echo apiVersion: apps/v1
echo kind: StatefulSet
echo metadata:
echo   name: validator-node
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo     version: v1.0.0
echo spec:
echo   serviceName: validator-node
echo   replicas: 3
echo   selector:
echo     matchLabels:
echo       app: validator-node
echo   template:
echo     metadata:
echo       labels:
echo         app: validator-node
echo         version: v1.0.0
echo       annotations:
echo         prometheus.io/scrape: "true"
echo         prometheus.io/port: "9090"
echo         prometheus.io/path: "/metrics"
echo     spec:
echo       securityContext:
echo         runAsNonRoot: true
echo         runAsUser: 1000
echo         fsGroup: 1000
echo       containers:
echo       - name: validator-node
echo         image: %IMAGE_NAME%:%IMAGE_TAG%
echo         imagePullPolicy: Always
echo         ports:
echo         - containerPort: 3001
echo           name: p2p
echo           protocol: TCP
echo         - containerPort: 3002
echo           name: api
echo           protocol: TCP
echo         - containerPort: 9090
echo           name: metrics
echo           protocol: TCP
echo         env:
echo         - name: POD_NAME
echo           valueFrom:
echo             fieldRef:
echo               fieldPath: metadata.name
echo         - name: POD_NAMESPACE
echo           valueFrom:
echo             fieldRef:
echo               fieldPath: metadata.namespace
echo         - name: POD_IP
echo           valueFrom:
echo             fieldRef:
echo               fieldPath: status.podIP
echo         - name: P2P_PORT
echo           value: "3001"
echo         - name: API_PORT
echo           value: "3002"
echo         - name: METRICS_PORT
echo           value: "9090"
echo         envFrom:
echo         - configMapRef:
echo             name: validator-node-config
echo         - secretRef:
echo             name: validator-node-secrets
echo         volumeMounts:
echo         - name: blockchain-data
echo           mountPath: /data
echo         - name: logs
echo           mountPath: /var/log/validator-node
echo         - name: tmp
echo           mountPath: /tmp
echo         readinessProbe:
echo           httpGet:
echo             path: /health
echo             port: 3002
echo             scheme: HTTP
echo           initialDelaySeconds: 30
echo           periodSeconds: 10
echo           timeoutSeconds: 5
echo           failureThreshold: 3
echo           successThreshold: 1
echo         livenessProbe:
echo           httpGet:
echo             path: /health
echo             port: 3002
echo             scheme: HTTP
echo           initialDelaySeconds: 60
echo           periodSeconds: 30
echo           timeoutSeconds: 5
echo           failureThreshold: 5
echo         startupProbe:
echo           httpGet:
echo             path: /health
echo             port: 3002
echo             scheme: HTTP
echo           initialDelaySeconds: 10
echo           periodSeconds: 5
echo           timeoutSeconds: 3
echo           failureThreshold: 30
echo         resources:
echo           requests:
echo             memory: "512Mi"
echo             cpu: "250m"
echo             ephemeral-storage: "1Gi"
echo           limits:
echo             memory: "1Gi"
echo             cpu: "500m"
echo             ephemeral-storage: "2Gi"
echo         securityContext:
echo           allowPrivilegeEscalation: false
echo           readOnlyRootFilesystem: false
echo           runAsNonRoot: true
echo           runAsUser: 1000
echo           runAsGroup: 1000
echo           capabilities:
echo             drop:
echo             - ALL
echo       volumes:
echo       - name: logs
echo         emptyDir:
echo           sizeLimit: 1Gi
echo       - name: tmp
echo         emptyDir:
echo           sizeLimit: 500Mi
echo       nodeSelector:
echo         kubernetes.io/os: linux
echo       tolerations:
echo       - key: node.kubernetes.io/not-ready
echo         operator: Exists
echo         effect: NoExecute
echo         tolerationSeconds: 300
echo       - key: node.kubernetes.io/unreachable
echo         operator: Exists
echo         effect: NoExecute
echo         tolerationSeconds: 300
echo   volumeClaimTemplates:
echo   - metadata:
echo       name: blockchain-data
echo       labels:
echo         app: validator-node
echo     spec:
echo       accessModes: [ "ReadWriteOnce" ]
echo       storageClassName: standard
echo       resources:
echo         requests:
echo           storage: 50Gi
echo   podManagementPolicy: OrderedReady
echo   updateStrategy:
echo     type: RollingUpdate
echo     rollingUpdate:
echo       maxUnavailable: 1
) > "%K8S_DIR%\statefulset.yaml"

echo    📝 Generando network-policy.yaml...
(
echo apiVersion: networking.k8s.io/v1
echo kind: NetworkPolicy
echo metadata:
echo   name: validator-node-network-policy
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: validator-node
echo spec:
echo   podSelector:
echo     matchLabels:
echo       app: validator-node
echo   policyTypes:
echo   - Ingress
echo   - Egress
echo   ingress:
echo   # Permitir comunicación P2P entre nodos validadores
echo   - from:
echo     - podSelector:
echo         matchLabels:
echo           app: validator-node
echo     ports:
echo     - protocol: TCP
echo       port: 3001
echo     - protocol: TCP
echo       port: 3002
echo   # Permitir acceso API externo
echo   - ports:
echo     - protocol: TCP
echo       port: 3002
echo   # Permitir acceso a métricas
echo   - ports:
echo     - protocol: TCP
echo       port: 9090
echo   egress:
echo   # Permitir comunicación entre nodos validadores
echo   - to:
echo     - podSelector:
echo         matchLabels:
echo           app: validator-node
echo     ports:
echo     - protocol: TCP
echo       port: 3001
echo     - protocol: TCP
echo       port: 3002
echo   # Permitir DNS
echo   - to: []
echo     ports:
echo     - protocol: UDP
echo       port: 53
echo     - protocol: TCP
echo       port: 53
echo   # Permitir conexión al seed node
echo   - to: []
echo     ports:
echo     - protocol: TCP
echo       port: 3000
echo   # Permitir HTTPS para registry
echo   - to: []
echo     ports:
echo     - protocol: TCP
echo       port: 443
) > "%K8S_DIR%\network-policy.yaml"

echo    📝 Generando seed-node-service.yaml...
(
echo # Servicio para conectar al nodo semilla en localhost
echo apiVersion: v1
echo kind: Service
echo metadata:
echo   name: seed-node-external
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: seed-node
echo     service-type: external
echo spec:
echo   type: ExternalName
echo   externalName: host.minikube.internal
echo   ports:
echo   - protocol: TCP
echo     port: 3000
echo     targetPort: 3000
echo ---
echo # Alternativa: Endpoint manual si ExternalName no funciona
echo apiVersion: v1
echo kind: Service
echo metadata:
echo   name: seed-node-manual
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: seed-node
echo     service-type: manual
echo spec:
echo   ports:
echo   - protocol: TCP
echo     port: 3000
echo     targetPort: 3000
echo ---
echo apiVersion: v1
echo kind: Endpoints
echo metadata:
echo   name: seed-node-manual
echo   namespace: %NAMESPACE%
echo   labels:
echo     app: seed-node
echo subsets:
echo - addresses:
echo   - ip: "!MINIKUBE_IP!"
echo   ports:
echo   - port: 3000
echo     protocol: TCP
) > "%K8S_DIR%\seed-node-service.yaml"

echo ✅ Todos los manifiestos YAML generados en %K8S_DIR%/
exit /b 0

:build_and_push_image
echo    🏗️  Construyendo imagen optimizada...

REM Cambiar al directorio padre donde están go.mod, go.sum y el código fuente
cd ..

REM Construir desde el directorio raíz del proyecto
docker build ^
    --cache-from %IMAGE_NAME%:%IMAGE_TAG% ^
    --build-arg BUILDKIT_INLINE_CACHE=1 ^
    -t %IMAGE_NAME%:%IMAGE_TAG% ^
    .

REM Volver al directorio scripts
cd scripts

if errorlevel 1 (
    echo ❌ Error al construir la imagen
    exit /b 1
)

echo    🔑 Verificando autenticación Docker Hub...
docker info | findstr "Username" >nul 2>&1
if errorlevel 1 (
    echo    Iniciando sesión en Docker Hub...
    docker login
    if errorlevel 1 (
        echo ❌ Error al autenticar con Docker Hub
        exit /b 1
    )
)

echo    📤 Pusheando imagen al registry...
docker push %IMAGE_NAME%:%IMAGE_TAG%
if errorlevel 1 (
    echo ❌ Error al pushear imagen
    exit /b 1
)

echo ✅ Imagen construida y pusheada exitosamente
exit /b 0

:prepare_environment
echo    🔍 Aplicando namespace...
kubectl apply -f "%K8S_DIR%\namespace.yaml"

echo    🧹 Limpieza controlada de recursos...
kubectl scale statefulset validator-node --replicas=0 -n %NAMESPACE% 2>nul
timeout /t 15 /nobreak >nul 2>&1

kubectl delete statefulset validator-node -n %NAMESPACE% --ignore-not-found=true --timeout=60s
kubectl delete pods -l app=validator-node -n %NAMESPACE% --force --grace-period=0 2>nul

echo ✅ Entorno preparado
exit /b 0

:manage_secrets
echo    🔐 Generando secrets criptográficamente seguros...
kubectl delete secret validator-node-secrets -n %NAMESPACE% --ignore-not-found=true

for /f %%i in ('powershell -Command "$bytes = [byte[]]::new(32); [Security.Cryptography.RNGCryptoServiceProvider]::new().GetBytes($bytes); [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([Convert]::ToBase64String($bytes)))"') do set VALIDATOR_KEY=%%i

for /f %%i in ('powershell -Command "$bytes = [byte[]]::new(32); [Security.Cryptography.RNGCryptoServiceProvider]::new().GetBytes($bytes); [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([Convert]::ToBase64String($bytes)))"') do set DELEGATE_KEY=%%i

for /f %%i in ('powershell -Command "[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('dummy-tls-cert-for-development'))"') do set TLS_CERT=%%i

for /f %%i in ('powershell -Command "[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('dummy-tls-key-for-development'))"') do set TLS_KEY=%%i

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
echo   VALIDATOR_PRIVATE_KEY: !VALIDATOR_KEY!
echo   DELEGATE_PRIVATE_KEY: !DELEGATE_KEY!
echo   TLS_CERT: !TLS_CERT!
echo   TLS_KEY: !TLS_KEY!
) > "%K8S_DIR%\secrets.yaml"

kubectl apply -f "%K8S_DIR%\secrets.yaml"
echo ✅ Secrets creados y aplicados
exit /b 0

:deploy_application
echo    🚢 Aplicando manifiestos...
kubectl apply -f "%K8S_DIR%\configmap.yaml"
kubectl apply -f "%K8S_DIR%\persistent-volume.yaml"
kubectl apply -f "%K8S_DIR%\network-policy.yaml"
kubectl apply -f "%K8S_DIR%\seed-node-service.yaml"
kubectl apply -f "%K8S_DIR%\service.yaml"
kubectl apply -f "%K8S_DIR%\statefulset.yaml"

echo ✅ Manifiestos aplicados
exit /b 0

:validate_deployment
echo    ⏳ Esperando que los pods estén listos...
kubectl wait --for=condition=ready pod -l app=validator-node -n %NAMESPACE% --timeout=%TIMEOUT%
if errorlevel 1 (
    echo ❌ Timeout esperando pods. Mostrando logs de diagnóstico...
    call :show_debug_info
    exit /b 1
)

echo ✅ Deployment validado exitosamente
exit /b 0

:setup_monitoring
echo    📊 Configurando observabilidad...
echo    📈 Port forwarding para métricas disponible...
echo    Ejecuta: kubectl port-forward svc/validator-node-metrics 9090:9090 -n %NAMESPACE%
echo ✅ Observabilidad configurada
exit /b 0

:show_debug_info
echo 🔍 Información de diagnóstico:
echo.
echo --- Pods ---
kubectl get pods -n %NAMESPACE% -o wide
echo.
echo --- Events ---
kubectl get events -n %NAMESPACE% --sort-by='.lastTimestamp'
echo.
echo --- Logs del primer pod ---
kubectl logs validator-node-0 -n %NAMESPACE% --tail=50 2>nul || echo No hay logs disponibles
exit /b 0

:show_cluster_info
echo 📋 Información del cluster:
echo.
echo 📁 Archivos generados en %K8S_DIR%/:
dir /B "%K8S_DIR%\*.yaml"
echo.
echo 🏠 Minikube IP: 
minikube ip
echo.
echo 📊 Estado de pods:
kubectl get pods -n %NAMESPACE% -o wide
echo.
echo 🌐 Servicios:
kubectl get services -n %NAMESPACE%
echo.
echo 🔗 URLs de acceso:
for /f %%i in ('minikube ip') do (
    echo    API Node 0: http://%%i:30001
    echo    API Node 1: http://%%i:30002  
    echo    API Node 2: http://%%i:30003
)
echo.
echo 🛠️  Comandos útiles:
echo    Ver logs:       kubectl logs -f validator-node-0 -n %NAMESPACE%
echo    Estado:         kubectl get pods -n %NAMESPACE%
echo    Entrar al pod:  kubectl exec -it validator-node-0 -n %NAMESPACE% -- /bin/bash
echo    Port forward:   kubectl port-forward validator-node-0 3002:3002 -n %NAMESPACE%
echo    Métricas:       kubectl port-forward svc/validator-node-metrics 9090:9090 -n %NAMESPACE%
echo    Cleanup:        kubectl delete namespace %NAMESPACE%
echo.
echo 🐳 Docker Hub: https://hub.docker.com/r/%IMAGE_NAME%
exit /b 0