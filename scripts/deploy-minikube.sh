#!/bin/bash
# scripts/deploy-minikube.sh - Despliegue completo en Minikube

set -e

echo "🚀 Desplegando red DPoS en Minikube..."

# Verificar que Minikube esté corriendo
if ! minikube status | grep -q "Running"; then
    echo "❌ Minikube no está corriendo. Iniciando..."
    minikube start --driver=docker --cpus=4 --memory=6144 --disk-size=20g
fi

# Habilitar addons necesarios
echo "🔧 Habilitando addons de Minikube..."
minikube addons enable ingress
minikube addons enable metrics-server
minikube addons enable dashboard

# Configurar entorno Docker de Minikube
echo "🔧 Configurando entorno Docker..."
eval $(minikube docker-env)

# Construir imagen en Minikube
echo "📦 Construyendo imagen en Minikube..."
docker build -t dpos-node:latest .

# Verificar que la imagen se construyó
docker images | grep dpos-node

# Aplicar manifiestos en orden
echo "🔧 Aplicando manifiestos de Kubernetes..."

# 1. Namespace y configuración básica
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml

# 2. RBAC
kubectl apply -f k8s/rbac/

# 3. Servicios principales
kubectl apply -f k8s/main/
kubectl apply -f k8s/delegates/

# 4. Red y seguridad
kubectl apply -f k8s/network/

# Esperar a que los pods estén listos
echo "⏳ Esperando a que los pods estén listos..."
kubectl wait --for=condition=ready pod -l app=dpos-main -n dpos-network --timeout=300s
echo "⏳ Esperando a que los delegados estén listos..."
kubectl wait --for=condition=ready pod -l app=dpos-delegates -n dpos-network --timeout=300s

# Obtener información de acceso
echo "🌐 Información de acceso:"
echo "  - Nodo principal: $(minikube service dpos-main-service --url -n dpos-network)"
echo "  - Dashboard: $(minikube dashboard --url)"

# Configurar /etc/hosts para ingress (opcional)
MINIKUBE_IP=$(minikube ip)
echo "🔧 Para usar ingress, agregue estas líneas a /etc/hosts:"
echo "  $MINIKUBE_IP dpos.local"
echo "  $MINIKUBE_IP delegates.dpos.local"

# Mostrar estado
echo "📊 Estado actual:"
kubectl get all -n dpos-network
