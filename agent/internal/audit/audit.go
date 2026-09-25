package audit

import (
	"io"
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
// When enabled, audit events are written to stdout in the same format as the
// rest of the agent's logs ("json", or human-readable text otherwise), so a
// text-format deployment doesn't get JSON lines mixed into its journal.
func Init(enabled bool, format string) {
	once.Do(func() {
		if enabled {
			logger = newLogger(os.Stdout, format)
		}
	})
}

func newLogger(w io.Writer, format string) zerolog.Logger {
	if format != "json" {
		w = zerolog.ConsoleWriter{Out: w}
	}
	return zerolog.New(w).With().
		Timestamp().
		Str("log_type", "audit").
		Logger()
}

// Agent returns a pre-populated event for agent-side audit entries.
// The caller must call .Msg("audit") to emit the event.
func Agent(peerID, networkID string) *zerolog.Event {
	return logger.Info().
		Str("actor_type", "agent").
		Str("peer_id", peerID).
		Str("network_id", networkID)
}
