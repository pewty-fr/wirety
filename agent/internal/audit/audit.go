package audit

import (
	"os"
	"sync"

	"github.com/rs/zerolog"
)

var (
	once   sync.Once
	logger zerolog.Logger
)

func init() { logger = zerolog.Nop() }

// Init enables the audit logger. Must be called once at startup.
// When enabled, audit events are written as JSON to stdout.
func Init(enabled bool) {
	once.Do(func() {
		if enabled {
			logger = zerolog.New(os.Stdout).With().
				Timestamp().
				Str("log_type", "audit").
				Logger()
		}
	})
}

// Agent returns a pre-populated event for agent-side audit entries.
// The caller must call .Msg("audit") to emit the event.
func Agent(peerID, networkID string) *zerolog.Event {
	return logger.Info().
		Str("actor_type", "agent").
		Str("peer_id", peerID).
		Str("network_id", networkID)
}
