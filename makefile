# Makefile para DPoS Validator Node

.PHONY: help build setup-folders deploy-local deploy-k8s monitor clean

# Variables
DOCKER_IMAGE = dpos-node:latest
NAMESPACE = dpos-network
REPLICAS = 22

help: ## Mostrar ayuda
	@echo "DPoS Validator Node - Comandos disponibles:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup-folders: ## Crear estructura de carpetas
	@chmod +x scripts/*.sh
	@./scripts/setup-folders.sh

build: ## Construir imagen Docker
	@echo "📦 Construyendo imagen Docker..."
	@docker build -t $(DOCKER_IMAGE) .

# Despliegues locales
deploy-local: build ## Desplegar con Docker Compose
	@echo "🚀 Desplegando localmente..."
	@docker-compose up -d

stop-local: ## Detener despliegue local
	@echo "🛑 Deteniendo servicios locales..."
	@docker-compose down

# Despliegues Kubernetes
deploy-k8s: build ## Desplegar en Minikube
	@echo "🚀 Desplegando en Kubernetes..."
	@./scripts/deploy-minikube.sh

clean-k8s: ## Limpiar despliegue de Kubernetes
	@./scripts/cleanup-k8s.sh

# Monitoreo
monitor-local: ## Monitorear servicios locales
	@./scripts/monitor.sh

monitor-k8s: ## Monitorear servicios en Kubernetes
	@./scripts/monitor-k8s.sh

logs-k8s: ## Ver logs de Kubernetes
	@echo "📝 Logs del nodo principal:"
	@kubectl logs -n $(NAMESPACE) -l app=dpos-main --tail=20
	@echo ""
	@echo "📝 Logs de delegados:"
	@kubectl logs -n $(NAMESPACE) -l app=dpos-delegates --tail=10

# Escalado
scale: ## Escalar delegados (make scale REPLICAS=30)
	@./scripts/scale-delegates.sh $(REPLICAS)

# Utilidades
port-forward: ## Configurar port forwarding
	@./scripts/port-forward.sh

status: ## Mostrar estado de servicios
	@echo "🔍 Estado Docker Compose:"
	@docker-compose ps 2>/dev/null || echo "No hay servicios locales corriendo"
	@echo ""
	@echo "🔍 Estado Kubernetes:"
	@kubectl get all -n $(NAMESPACE) 2>/dev/null || echo "No hay servicios en Kubernetes"

# Limpieza completa
clean: stop-local clean-k8s ## Limpiar todo
	@echo "🧹 Limpieza completa..."
	@docker system prune -f
	@docker volume prune -f

# Desarrollo
dev: deploy-local ## Modo desarrollo (local)
	@echo "🔧 Iniciando modo desarrollo..."
	@./scripts/monitor.sh --watch

# Testing
test-connectivity: ## Probar conectividad de la red
	@echo "🔗 Probando conectividad local..."
	@for port in $$(seq 3001 3023); do \
		if curl -s http://localhost:$$port/health >/dev/null 2>&1; then \
			echo "✅ Puerto $$port - OK"; \
		else \
			echo "❌ Puerto $$port - ERROR"; \
		fi; \
	done

# Información del sistema
info: ## Mostrar información del sistema
	@echo "ℹ️ Información del sistema:"
	@echo "Docker version: $$(docker --version)"
	@echo "Docker Compose version: $$(docker-compose --version)"
	@echo "Kubectl version: $$(kubectl version --client --short 2>/dev/null || echo 'No disponible')"
	@echo "Minikube version: $$(minikube version --short 2>/dev/null || echo 'No disponible')"
	@echo "Minikube status: $$(minikube status 2>/dev/null || echo 'No corriendo')"

# Backup y restauración
backup: ## Hacer backup de configuraciones
	@echo "💾 Creando backup..."
	@mkdir -p backups
	@tar -czf backups/dpos-config-$$(date +%Y%m%d-%H%M%S).tar.gz k8s/ config/ docker-compose.yml

init: setup-folders build ## Inicializar proyecto completo
	@echo "🎉 Proyecto DPoS inicializado correctamente"
	@echo "Usa 'make deploy-local' o 'make deploy-k8s' para desplegar"