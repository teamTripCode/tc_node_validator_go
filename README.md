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

## 🧠 Model Context Protocol (MCP) Integration

TripCodeChain integrates the Model Context Protocol (MCP) to facilitate decentralized access to and aggregation of information from Large Language Models (LLMs) or other information sources distributed across the network.

-   **Purpose**: MCP enables nodes to query other participating nodes for context or model-generated responses. This allows for richer data inputs into smart contracts or other on-chain/off-chain processes.
-   **Decentralized LLM Access**: Each validator node can run its own MCP server (integrated as a Go library within the validator node software), making its information or model accessible to others via MCP.
-   **Distributed LLM Service**: A core component, the Distributed LLM Service, is responsible for sending out queries to multiple MCP servers simultaneously. It aggregates the responses based on configurable strategies (e.g., first valid response, majority consensus).
-   **P2P Communication**: MCP messages (queries and responses) are transported over the existing P2P network infrastructure, ensuring secure and direct node-to-node communication.
-   **On-Chain Logging (Optional)**: To maintain transparency and auditability of significant MCP interactions (like those that might trigger critical on-chain actions), the system supports logging MCP activities. This is done via a special transaction type, `LogMCPActivityTransaction`, recorded on the DPoS (Transaction) chain. This provides an immutable record of query initiations and aggregated response hashes.

The MCP and Distributed LLM Service components extend the capabilities of the TripCodeChain network beyond simple transactions and critical process logging, enabling more complex, data-driven decentralized applications.

*(Note: The architecture overview/diagram should be updated to visually include the MCP servers, the Distributed LLM Service, and their interactions with the P2P network and the DPoS chain.)*

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


