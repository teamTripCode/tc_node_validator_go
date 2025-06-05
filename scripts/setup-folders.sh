#!/bin/bash
# scripts/setup-folders.sh - Crear estructura de carpetas

echo "📁 Creando estructura de carpetas para Kubernetes..."

# Crear directorios principales
mkdir -p k8s/{main,delegates,rbac,network,monitoring}
mkdir -p scripts
mkdir -p config
mkdir -p helm/dpos-node/{templates,charts}

echo "✅ Estructura de carpetas creada"