package p2p

import (
	"github.com/gin-gonic/gin"
)

// Init initializes P2P module and registers routes
func Init(r *gin.Engine, dsn string) error {
	store, err := NewP2PStore(r.Context, dsn)
	if err != nil {
		return err
	}

	handlers := NewHandlers(store)
	handlers.RegisterRoutes(r)
	
	return nil
}