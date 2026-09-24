//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
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

// hasRule reports whether one rule (line) of an `iptables -S` dump contains
// every fragment. iptables normalises rule order ("-s X -i if -j T"), so a
// rule must be matched by its parts rather than as one substring.
func hasRule(dump string, fragments ...string) bool {
	for _, line := range strings.Split(dump, "\n") {
		if containsAll(line, fragments...) {
			return true
		}
	}
	return false
}

// requireRuleEventually polls until chain holds a rule containing every
// fragment, then returns the final dump.
func requireRuleEventually(ctx context.Context, t *testing.T, c testcontainers.Container, chain string, fragments ...string) string {
	t.Helper()
	return requireRuleEventuallyWith(ctx, t, c, dumpChain, chain, fragments...)
}

// requireRule6Eventually is requireRuleEventually for an ip6tables chain.
func requireRule6Eventually(ctx context.Context, t *testing.T, c testcontainers.Container, chain string, fragments ...string) string {
	t.Helper()
	return requireRuleEventuallyWith(ctx, t, c, dumpChain6, chain, fragments...)
}

func requireRuleEventuallyWith(ctx context.Context, t *testing.T, c testcontainers.Container,
	dump func(context.Context, testcontainers.Container, string) (string, error), chain string, fragments ...string) string {
	t.Helper()
	var final string
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		out, err := dump(ctx, c, chain)
		if err != nil {
			return err
		}
		final = out
		if !hasRule(out, fragments...) {
			return &execError{chain: chain, code: 0, out: fmt.Sprintf("no rule matching %q in:\n%s", fragments, out)}
		}
		return nil
	})
	return final
}
