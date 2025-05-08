# TripCodeChain Validator Node

**Dual-Blockchain System with Delegated Proof-of-Stake Consensus for Secure Transaction and Critical Process Management**

---

## 📜 Descripción General

TripCodeChain Validator Node es una implementación avanzada de blockchain que opera dos cadenas paralelas:
1. **Cadena de Transacciones**: Para operaciones financieras y transferencias de valor
2. **Cadena de Procesos Críticos**: Para gestión de contratos inteligentes y operaciones sistémicas vitales

El sistema emplea:
- **DPoS (Delegated Proof-of-Stake)**: Mecanismo de consenso eficiente y escalable
- **Arquitectura Dual-Chain**: Separación de preocupaciones para distintos tipos de operaciones
- **Red P2P**: Comunicación descentralizada entre nodos validadores
- **Mecanismos Anti-Spam**: Mempools gestionados con límites de gas y priorización

---

## 🚦 Estado del Proyecto

**Fase Actual:** 🚧 Desarrollo Activo (Versión Alfa)

**Última Actualización:**  
Sistema básico de consenso operativo  
Mecanismos de sincronización de red implementados  
API de gestión y monitoreo funcional

---

## 🛠️ Características Principales

### Sistema de Doble Blockchain
- **Transacciones Regulares:**
  - Sistema de gas y comisiones ajustables
  - Validación criptográfica de firmas
  - Procesamiento por lotes (batch transactions)
  
- **Procesos Críticos:**
  - Registros inmutables de alta prioridad
  - Estructuras de datos complejas anidadas
  - Mecanismos de consenso reforzados

### Implementación de DPoS
- Selección dinámica de delegados
- Sistema de votación ponderada por stake
- Rotación programada de productores de bloques
- Penalizaciones por inactividad de delegados

### Red y Seguridad
- Descubrimiento automático de nodos
- Protocolo heartbeat para monitoreo de red
- Cifrado SHA-256 para todas las comunicaciones
- Sistema de reputación nodal integrado

### Optimizaciones de Rendimiento
- Procesamiento paralelo de mempools
- Compresión selectiva de datos en bloques
- Cacheado inteligente de estados de cadena
- Balanceo dinámico de carga de trabajo

---

## 🧩 Tecnologías Utilizadas

**Núcleo del Sistema:**
- Go 1.24.2 (Rendimiento nativo y concurrencia)
- Gorilla Mux (Enrutamiento HTTP avanzado)
- SHA-256 (Hashing criptográfico)

**Módulos Principales:**
- Blockchain: Sistema dual-chain con minería adaptativa
- Consenso: Implementación personalizada de DPoS
- Red: Capa P2P con autodetección de nodos
- Contratos: Motor de ejecución WASM (en desarrollo)

**Herramientas de Soporte:**
- JSON-RPC 2.0 (Comunicación API)
- Protocolo Buffers (Serialización eficiente)
- Prometheus (Métricas en tiempo real)
- Grafana (Tableros de visualización)

---

## 🖥️ Instalación y Configuración

### Requisitos Mínimos
- Go 1.24.2+ ([Instalación](https://go.dev/dl/))
- 4GB RAM (Recomendado 8GB+ para redes grandes)
- 50GB Almacenamiento (SSD recomendado)
- Puerto TCP/UDP abierto (default: 3000)


