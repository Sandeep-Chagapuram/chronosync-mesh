package consistency

import (
	"sync"
	"sync/atomic"
)

type TokenLedger struct {
	ledger      sync.Map
	globalClock uint64
}

func NewTokenLedger() *TokenLedger {
	return &TokenLedger{
		globalClock: 0,
	}
}

// MintNewToken increments the version tracker and binds it to an entity
func (tl *TokenLedger) MintNewToken(entityID string) uint64 {
	newToken := atomic.AddUint64(&tl.globalClock, 1)
	tl.ledger.Store(entityID, newToken)
	return newToken
}

// GetLatestToken retrieves the expected version check for an entity
func (tl *TokenLedger) GetLatestToken(entityID string) uint64 {
	val, exists := tl.ledger.Load(entityID)
	if !exists {
		return 0 // No updates tracked yet, completely safe to read immediately
	}
	return val.(uint64)
}
