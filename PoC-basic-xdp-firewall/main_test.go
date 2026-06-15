package main

import (
	"testing"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

func TestLoadObjects(t *testing.T) {
	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatal(err)
	}

	spec, err := ebpf.LoadCollectionSpec("bpf/firewall.bpf.o")
	if err != nil {
		t.Fatalf("Failed to load BPF spec: %v", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		t.Fatalf("Failed to create new collection: %v", err)
	}
	defer coll.Close()

	if coll.Programs["firewall_filter"] == nil {
		t.Error("Program 'firewall_filter' not found")
	}
	if coll.Maps["rules"] == nil {
		t.Error("Map 'rules' not found")
	}
	if coll.Maps["events"] == nil {
		t.Error("Map 'events' not found")
	}

	rulesMap := coll.Maps["rules"]

	// Test rule insertion
	key := RuleKey{Proto: 6, DstPort: htons(80)}
	val := RuleValue{Action: 0, RuleID: 100}

	if err := rulesMap.Put(key, val); err != nil {
		t.Errorf("Failed to put rule: %v", err)
	}

	var lookupVal RuleValue
	if err := rulesMap.Lookup(key, &lookupVal); err != nil {
		t.Errorf("Failed to lookup rule: %v", err)
	}

	if lookupVal.RuleID != 100 || lookupVal.Action != 0 {
		t.Errorf("Lookup returned wrong value: %+v", lookupVal)
	}
}
