package main

import (
	"testing"
)

func TestRootCmdUsage(t *testing.T) {
	if rootCmd.Use != "loopctl" {
		t.Errorf("expected rootCmd.Use = %q, got %q", "loopctl", rootCmd.Use)
	}
}
