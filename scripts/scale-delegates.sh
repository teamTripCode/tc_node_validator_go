#!/bin/bash
# scripts/scale-delegates.sh - Escalado dinámico de delegados

REPLICAS=${1:-22}
MAX_REPLICAS=101
MIN_REPLICAS=21

if [[ $REPLICAS -lt $MIN_REPLICAS ]]; then
    echo "❌ Mínimo $MIN_REPLICAS delegados requeridos para DPoS"
    exit 1
fi

if [[ $REPLICAS -gt $MAX_REPLICAS ]]; then
    echo "❌ Máximo $MAX_REPLICAS delegados permitidos"
    exit 1
fi

echo "⚖️ Escalando delegados a $REPLICAS réplicas..."

# Escalar deployment
kubectl scale deployment dpos-delegates --replicas=$REPLICAS -n dpos-network

# Esperar a que el escalado se complete
echo "⏳ Esperando completar escalado..."
kubectl rollout status deployment/dpos-delegates -n dpos-network

# Verificar estado final
echo "✅ Escalado completado:"
kubectl get deployment dpos-delegates -n dpos-network