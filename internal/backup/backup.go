// Package backup implements FR-08 (Backup) and FR-09 (Restore): capturing
// the MariaDB database, Nextcloud configuration, and user files into a
// timestamped, checksummed backup set (PRD section 14), and restoring one
// back onto a stopped-then-restarted stack.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/2005Hari/campuscloud/internal/config"
	"github.com/2005Hari/campuscloud/internal/deployment"
	"github.com/2005Hari/campuscloud/internal/dockercli"
)

const (
	manifestFileName = "manifest.json"
	databaseFileName = "database.sql"
	configFileName   = "config.php"
	filesArchiveName = "user-files.tar.gz"

	// ManifestVersion is bumped whenever the on-disk backup layout changes
	// in a way that Restore needs to know about.
	ManifestVersion = 1
)

// FileEntry records one file captured in a backup set, with enough
// information for Restore to validate it wasn't truncated or corrupted.
type FileEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest describes the contents of one backup set.
type Manifest struct {
	Version   int         `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	Files     []FileEntry `json:"files"`
}

func hashFile(path string) (FileEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileEntry{}, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return FileEntry{}, err
	}
	return FileEntry{
		Name:   filepath.Base(path),
		SHA256: hex.EncodeToString(h.Sum(nil)),
		Size:   size,
	}, nil
}

// WriteManifest hashes each named file (relative to dir) and writes
// manifest.json describing them.
func WriteManifest(dir string, fileNames []string) (Manifest, error) {
	m := Manifest{Version: ManifestVersion, CreatedAt: time.Now().UTC()}
	for _, name := range fileNames {
		entry, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return Manifest{}, fmt.Errorf("hashing %s: %w", name, err)
		}
		m.Files = append(m.Files, entry)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), data, 0o640); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ReadManifest loads and parses manifest.json from a backup directory.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFileName))
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing manifest: %w", err)
	}
	return m, nil
}

// Validate re-hashes every file the manifest lists (relative to dir) and
// confirms it is present, the right size, and matches its recorded
// checksum — FR-09's "validate a backup" step.
func (m Manifest) Validate(dir string) error {
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest lists no files")
	}
	for _, want := range m.Files {
		got, err := hashFile(filepath.Join(dir, want.Name))
		if err != nil {
			return fmt.Errorf("%s: %w", want.Name, err)
		}
		if got.Size != want.Size {
			return fmt.Errorf("%s: size mismatch: expected %d bytes, found %d", want.Name, want.Size, got.Size)
		}
		if got.SHA256 != want.SHA256 {
			return fmt.Errorf("%s: checksum mismatch (backup may be corrupted)", want.Name)
		}
	}
	return nil
}

// mysqldumpEnv is the shell fragment run inside the mariadb container. It
// reads MYSQL_ROOT_PASSWORD from the container's own environment (set by
// docker-compose.yml/.env) via MYSQL_PWD, so the credential never appears
// as a process argument or in host-side logs.
const mysqldumpEnv = `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump -uroot --single-transaction --databases "$MYSQL_DATABASE"`
const mysqlRestoreEnv = `MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot`

// Create implements FR-08: dump the database, copy the Nextcloud
// configuration, archive user files, and write a checksummed manifest,
// all under a new timestamped directory inside cfg.Backup.Dir.
func Create(ctx context.Context, cfg *config.Config, client *dockercli.Client, out io.Writer) (string, error) {
	timestamp := time.Now().UTC().Format("20060102-150405")
	dir := filepath.Join(cfg.Backup.Dir, timestamp)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	fmt.Fprintln(out, "==> Dumping database")
	dbPath := filepath.Join(dir, databaseFileName)
	dbFile, err := os.Create(dbPath)
	if err != nil {
		return "", err
	}
	dumpErr := client.ComposeExecOut(ctx, dbFile, cfg.MariaDBService(), "sh", "-c", mysqldumpEnv)
	dbFile.Close()
	if dumpErr != nil {
		return "", fmt.Errorf("dumping database: %w", dumpErr)
	}

	fmt.Fprintln(out, "==> Copying Nextcloud configuration")
	configPath := filepath.Join(dir, configFileName)
	if err := client.ComposeCp(ctx, cfg.NextcloudService()+":/var/www/html/config/config.php", configPath); err != nil {
		return "", fmt.Errorf("copying nextcloud config: %w", err)
	}

	fmt.Fprintln(out, "==> Archiving user files")
	archivePath := filepath.Join(dir, filesArchiveName)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	archiveErr := client.ComposeExecOut(ctx, archiveFile, cfg.NextcloudService(), "tar", "-czf", "-", "-C", "/var/www/html/data", ".")
	archiveFile.Close()
	if archiveErr != nil {
		return "", fmt.Errorf("archiving user files: %w", archiveErr)
	}

	fmt.Fprintln(out, "==> Writing manifest")
	if _, err := WriteManifest(dir, []string{databaseFileName, configFileName, filesArchiveName}); err != nil {
		return "", fmt.Errorf("writing manifest: %w", err)
	}

	if err := prune(cfg.Backup.Dir, cfg.Backup.Retain); err != nil {
		fmt.Fprintf(out, "warning: failed to prune old backups: %v\n", err)
	}

	fmt.Fprintf(out, "\nBackup complete: %s\n", dir)
	return dir, nil
}

// prune removes the oldest backup directories once there are more than
// retain (a retain of 0 or less disables pruning).
func prune(backupRoot string, retain int) error {
	if retain <= 0 {
		return nil
	}
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs) // timestamp-named directories sort chronologically
	if len(dirs) <= retain {
		return nil
	}
	for _, name := range dirs[:len(dirs)-retain] {
		if err := os.RemoveAll(filepath.Join(backupRoot, name)); err != nil {
			return err
		}
	}
	return nil
}

// Restore implements FR-09: validate the backup, stop the application,
// restore the database/files/configuration, restart, and run a health
// check.
func Restore(ctx context.Context, cfg *config.Config, client *dockercli.Client, backupDir string, out io.Writer) error {
	fmt.Fprintln(out, "==> Validating backup")
	manifest, err := ReadManifest(backupDir)
	if err != nil {
		return err
	}
	if err := manifest.Validate(backupDir); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	fmt.Fprintln(out, "==> Stopping the application")
	if err := client.ComposeStop(ctx); err != nil {
		return fmt.Errorf("stopping services: %w", err)
	}

	fmt.Fprintln(out, "==> Starting database for restore")
	if err := client.ComposeStart(ctx); err != nil {
		return fmt.Errorf("starting mariadb for restore: %w", err)
	}

	fmt.Fprintln(out, "==> Restoring database")
	dbFile, err := os.Open(filepath.Join(backupDir, databaseFileName))
	if err != nil {
		return err
	}
	restoreErr := client.ComposeExecIn(ctx, dbFile, cfg.MariaDBService(), "sh", "-c", mysqlRestoreEnv)
	dbFile.Close()
	if restoreErr != nil {
		return fmt.Errorf("restoring database: %w", restoreErr)
	}

	fmt.Fprintln(out, "==> Restoring Nextcloud configuration")
	if err := client.ComposeCp(ctx, filepath.Join(backupDir, configFileName), cfg.NextcloudService()+":/var/www/html/config/config.php"); err != nil {
		return fmt.Errorf("restoring nextcloud config: %w", err)
	}

	fmt.Fprintln(out, "==> Restoring user files")
	archiveFile, err := os.Open(filepath.Join(backupDir, filesArchiveName))
	if err != nil {
		return err
	}
	extractErr := client.ComposeExecIn(ctx, archiveFile, cfg.NextcloudService(), "tar", "-xzf", "-", "-C", "/var/www/html/data")
	archiveFile.Close()
	if extractErr != nil {
		return fmt.Errorf("restoring user files: %w", extractErr)
	}

	fmt.Fprintln(out, "==> Restarting services")
	if err := client.ComposeUp(ctx); err != nil {
		return fmt.Errorf("restarting services: %w", err)
	}

	fmt.Fprintln(out, "==> Running health check")
	if err := deployment.WaitHealthy(ctx, cfg, client, deployment.DefaultOptions().HealthTimeout, deployment.DefaultOptions().PollInterval); err != nil {
		return fmt.Errorf("post-restore health check failed: %w", err)
	}

	fmt.Fprintln(out, "\nRestore complete.")
	return nil
}

// List returns the names of every backup set under cfg.Backup.Dir, most
// recent first.
func List(backupRoot string) ([]string, error) {
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	return dirs, nil
}
