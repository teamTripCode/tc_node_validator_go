#!/bin/bash
# scripts/port-forward.sh - Port forwarding para desarrollo local

echo "🔌 Configurando port forwarding..."

# Port forward para nodo principal
kubectl port-forward -n dpos-network service/dpos-main-service 3001:3001 &
PID_MAIN=$!

# Port forward para delegados (balanceador)
kubectl port-forward -n dpos-network service/dpos-delegates-service 3002:3001 &
PID_DELEGATES=$!

echo "✅ Port forwarding configurado:"
echo "  - Nodo principal: http://localhost:3001"
echo "  - Delegados (load balanced): http://localhost:3002"
echo ""
echo "Presiona Ctrl+C para detener..."

# Trap para limpiar procesos al salir
trap "kill $PID_MAIN $PID_DELEGATES 2>/dev/null" EXIT

# Mantener el script corriendo
wait