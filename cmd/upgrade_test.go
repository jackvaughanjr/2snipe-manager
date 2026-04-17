package cmd

import (
	"testing"
)

func TestUpgradeNeeded_OlderInstalled(t *testing.T) {
	needed, warn := upgradeNeeded("0.0.1", "1.2.0")
	if !needed {
		t.Fatal("expected upgrade needed when installed 0.0.1 < manifest 1.2.0")
	}
	if warn != "" {
		t.Fatalf("unexpected warning for older installed version: %s", warn)
	}
}

func TestUpgradeNeeded_SameVersion(t *testing.T) {
	needed, warn := upgradeNeeded("1.2.0", "1.2.0")
	if needed {
		t.Fatal("expected no upgrade when installed == manifest")
	}
	if warn != "" {
		t.Fatalf("unexpected warning for matching versions: %s", warn)
	}
}

func TestUpgradeNeeded_NewerInstalled(t *testing.T) {
	needed, warn := upgradeNeeded("1.3.0", "1.2.0")
	if needed {
		t.Fatal("expected no upgrade when installed 1.3.0 > manifest 1.2.0")
	}
	if warn == "" {
		t.Fatal("expected a warning when installed version is ahead of manifest")
	}
}
