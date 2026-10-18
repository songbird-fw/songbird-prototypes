package defaults_test

import (
	"testing"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

// The factory default must be clean: no error and no warning, so that any
// finding in the UI comes from a user change.
func TestFactoryDefaultIsClean(t *testing.T) {
	for _, e := range validator.Validate(defaults.Tree()) {
		t.Error(e)
	}
}
