package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAndReadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "database.sql", "CREATE TABLE foo;")
	writeTestFile(t, dir, "config.php", "<?php")

	written, err := WriteManifest(dir, []string{"database.sql", "config.php"})
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Files) != 2 {
		t.Fatalf("expected 2 files in manifest, got %d", len(written.Files))
	}

	read, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if read.Version != ManifestVersion {
		t.Errorf("version = %d, want %d", read.Version, ManifestVersion)
	}
	if len(read.Files) != 2 || read.Files[0].SHA256 != written.Files[0].SHA256 {
		t.Errorf("manifest round-trip mismatch: wrote %+v, read %+v", written, read)
	}
}

func TestManifestValidateSucceedsForIntactBackup(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "database.sql", "some data")

	m, err := WriteManifest(dir, []string{"database.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(dir); err != nil {
		t.Errorf("expected valid backup, got error: %v", err)
	}
}

func TestManifestValidateDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "database.sql", "original data")

	m, err := WriteManifest(dir, []string{"database.sql"})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate corruption/truncation after the manifest was written.
	writeTestFile(t, dir, "database.sql", "tampered data!!")

	if err := m.Validate(dir); err == nil {
		t.Fatal("expected Validate to detect the checksum mismatch")
	}
}

func TestManifestValidateDetectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "database.sql", "data")

	m, err := WriteManifest(dir, []string{"database.sql"})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "database.sql"))

	if err := m.Validate(dir); err == nil {
		t.Fatal("expected Validate to fail for a missing file")
	}
}

func TestManifestValidateRejectsEmptyManifest(t *testing.T) {
	m := Manifest{Version: ManifestVersion}
	if err := m.Validate(t.TempDir()); err == nil {
		t.Fatal("expected Validate to reject a manifest listing no files")
	}
}

func TestReadManifestMissingFile(t *testing.T) {
	_, err := ReadManifest(t.TempDir())
	if err == nil {
		t.Fatal("expected error reading a nonexistent manifest")
	}
}

func TestPruneKeepsOnlyMostRecent(t *testing.T) {
	root := t.TempDir()
	names := []string{"20260101-000000", "20260102-000000", "20260103-000000", "20260104-000000"}
	for _, n := range names {
		if err := os.Mkdir(filepath.Join(root, n), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	if err := prune(root, 2); err != nil {
		t.Fatal(err)
	}

	remaining, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining backups, got %d: %v", len(remaining), remaining)
	}
	want := map[string]bool{"20260104-000000": true, "20260103-000000": true}
	for _, n := range remaining {
		if !want[n] {
			t.Errorf("unexpected surviving backup %s", n)
		}
	}
}

func TestPruneNoOpWhenRetainIsZeroOrBelowLimit(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "20260101-000000"), 0o750)

	if err := prune(root, 0); err != nil {
		t.Fatal(err)
	}
	remaining, _ := List(root)
	if len(remaining) != 1 {
		t.Errorf("expected prune(retain=0) to be a no-op, got %v", remaining)
	}
}

func TestListReturnsNewestFirst(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"20260101-000000", "20260103-000000", "20260102-000000"} {
		os.Mkdir(filepath.Join(root, n), 0o750)
	}

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"20260103-000000", "20260102-000000", "20260101-000000"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestListEmptyDirWhenBackupRootMissing(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected no backups, got %v", got)
	}
}
