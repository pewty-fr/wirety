//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// dumpChain returns `iptables -S <chain>` output from inside the agent container.
// An error (e.g. chain missing) is returned as a non-nil error so callers can
// poll with eventually().
func dumpChain(ctx context.Context, c testcontainers.Container, chain string) (string, error) {
	code, out, err := execInContainer(ctx, c, "iptables", "-S", chain)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return out, &execError{chain: chain, code: code, out: out}
	}
	return out, nil
}

// dumpChain6 is the ip6tables equivalent.
func dumpChain6(ctx context.Context, c testcontainers.Container, chain string) (string, error) {
	code, out, err := execInContainer(ctx, c, "ip6tables", "-S", chain)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return out, &execError{chain: chain, code: code, out: out}
	}
	return out, nil
}

type execError struct {
	chain string
	code  int
	out   string
}

func (e *execError) Error() string {
	return "chain " + e.chain + " not ready (exit " + itoa(e.code) + "): " + e.out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

// requireChainEventually polls until `iptables -S chain` succeeds and contains
// all the given substrings, then returns the final dump.
func requireChainEventually(ctx context.Context, t *testing.T, c testcontainers.Container, chain string, contains ...string) string {
	t.Helper()
	var final string
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		out, err := dumpChain(ctx, c, chain)
		if err != nil {
			return err
		}
		final = out
		if !containsAll(out, contains...) {
			return &execError{chain: chain, code: 0, out: "missing expected rules in:\n" + out}
		}
		return nil
	})
	return final
}
