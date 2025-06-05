#!/bin/bash
# scripts/monitor-k8s.sh - Monitoreo avanzado para Kubernetes

echo "📊 Monitoreando red DPoS en Kubernetes..."

# Función para mostrar estado de pods
show_pod_status() {
    echo "🔍 Estado de los pods:"
    kubectl get pods -n dpos-network -o wide
    echo ""
}

# Función para mostrar logs
show_logs() {
    echo "📝 Logs recientes del nodo principal:"
    kubectl logs -n dpos-network -l app=dpos-main --tail=10
    echo ""
    
    echo "📝 Logs recientes de delegados:"
    kubectl logs -n dpos-network -l app=dpos-delegates --tail=5
    echo ""
}

# Función para mostrar métricas de recursos
show_metrics() {
    echo "📈 Uso de recursos:"
    kubectl top pods -n dpos-network
    echo ""
}

# Función para verificar conectividad
check_connectivity() {
    echo "🔗 Verificando conectividad:"
    
    MAIN_SERVICE_URL=$(minikube service dpos-main-service --url -n dpos-network)
    if curl -s "$MAIN_SERVICE_URL/health" > /dev/null; then
        echo "✅ Nodo principal - OK"
    else
        echo "❌ Nodo principal - ERROR"
    fi
    
    # Test interno de conectividad entre pods
    MAIN_POD=$(kubectl get pods -n dpos-network -l app=dpos-main -o jsonpath='{.items[0].metadata.name}')
    if kubectl exec -n dpos-network $MAIN_POD -- wget -q -O- http://dpos-delegates-service:3001/health > /dev/null; then
        echo "✅ Conectividad interna - OK"
    else
        echo "❌ Conectividad interna - ERROR"
    fi
}

# Ejecutar todas las verificaciones
show_pod_status
show_metrics
check_connectivity
show_logs

# Opción para seguimiento continuo
if [[ "$1" == "--watch" ]]; then
    echo "👀 Modo seguimiento activado (Ctrl+C para salir)..."
    watch -n 5 kubectl get pods -n dpos-network
fi
