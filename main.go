// main.go
package main

import (
	"flag"
	"log"
	"os"

	"tripcodechain_go/p2p"
	"tripcodechain_go/utils"
)

func main() {
	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	// Parse command line flags
	portFlag := flag.Int("port", 3000, "Port to listen on")
	verboseFlag := flag.Bool("verbose", true, "Enable verbose logging")
	flag.Parse()

	// Set verbose logging mode
	utils.SetVerbose(*verboseFlag)

	// Create a new node
	node := p2p.NewNode(*portFlag)
	utils.PrintStartupMessage(node.ID, *portFlag)
}
