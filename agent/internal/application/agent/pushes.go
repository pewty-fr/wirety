package agent

import (
	"crypto/sha256"
	"encoding/json"
	"sync"
	"time"
)

// pushReapplyInterval is how often an unchanged push is applied anyway, so a
// manual change to the WireGuard interface or to the firewall gets repaired.
const pushReapplyInterval = 5 * time.Minute

// appliedPushes remembers what the server's pushes last applied. The server
// pushes the network state at every jump heartbeat (every 30 s), almost always
// unchanged: re-applying it would rewrite the WireGuard config, reload DNS and
// rebuild the firewall each time.
type appliedPushes struct {
	mu   sync.Mutex
	last map[string]appliedPush
}

type appliedPush struct {
	sum [sha256.Size]byte
	at  time.Time
}

// unchanged reports whether v is what was last applied under name, less than
// maxAge ago (0: whatever its age).
func (a *appliedPushes) unchanged(name string, v any, maxAge time.Duration) bool {
	sum, ok := pushSum(v)
	if !ok {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	last, found := a.last[name]
	return found && last.sum == sum && (maxAge == 0 || time.Since(last.at) < maxAge)
}

// applied records v as applied under name.
func (a *appliedPushes) applied(name string, v any) {
	sum, ok := pushSum(v)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !ok {
		delete(a.last, name)
		return
	}
	if a.last == nil {
		a.last = make(map[string]appliedPush)
	}
	a.last[name] = appliedPush{sum: sum, at: time.Now()}
}

func pushSum(v any) ([sha256.Size]byte, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256(b), true
}
