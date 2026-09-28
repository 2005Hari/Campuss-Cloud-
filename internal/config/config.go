// Package config loads and validates CampusCloud's YAML configuration
// (FR-10) together with environment-variable overrides supplied via .env
// or the real process environment.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Thresholds holds the resource-usage percentage boundaries described in
// PRD section 13 (Monitoring & Resource Thresholds).
type Thresholds struct {
	Warning  float64 `yaml:"warning"`
	High     float64 `yaml:"high"`
	Critical float64 `yaml:"critical"`
}

// Config is the full effective configuration for the campuscloud CLI.
type Config struct {
	Project struct {
		Name string `yaml:"name"`
	} `yaml:"project"`

	Network struct {
		Name string `yaml:"name"`
	} `yaml:"network"`

	Volumes struct {
		NextcloudData string `yaml:"nextcloud_data"`
		MariaDBData   string `yaml:"mariadb_data"`
	} `yaml:"volumes"`

	Compose struct {
		File string `yaml:"file"`
	} `yaml:"compose"`

	Nextcloud struct {
		ServiceName   string `yaml:"service_name"`
		ContainerName string `yaml:"container_name"`
		HTTPPort      int    `yaml:"http_port"`
		HealthPath    string `yaml:"health_path"`
	} `yaml:"nextcloud"`

	MariaDB struct {
		ServiceName   string `yaml:"service_name"`
		ContainerName string `yaml:"container_name"`
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
	} `yaml:"mariadb"`

	Monitoring struct {
		IntervalSeconds int        `yaml:"interval_seconds"`
		Thresholds      Thresholds `yaml:"thresholds"`
	} `yaml:"monitoring"`

	Backup struct {
		Dir    string `yaml:"dir"`
		Retain int    `yaml:"retain"`
	} `yaml:"backup"`

	// Credentials, deliberately not read from YAML (Security Requirements,
	// PRD section 18): populated only from the environment / .env file.
	DB struct {
		RootPassword string
		Database     string
		User         string
		Password     string
	} `yaml:"-"`

	AdminUser     string `yaml:"-"`
	AdminPassword string `yaml:"-"`
}

// Default returns the built-in defaults used when no config.yaml is found
// or when a field is left unset.
func Default() *Config {
	c := &Config{}
	c.Project.Name = "campuscloud"
	c.Network.Name = "campuscloud_net"
	c.Volumes.NextcloudData = "campuscloud_nextcloud_data"
	c.Volumes.MariaDBData = "campuscloud_mariadb_data"
	c.Compose.File = "docker/docker-compose.yml"
	c.Nextcloud.ServiceName = "nextcloud"
	c.Nextcloud.ContainerName = "campuscloud-nextcloud"
	c.Nextcloud.HTTPPort = 8080
	c.Nextcloud.HealthPath = "/status.php"
	c.MariaDB.ServiceName = "mariadb"
	c.MariaDB.ContainerName = "campuscloud-mariadb"
	c.MariaDB.Host = "mariadb"
	c.MariaDB.Port = 3306
	c.Monitoring.IntervalSeconds = 30
	c.Monitoring.Thresholds = Thresholds{Warning: 70, High: 80, Critical: 90}
	c.Backup.Dir = "./backups"
	c.Backup.Retain = 5
	return c
}

// Load reads configPath (falling back to defaults for any field the file
// omits), loads envPath into the process environment if present, then
// applies environment-variable overrides on top.
func Load(configPath, envPath string) (*Config, error) {
	cfg := Default()

	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading %s: %w", configPath, err)
		}
	}

	if envPath != "" {
		if err := LoadEnvFile(envPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading %s: %w", envPath, err)
		}
	}

	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides layers environment variables on top of YAML-sourced
// values, per FR-10 ("Use YAML configuration and environment variables").
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("CAMPUSCLOUD_HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Nextcloud.HTTPPort = p
		}
	}
	if v := os.Getenv("CAMPUSCLOUD_BACKUP_DIR"); v != "" {
		c.Backup.Dir = v
	}

	c.DB.RootPassword = os.Getenv("MYSQL_ROOT_PASSWORD")
	c.DB.Database = envOr("MYSQL_DATABASE", "nextcloud")
	c.DB.User = envOr("MYSQL_USER", "nextcloud")
	c.DB.Password = os.Getenv("MYSQL_PASSWORD")

	c.AdminUser = envOr("NEXTCLOUD_ADMIN_USER", "admin")
	c.AdminPassword = os.Getenv("NEXTCLOUD_ADMIN_PASSWORD")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// NextcloudService returns the docker-compose service name for Nextcloud
// (as opposed to ContainerName, which is the running container's name —
// `docker compose exec`/`cp` take the former, `docker inspect` the latter).
func (c *Config) NextcloudService() string { return c.Nextcloud.ServiceName }

// MariaDBService returns the docker-compose service name for MariaDB.
func (c *Config) MariaDBService() string { return c.MariaDB.ServiceName }

// Validate checks that fields required for a real deployment are present.
// It is intentionally strict about secrets (Security Requirements) but
// lenient about structural fields, which always carry defaults.
func (c *Config) Validate() error {
	var missing []string
	if c.DB.RootPassword == "" {
		missing = append(missing, "MYSQL_ROOT_PASSWORD")
	}
	if c.DB.Password == "" {
		missing = append(missing, "MYSQL_PASSWORD")
	}
	if c.AdminPassword == "" {
		missing = append(missing, "NEXTCLOUD_ADMIN_PASSWORD")
	}
	if c.Nextcloud.HTTPPort <= 0 || c.Nextcloud.HTTPPort > 65535 {
		return fmt.Errorf("invalid nextcloud.http_port: %d", c.Nextcloud.HTTPPort)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}

// LoadEnvFile parses a simple KEY=VALUE .env file and sets each variable in
// the process environment, without overwriting a variable that is already
// set (so real environment variables always take precedence over the file).
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
