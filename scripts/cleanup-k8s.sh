echo "🧹 Limpiando despliegue de Kubernetes..."

# Eliminar namespace (esto elimina todo dentro)
kubectl delete namespace dpos-network --ignore-not-found=true

# Esperar a que se complete la eliminación
echo "⏳ Esperando eliminación completa..."
kubectl wait --for=delete namespace/dpos-network --timeout=60s

# Limpiar imágenes Docker en Minikube
eval $(minikube docker-env)
docker image prune -f
docker rmi dpos-node:latest 2>/dev/null || true

echo "✅ Limpieza completada"