package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLoggerFollowsLogFormat(t *testing.T) {
	var text bytes.Buffer
	l := newLogger(&text, "text")
	l.Info().Str("action", "config.sync").Msg("audit")
	out := text.String()
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("text format produced a JSON line: %q", out)
	}
	for _, want := range []string{"audit", "log_type=", "action=", "config.sync"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output %q is missing %q", out, want)
		}
	}

	var js bytes.Buffer
	l = newLogger(&js, "json")
	l.Info().Str("action", "config.sync").Msg("audit")
	var fields map[string]any
	if err := json.Unmarshal(js.Bytes(), &fields); err != nil {
		t.Fatalf("json format produced invalid JSON %q: %v", js.String(), err)
	}
	if fields["log_type"] != "audit" || fields["action"] != "config.sync" {
		t.Errorf("unexpected JSON fields: %v", fields)
	}
}
