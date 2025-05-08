package consensus

import "fmt"

// NewConsensus crea una nueva instancia de consenso del tipo especificado
func NewConsensus(consensusType ConsensusType, nodeID string) (Consensus, error) {
	var c Consensus

	switch consensusType {
	case "DPOS":
		c = NewDPoS()
	case "PBFT":
		c = NewPBFT()
	default:
		return nil, fmt.Errorf("tipo de consenso no soportado: %s", consensusType)
	}

	// Inicializar el algoritmo de consenso
	if err := c.Initialize(nodeID); err != nil {
		return nil, fmt.Errorf("error inicializando consenso: %v", err)
	}

	return c, nil
}
