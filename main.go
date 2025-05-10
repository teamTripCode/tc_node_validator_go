package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/consensus"
	"tripcodechain_go/contracts"
	"tripcodechain_go/mempool"
	"tripcodechain_go/p2p"
	"tripcodechain_go/utils"
)

func main() {
	// Configurar logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	// Parsear flags
	portFlag := flag.Int("port", 3000, "Puerto para escuchar")
	verboseFlag := flag.Bool("verbose", true, "Habilitar logging detallado")
	flag.Parse()

	// Configurar modo verbose
	utils.SetVerbose(*verboseFlag)

	// Crear nuevo nodo
	node := p2p.NewNode(*portFlag)
	utils.PrintStartupMessage(node.ID, *portFlag)

	// Inicializar cadenas de bloques
	txChain := blockchain.NewBlockchain(blockchain.TransactionBlock)
	criticalChain := blockchain.NewBlockchain(blockchain.CriticalProcessBlock)
	utils.LogInfo("Cadenas inicializadas - Transacciones: %d bloques, Críticos: %d bloques",
		txChain.GetLength(), criticalChain.GetLength())

	// Inicializar sistema económico
	currencyManager := blockchain.InitNativeToken(txChain, "TC", 1000000)
	utils.LogInfo("Sistema de moneda nativa inicializado")

	// Configurar consensos duales
	consensusTx, err := consensus.NewConsensus("DPOS", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error inicializando consenso DPoS:", err)
	}

	consensusCritical, err := consensus.NewConsensus("PBFT", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error inicializando consenso PBFT:", err)
	}

	// Asignar consensos a las cadenas correspondientes
	txChain.SetConsensus(consensusTx)
	criticalChain.SetConsensus(consensusCritical)
	utils.LogInfo("Sistema de consenso dual configurado - DPoS para transacciones, PBFT para procesos críticos")

	// Desplegar contratos del sistema
	contracts.DeploySystemContracts(txChain)
	utils.LogInfo("Contratos inteligentes base desplegados")

	// Inicializar mempools
	txMempool := mempool.NewMempool()
	criticalMempool := mempool.NewMempool()
	utils.LogInfo("Mempools inicializados - Transacciones: %d, Procesos: %d",
		txMempool.GetSize(), criticalMempool.GetSize())

	// Configurar e iniciar servidor
	server := p2p.NewServer(node, txChain, criticalChain, txMempool, criticalMempool)

	// Iniciar procesos en segundo plano
	server.StartBackgroundProcessing()
	utils.LogInfo("Procesamiento en segundo plano iniciado")

	// Iniciar monitoreo de nodo
	go node.StartHeartbeat()
	utils.LogInfo("Heartbeat del nodo iniciado")

	// Descubrir otros nodos
	go func() {
		time.Sleep(2 * time.Second)
		node.DiscoverNodes()
		utils.LogInfo("Iniciado descubrimiento de nodos...")
	}()

	// Manejar apagado seguro
	setupGracefulShutdown()

	// Iniciar servidor (bloqueante)
	utils.LogInfo("Iniciando servidor en el puerto %d...", *portFlag)
	server.Start()
}

func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		utils.LogInfo("Apagando nodo...")
		// Aquí deberías añadir limpieza de recursos
		os.Exit(0)
	}()
}
