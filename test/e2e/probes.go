//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// digEventually queries dnsServer for a qtype ("A", "AAAA") record of name from
// inside c until the answer is exactly want ("" = an empty answer), and
// returns it.
func digEventually(ctx context.Context, t *testing.T, c testcontainers.Container, dnsServer, name, qtype, want string) string {
	t.Helper()
	return digEventuallyMatch(ctx, t, c, dnsServer, name, qtype, fmt.Sprintf("%q", want),
		func(got string) bool { return got == want })
}

// digEventuallyNot is digEventually for "any answer except notWant" (e.g. a
// name that must no longer be steered to the captive portal, whose real
// upstream answer is not fixed).
func digEventuallyNot(ctx context.Context, t *testing.T, c testcontainers.Container, dnsServer, name, qtype, notWant string) string {
	t.Helper()
	return digEventuallyMatch(ctx, t, c, dnsServer, name, qtype, fmt.Sprintf("anything but %q", notWant),
		func(got string) bool { return !strings.Contains(got, notWant) })
}

func digEventuallyMatch(ctx context.Context, t *testing.T, c testcontainers.Container,
	dnsServer, name, qtype, desc string, ok func(string) bool) string {
	t.Helper()
	var got string
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		code, out, err := execInContainer(ctx, c, "dig", "+short", "+time=2", "+tries=1", "@"+dnsServer, name, qtype)
		got = strings.TrimSpace(out)
		if err != nil {
			return err
		}
		if code != 0 || !ok(got) {
			return fmt.Errorf("dig %s %s @%s: want %s (exit %d): %q", qtype, name, dnsServer, desc, code, got)
		}
		return nil
	})
	return got
}

// httpProbe issues one HTTP GET from inside c without following redirects and
// returns the status code and redirect target. A connection that is dropped or
// refused yields status "000" (curl's convention) rather than an error; err is
// reserved for failures to run the probe at all.
func httpProbe(ctx context.Context, c testcontainers.Container, url string) (status, location string, err error) {
	// -g: accept IPv6 literals such as http://[fd00::1]/ unescaped.
	_, out, err := execInContainer(ctx, c, "curl", "-g", "-s", "-o", "/dev/null", "--max-time", "5",
		"-w", "%{http_code} %{redirect_url}", url)
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", "", fmt.Errorf("curl %s: unexpected output %q", url, out)
	}
	status = fields[0]
	if len(fields) > 1 {
		location = fields[1]
	}
	return status, location, nil
}
