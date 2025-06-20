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
- **Descubrimiento Automático y Dinámico de Nodos:**
    - **Registro Inicial con Nodos Semilla:** Al arrancar, un nodo puede registrarse con un nodo semilla configurado (por ejemplo, mediante un flag como `-seed <seed_node_address>`) para obtener una lista inicial de pares, facilitando su rápida incorporación a la red.
    - **Descubrimiento Basado en DHT (Distributed Hash Table):** Utilizamos una tabla Kademlia DHT para un descubrimiento de pares robusto y descentralizado. Los nodos se conectan a la DHT utilizando una lista de pares de arranque iniciales y luego participan activamente en el mantenimiento de la tabla de enrutamiento.
    - **Anuncio y Búsqueda de Validadores en DHT:** Los nodos validadores se anuncian a sí mismos en la DHT bajo una etiqueta de servicio específica (`ValidatorServiceTag`). Esto permite que otros nodos, y especialmente los nuevos nodos que se unen a la red, puedan encontrar eficientemente a los validadores activos para participar en el consenso y sincronizar el estado de la cadena.
    - **Escaneo de Red (Opcional y Complementario):** Como mecanismo adicional, especialmente útil en las fases iniciales de la red o para mejorar la conectividad, los nodos tienen la capacidad opcional de escanear rangos de IP preconfigurados para descubrir otros participantes activos.
- **Protocolo Heartbeat y Monitoreo de Red:** Se utiliza un sistema de heartbeats para que los nodos monitoreen la actividad y disponibilidad de sus pares conocidos. Esto, junto con la información de las vistas de validadores compartidas, alimenta el sistema de detección de particiones de red, mejorando la resiliencia general.
- **Cifrado SHA-256 para todas las comunicaciones:** (Este punto ya existía y es relevante, aunque SHA-256 es para hashing, no cifrado de comunicación. La comunicación P2P de libp2p usa cifrado autenticado como TLS, Noise. Se mantiene por ahora pero podría requerir revisión conceptual).
- **Sistema de reputación nodal integrado:** Los nodos mantienen un puntaje de reputación para sus pares basado en interacciones exitosas, latencia y comportamiento, ayudando a priorizar conexiones con pares fiables y penalizar a los maliciosos o no fiables.
- **Objetivos de la Capa P2P:** Estos mecanismos trabajan conjuntamente para asegurar que los nodos puedan descubrirse dinámicamente, mantener una visión actualizada y coherente de la red, y localizar específicamente a los nodos validadores cruciales para el proceso de consenso y la integridad de las blockchains.

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


