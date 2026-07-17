package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseProcStatHandlesSpacesInCommand(t *testing.T) {
	fields := []string{
		"S", "100", "200", "200", "0", "-1", "4194304", "1", "2", "3",
		"4", "5", "6", "7", "8", "20", "0", "1", "0", "987654",
	}
	startTicks, processGroupID, err := parseProcStat("123 (service with spaces) " + strings.Join(fields, " "))
	if err != nil {
		t.Fatal(err)
	}
	if startTicks != 987654 || processGroupID != 200 {
		t.Fatalf("startTicks=%d processGroupID=%d", startTicks, processGroupID)
	}
}

func TestVerifyProcessIdentityDetectsMismatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc test")
	}
	identity, err := captureProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyProcessIdentity(os.Getpid(), identity); err != nil {
		t.Fatal(err)
	}
	identity.StartTicks++
	if _, err := verifyProcessIdentity(os.Getpid(), identity); err == nil {
		t.Fatal("changed process identity was accepted")
	}
}
