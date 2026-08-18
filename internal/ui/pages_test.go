package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tomsk73/chaintui/internal/api"
)

func TestWriteLibraryInventory(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	inv := api.LibraryInventory{
		Ecosystem:    "python",
		GeneratedAt:  time.Date(2026, 8, 18, 14, 30, 0, 0, time.UTC),
		PackageCount: 1,
		VersionCount: 2,
		Packages: []api.InventoryPackage{
			{Name: "requests", LatestVersion: "2.32.3", Versions: []string{"2.31.0", "2.32.3"}},
		},
	}

	name, err := writeLibraryInventory(inv)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if name != "20260818T143000Z-python.json" {
		t.Fatalf("name=%q", name)
	}

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	var got api.LibraryInventory
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Ecosystem != "python" || len(got.Packages) != 1 {
		t.Fatalf("round trip: %+v", got)
	}
	if pkg := got.Packages[0]; pkg.Name != "requests" || len(pkg.Versions) != 2 {
		t.Fatalf("package: %+v", got.Packages[0])
	}

	// Same timestamp again must not clobber the existing snapshot.
	if _, err := writeLibraryInventory(inv); !os.IsExist(err) {
		t.Fatalf("second write err=%v, want already-exists", err)
	}
}
