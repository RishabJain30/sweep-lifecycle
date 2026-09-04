package cli

import (
	"bytes"
	"testing"
)

func TestScanCommand(t *testing.T) {
	cmd := newRootCommand()

	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"scan"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute scan command: %v", err)
	}

	want := "Scanning...\n"
	if got := output.String(); got != want {
		t.Fatalf("unexpected output:\nwant: %q\ngot:  %q", want, got)
	}
}
