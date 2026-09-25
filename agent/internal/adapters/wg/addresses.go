package wg

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// syncAddresses makes the interface's global addresses match the config's
// Address lines.
//
// `wg syncconf` only syncs keys and peers (`wg-quick strip` drops Address), so
// without this an interface that outlives an agent restart keeps the addresses
// it was created with. When an address is added later — typically IPv6 enabled
// on an existing network — it is never assigned, and everything bound to it
// (DNS, captive portal) fails with "cannot assign requested address".
func (w *Writer) syncAddresses() error {
	cfg, err := os.ReadFile(w.Path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	want, err := configAddresses(string(cfg))
	if err != nil {
		return err
	}
	out, err := exec.Command("ip", "-o", "address", "show", "dev", w.Interface, "scope", "global").Output() // #nosec G204 - w.Interface is sanitized and controlled
	if err != nil {
		return fmt.Errorf("list interface addresses: %w", err)
	}
	have, err := interfaceAddresses(string(out))
	if err != nil {
		return err
	}

	add, del := addressDiff(want, have)
	for _, p := range add {
		if err := runIP("address", "add", p.String(), "dev", w.Interface); err != nil {
			return err
		}
		log.Info().Str("interface", w.Interface).Str("address", p.String()).Msg("added missing interface address")
	}
	for _, p := range del {
		if err := runIP("address", "del", p.String(), "dev", w.Interface); err != nil {
			return err
		}
		log.Info().Str("interface", w.Interface).Str("address", p.String()).Msg("removed stale interface address")
	}
	return nil
}

func runIP(args ...string) error {
	cmd := exec.Command("ip", args...) // #nosec G204 - parameters are controlled
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ip %s: %v stderr=%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// configAddresses returns the addresses of a WireGuard config's [Interface]
// Address lines (comma-separated, possibly repeated). A bare address is a host
// prefix, as `ip address add` treats it.
func configAddresses(cfg string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, line := range strings.Split(cfg, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Address") {
			continue
		}
		for _, field := range strings.Split(value, ",") {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			p, err := parseAddress(field)
			if err != nil {
				return nil, fmt.Errorf("config Address %q: %w", field, err)
			}
			out = append(out, p)
		}
	}
	return out, nil
}

// interfaceAddresses parses `ip -o address show` output: one address per
// line, "<idx>: <iface> inet|inet6 <addr>/<len> ...".
func interfaceAddresses(ipOutput string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, line := range strings.Split(ipOutput, "\n") {
		fields := strings.Fields(line)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] != "inet" && fields[i] != "inet6" {
				continue
			}
			p, err := parseAddress(fields[i+1])
			if err != nil {
				return nil, fmt.Errorf("ip output %q: %w", line, err)
			}
			out = append(out, p)
			break
		}
	}
	return out, nil
}

func parseAddress(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}

// addressDiff returns the addresses to add (wanted, not present) and to
// delete (present, not wanted).
func addressDiff(want, have []netip.Prefix) (add, del []netip.Prefix) {
	haveSet := make(map[netip.Prefix]bool, len(have))
	for _, p := range have {
		haveSet[p] = true
	}
	wantSet := make(map[netip.Prefix]bool, len(want))
	for _, p := range want {
		wantSet[p] = true
		if !haveSet[p] {
			add = append(add, p)
		}
	}
	for _, p := range have {
		if !wantSet[p] {
			del = append(del, p)
		}
	}
	return add, del
}
