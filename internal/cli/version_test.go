package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandPrintsBuildInformation(t *testing.T) {
	command := newVersionCommand()

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := strings.TrimSpace(output.String())
	want := "sweep version dev (commit unknown, built unknown)"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestRootCommandVersionFlagMatchesVersionCommand(t *testing.T) {
	rootCommand := newRootCommand()

	var output bytes.Buffer
	rootCommand.SetOut(&output)
	rootCommand.SetArgs([]string{"--version"})

	if err := rootCommand.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := strings.TrimSpace(output.String())
	want := "sweep version dev (commit unknown, built unknown)"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
