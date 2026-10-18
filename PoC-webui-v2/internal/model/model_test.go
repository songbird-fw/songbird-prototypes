package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// An empty RuleSet is valid ("rules: []", schema open question 2): it must
// never be written as null.
func TestAssignUUIDsKeepsEmptyRules(t *testing.T) {
	rs := RuleSet{APIVersion: APIVersion, Kind: "RuleSet", Metadata: Meta{Name: "x"}, Spec: RuleSetSpec{From: "a", To: "b", Rules: []Rule{}}}
	out := AssignUUIDs(rs, nil).(RuleSet)
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), `"rules":[]`) || out.Metadata.UUID == "" {
		t.Fatalf("got %s", b)
	}
	again := AssignUUIDs(out, out).(RuleSet)
	if again.Metadata.UUID != out.Metadata.UUID {
		t.Error("a uuid must never change once assigned")
	}
}

func TestPortRangeForms(t *testing.T) {
	var p PortRange
	if err := json.Unmarshal([]byte(`"2500-3300"`), &p); err != nil || p.Lo != 2500 || p.Hi != 3300 {
		t.Fatalf("%v %v", p, err)
	}
	if b, _ := json.Marshal(PortRange{443, 443}); string(b) != "443" {
		t.Errorf("single port must be a number, got %s", b)
	}
}

func TestParseDuration(t *testing.T) {
	for s, ok := range map[string]bool{"30s": true, "8h": true, "7d": true, "1h30m": true, "8 hours": false, "": false} {
		if _, got := ParseDuration(s); got != ok {
			t.Errorf("%q: got %v", s, got)
		}
	}
}
