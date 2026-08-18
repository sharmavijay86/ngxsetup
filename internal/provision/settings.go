package provision

import (
	"fmt"
	"strconv"
	"strings"

	"ngxsetup/internal/tuning"
)

// RequireSetup reports an error unless the stack is installed — either by
// this tool's own `setup`, or adopted from an existing install (PHP and a
// database flavour were both detected even though SetupCompleted was never
// set). Shared by the CLI's site/tune commands and the web UI's equivalents.
func RequireSetup(c *Ctx) error {
	if c.State.SetupCompleted {
		return nil
	}
	if c.PHPVersion != "" && c.Facts.DBFlavor != "" {
		return nil
	}
	return fmt.Errorf("the stack is not installed yet; run `ngxsetup setup` first")
}

// ConfigRows renders the settings an operator can read or change. Keeping the
// list explicit — rather than reflecting over the struct — means every
// settable key has a documented meaning and a validated setter. Shared by the
// CLI (`config get|set|show`) and the web UI's settings page, so the two
// front ends can never drift into showing a different set of keys.
func ConfigRows(c *Ctx) [][2]string {
	cfg := c.Config
	return [][2]string{
		{"profile", cfg.Profile},
		{"timezone", cfg.Timezone},
		{"acme_email", cfg.ACMEEmail},
		{"trusted_networks", strings.Join(cfg.TrustedNetworks, ",")},
		{"admin_allow_list", strings.Join(cfg.AdminAllowList, ",")},
		{"block_xmlrpc", strconv.FormatBool(cfg.BlockXMLRPC)},
		{"block_bad_agents", strconv.FormatBool(cfg.BlockBadAgents)},
		{"block_bad_referrers", strconv.FormatBool(cfg.BlockBadReferrers)},
		{"block_scraper_bots", strconv.FormatBool(cfg.BlockScraperBots)},
		{"cache_vary_device", strconv.FormatBool(cfg.CacheVaryDevice)},
		{"strict_php_functions", strconv.FormatBool(cfg.StrictPHPFunctions)},
		{"upload_max_mb", strconv.Itoa(cfg.UploadMaxMB)},
		{"avg_php_worker_mb", strconv.Itoa(cfg.AvgPHPWorkerMB)},
		{"reserve_mb", strconv.Itoa(cfg.ReserveMB)},
		{"enable_binlog", strconv.FormatBool(cfg.EnableBinlog)},
		{"aggressive_opcache", strconv.FormatBool(cfg.AggressiveOpcache)},
		{"trust_cloudflare", strconv.FormatBool(cfg.TrustCloudflare)},
		{"log_keep_days", strconv.Itoa(cfg.LogKeepDays)},
		{"phpmyadmin.enabled", strconv.FormatBool(cfg.PhpMyAdmin.Enabled)},
		{"phpmyadmin.port", strconv.Itoa(cfg.PhpMyAdmin.Port)},
		{"phpmyadmin.allow_list", strings.Join(cfg.PhpMyAdmin.AllowList, ",")},
		{"security_yara_rules_dir", cfg.SecurityYARARulesDir},
		{"geoip_database_path", cfg.GeoIPDatabasePath},
		{"borg.enabled", strconv.FormatBool(cfg.Borg.Enabled)},
		{"borg.repo", cfg.Borg.Repo},
		{"borg.encryption", cfg.Borg.Encryption},
		{"borg.compression", cfg.Borg.Compression},
		{"borg.schedule", cfg.Borg.Schedule},
		{"borg.keep_daily", strconv.Itoa(cfg.Borg.KeepDaily)},
		{"borg.keep_weekly", strconv.Itoa(cfg.Borg.KeepWeekly)},
		{"borg.keep_monthly", strconv.Itoa(cfg.Borg.KeepMonthly)},
	}
}

// SetConfigKey applies a change and validates the whole configuration
// afterwards, so an invalid combination is rejected before it is written.
func SetConfigKey(c *Ctx, key, value string) error {
	cfg := c.Config

	parseBool := func() (bool, error) {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("%s expects true or false, got %q", key, value)
		}
		return b, nil
	}
	parseInt := func() (int, error) {
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%s expects a number, got %q", key, value)
		}
		return n, nil
	}
	splitList := func(s string) []string {
		if s == "" {
			return nil
		}
		var out []string
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}

	var err error
	switch key {
	case "profile":
		p, perr := tuning.ParseProfile(value)
		if perr != nil {
			return perr
		}
		cfg.Profile = string(p)
	case "timezone":
		cfg.Timezone = value
	case "acme_email":
		cfg.ACMEEmail = value
	case "trusted_networks":
		cfg.TrustedNetworks = splitList(value)
	case "admin_allow_list":
		cfg.AdminAllowList = splitList(value)
	case "block_xmlrpc":
		cfg.BlockXMLRPC, err = parseBool()
	case "block_bad_agents":
		cfg.BlockBadAgents, err = parseBool()
	case "block_bad_referrers":
		cfg.BlockBadReferrers, err = parseBool()
	case "block_scraper_bots":
		cfg.BlockScraperBots, err = parseBool()
	case "cache_vary_device":
		cfg.CacheVaryDevice, err = parseBool()
	case "strict_php_functions":
		cfg.StrictPHPFunctions, err = parseBool()
	case "upload_max_mb":
		cfg.UploadMaxMB, err = parseInt()
	case "avg_php_worker_mb":
		cfg.AvgPHPWorkerMB, err = parseInt()
	case "reserve_mb":
		cfg.ReserveMB, err = parseInt()
	case "enable_binlog":
		cfg.EnableBinlog, err = parseBool()
	case "aggressive_opcache":
		cfg.AggressiveOpcache, err = parseBool()
	case "trust_cloudflare":
		cfg.TrustCloudflare, err = parseBool()
	case "log_keep_days":
		cfg.LogKeepDays, err = parseInt()
	case "phpmyadmin.enabled":
		cfg.PhpMyAdmin.Enabled, err = parseBool()
	case "phpmyadmin.port":
		cfg.PhpMyAdmin.Port, err = parseInt()
	case "phpmyadmin.allow_list":
		cfg.PhpMyAdmin.AllowList = splitList(value)
	case "security_yara_rules_dir":
		cfg.SecurityYARARulesDir = value
	case "geoip_database_path":
		cfg.GeoIPDatabasePath = value
	case "borg.enabled":
		cfg.Borg.Enabled, err = parseBool()
	case "borg.repo":
		cfg.Borg.Repo = value
	case "borg.encryption":
		cfg.Borg.Encryption = value
	case "borg.compression":
		cfg.Borg.Compression = value
	case "borg.keep_daily":
		cfg.Borg.KeepDaily, err = parseInt()
	case "borg.keep_weekly":
		cfg.Borg.KeepWeekly, err = parseInt()
	case "borg.keep_monthly":
		cfg.Borg.KeepMonthly, err = parseInt()
	default:
		return fmt.Errorf("unknown key %q (see `ngxsetup config show`)", key)
	}
	if err != nil {
		return err
	}
	return cfg.Validate()
}
