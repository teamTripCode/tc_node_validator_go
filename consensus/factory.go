package consensus

import (
	"fmt"
	"tripcodechain_go/currency"
	"tripcodechain_go/pkg/consensus_events"
	"tripcodechain_go/pkg/consensus_types" // Added for consensus_types.ConsensusType
	"tripcodechain_go/pkg/validation"      // Added for validation.NewDPoS
)

// NewConsensus crea una nueva instancia de consenso del tipo especificado
func NewConsensus(consensusType consensus_types.ConsensusType, nodeID string, currencyManager *currency.CurrencyManager, broadcaster consensus_events.ConsensusBroadcaster, logger consensus_events.ConsensusLogger) (Consensus, error) {
	var c Consensus
	var err error

	switch consensusType {
	case "DPOS":
		// NewDPoS is now in pkg/validation
		dpos := validation.NewDPoS(currencyManager)
		// DPoS no usa broadcaster/logger de la misma manera que PBFT actualmente,
		// pero podrían pasarse si DPoS los necesitara en el futuro.
		// Por ahora, Initialize es suficiente.
		if err = dpos.Initialize(nodeID); err != nil {
			return nil, fmt.Errorf("error inicializando DPOS: %w", err)
		}
		c = dpos
	case "PBFT":
		pbft := NewPBFT(broadcaster, logger)
		if err = pbft.Initialize(nodeID); err != nil {
			return nil, fmt.Errorf("error inicializando PBFT: %w", err)
		}
		c = pbft
	default:
		return nil, fmt.Errorf("tipo de consenso no soportado: %s", consensusType)
	}

	// La inicialización específica del tipo ya se hizo arriba.
	// Si hubiera una inicialización común, iría aquí.
	// if err := c.Initialize(nodeID); err != nil { // Esto se movió a cada caso
	// 	return nil, fmt.Errorf("error inicializando consenso: %v", err)
	// }

	return c, nil
}
