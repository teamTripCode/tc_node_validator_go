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
- Docker (para ejecución en contenedor)
- 4GB RAM (Recomendado 8GB+ para redes grandes)
- 50GB Almacenamiento (SSD recomendado)

### Variables de Entorno y Puertos

El nodo se puede configurar mediante variables de entorno. Al ejecutar con Docker, estas variables se pueden pasar al contenedor.

**Puertos Principales:**
*   **P2P_PORT (HTTP/API):** Puerto para la API HTTP principal y las interacciones P2P HTTP (ej. heartbeats, estado del nodo).
    *   Default: `3001`
    *   Variable de Entorno: `P2P_PORT`
*   **LIBP2P_PORT (TCP/UDP):** Puerto utilizado por Libp2p para la comunicación directa entre pares (descubrimiento DHT, gossip, etc.).
    *   Calculado como: `P2P_PORT + 1000` (ej. si `P2P_PORT` es 3001, Libp2p usará 4001).
*   **PBFT_WS_PORT (WebSocket):** Puerto para el servidor WebSocket de consenso PBFT.
    *   Default: `8546`
    *   Variable de Entorno: `PBFT_WS_PORT`
*   **PBOS_WS_PORT (WebSocket):** Puerto para el servidor WebSocket de pBOS.
    *   Default: `8547`
    *   Variable de Entorno: `PBOS_WS_PORT`
*   **METRICS_PORT (HTTP):** Puerto para exponer métricas en formato Prometheus.
    *   Default: `9090`
    *   Variable de Entorno: `METRICS_PORT`

**Variables de Configuración del Nodo:**
*   `DATA_DIR`: Directorio para los datos de la blockchain (bloques, estado, etc.) y claves del nodo.
    *   Default en Docker: `/app/data`
*   `CONFIG_DIR`: Directorio para archivos de configuración del nodo.
    *   Default en Docker: `/app/config`
*   `LOG_DIR`: Directorio para los archivos de log del nodo.
    *   Default en Docker: `/app/logs`
*   `NODE_KEY_PASSPHRASE`: **Requerida.** Contraseña utilizada para encriptar/desencriptar la clave privada del nodo.
*   `SEED_NODES`: Lista opcional de direcciones de nodos semilla (formato: `ip:puerto,ip:puerto`) para el arranque inicial de la red.
    *   Ejemplo: `"10.0.0.1:3001,10.0.0.2:3001"`
*   `NODE_TYPE`: Define el tipo de nodo.
    *   Default: `"validator"`
    *   Opciones: `"validator"`, `"regular"` (u otros tipos que se definan).
*   `BOOTSTRAP_PEERS`: Lista opcional de multiaddrs de peers Libp2p para el bootstrap inicial de la DHT.
    *   Ejemplo: `"/ip4/10.0.0.1/tcp/4001/p2p/QmPeerID1,/ip4/10.0.0.2/tcp/4001/p2p/QmPeerID2"`
*   `IP_SCANNER_ENABLED`: Habilita (`"true"`) o deshabilita (`"false"`) el escáner de IP para descubrir peers.
    *   Default: `"false"`
*   `IP_SCAN_RANGES`: Lista de rangos CIDR que el escáner de IP debe revisar.
    *   Default: `"127.0.0.1/24"`
    *   Ejemplo: `"192.168.1.0/24,10.0.0.0/8"`
*   `IP_SCAN_TARGET_PORT`: Puerto HTTP (el `P2P_PORT` de otros nodos) al que el escáner de IP intentará conectarse.
    *   Default: Valor de `P2P_PORT` (ej. `3001`)

**Ejecución con Docker:**

Para construir la imagen Docker:
```bash
docker build -t tripcodechain-node .
```

Para ejecutar el contenedor (ejemplo básico):
```bash
docker run -d \
  --name tcc-node \
  -p 3001:3001 \ # Mapea P2P_PORT (HTTP/API)
  -p 4001:4001 \ # Mapea LIBP2P_PORT (TCP)
  -p 4001:4001/udp \ # Mapea LIBP2P_PORT (UDP)
  -p 8546:8546 \ # Mapea PBFT_WS_PORT
  -p 8547:8547 \ # Mapea PBOS_WS_PORT
  -p 9090:9090 \ # Mapea METRICS_PORT
  -v $(pwd)/node_data:/app/data \
  -v $(pwd)/node_config:/app/config \
  -v $(pwd)/node_logs:/app/logs \
  -e NODE_KEY_PASSPHRASE="your_strong_passphrase_here" \
  -e SEED_NODES="<ip_seed_1>:<port_seed_1>" \
  tripcodechain-node
```
Asegúrate de reemplazar `"your_strong_passphrase_here"` y configurar los volúmenes y otras variables según sea necesario.

### Volúmenes Persistentes (con Docker)

Cuando se ejecuta en Docker, se recomienda mapear los siguientes directorios internos del contenedor a volúmenes en el host para persistir los datos:
*   `/app/data`: Contiene los datos de la blockchain (tx_chain, critical_chain) y las claves criptográficas del nodo (`keys`). Es crucial persistir este directorio.
*   `/app/logs`: Contiene los archivos de log generados por el nodo.
*   `/app/config`: Puede usarse para montar archivos de configuración personalizados si la aplicación los soporta.

---


