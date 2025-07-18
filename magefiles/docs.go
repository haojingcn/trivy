//go:build mage_docs

package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/aquasecurity/trivy/pkg/commands"
	"github.com/aquasecurity/trivy/pkg/flag"
	"github.com/aquasecurity/trivy/pkg/log"
	"github.com/spf13/cobra/doc"
)

const (
	title       = "Config file"
	description = "Trivy can be customized by tweaking a `trivy.yaml` file.\n" +
		"The config path can be overridden by the `--config` flag.\n\n" +
		"An example is [here][example].\n\n" +
		"These samples contain default values for flags."
	footer = "[example]: https://github.com/aquasecurity/trivy/tree/{{ git.tag }}/examples/trivy-conf/trivy.yaml"
)

// Generate CLI references
func main() {
	// Set a dummy path for the documents
	flag.CacheDirFlag.Default = "/path/to/cache"
	flag.ModuleDirFlag.Default = "$HOME/.trivy/modules"

	// Set a dummy path not to load plugins
	os.Setenv("XDG_DATA_HOME", os.TempDir())

	cmd := commands.NewApp()
	allFlagGroups := getAllFlags()

	cmd.DisableAutoGenTag = true
	if err := doc.GenMarkdownTree(cmd, "./docs/docs/references/configuration/cli"); err != nil {
		log.Fatal("Fatal error", log.Err(err))
	}
	if err := generateConfigDocs("./docs/docs/references/configuration/config-file.md", allFlagGroups); err != nil {
		log.Fatal("Fatal error in config file generation", log.Err(err))
	}
	if err := generateTelemetryFlagDocs("./docs/docs/advanced/telemetry-flags.md", allFlagGroups); err != nil {
		log.Fatal("Fatal error in telemetry docs generation", log.Err(err))
	}
}

// generateTelemetryFlagDocs updates the telemetry section in the documentation file
// with the flags that are safe to be included in telemetry.
func generateTelemetryFlagDocs(filename string, allFlagGroups []flag.FlagGroup) error {
	var telemetryFlags []string
	for _, group := range allFlagGroups {
		flags := group.Flags()
		for _, f := range flags {
			if f.IsTelemetrySafe() && f.GetConfigName() != "" {
				telemetryFlags = append(telemetryFlags, fmt.Sprintf("--%s", f.GetName()))
			}
		}
	}

	sort.Strings(telemetryFlags)
	flagContent := fmt.Sprintf("```\n%s\n```\n", strings.Join(telemetryFlags, "\n"))
	if err := os.WriteFile(filename, []byte(flagContent), 0644); err != nil {
		return fmt.Errorf("failed to write to %s: %w", filename, err)
	}
	return nil
}

// generateConfigDocs creates markdown file for Trivy config.
func generateConfigDocs(filename string, allFlagGroups []flag.FlagGroup) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString("# " + title + "\n\n")
	f.WriteString(description + "\n")

	if len(allFlagGroups) == 0 {
		return fmt.Errorf("no flag groups found")
	}

	for _, group := range allFlagGroups {
		f.WriteString("## " + group.Name() + " options\n")
		writeFlags(group, f)
	}

	f.WriteString(footer)
	return nil
}

func writeFlags(group flag.FlagGroup, w *os.File) {
	flags := group.Flags()
	// Sort flags to avoid duplicates of non-last parts of config file
	slices.SortFunc(flags, func(a, b flag.Flagger) int {
		return cmp.Compare(a.GetConfigName(), b.GetConfigName())
	})
	w.WriteString("\n```yaml\n")

	var lastParts []string
	for _, flg := range flags {
		if flg.GetConfigName() == "" || flg.Hidden() {
			continue
		}
		// We need to split the config name on `.` to make the indentations needed in yaml.
		parts := strings.Split(flg.GetConfigName(), ".")
		for i := range parts {
			// Skip already added part
			if len(lastParts) >= i+1 && parts[i] == lastParts[i] {
				continue
			}
			ind := strings.Repeat("  ", i)
			// We need to add a comment and example values only for the last part of the config name.
			isLastPart := i == len(parts)-1
			if isLastPart {
				// Some `Flags` don't support flag for CLI. (e.g.`LicenseForbidden`).
				if flg.GetName() != "" {
					fmt.Fprintf(w, "%s# Same as '--%s'\n", ind, flg.GetName())
				}
			}
			w.WriteString(ind + parts[i] + ":")
			if isLastPart {
				writeFlagValue(flg.GetDefaultValue(), ind, w)
			}
			w.WriteString("\n")
		}
		lastParts = parts
	}
	w.WriteString("```\n")
}

func writeFlagValue(val any, ind string, w *os.File) {
	switch v := val.(type) {
	case []string:
		if len(v) > 0 {
			w.WriteString("\n")
			for _, vv := range v {
				fmt.Fprintf(w, "%s - %s\n", ind, vv)
			}
		} else {
			w.WriteString(" []\n")
		}
	case map[string][]string:
		w.WriteString("\n")
		for k, vv := range v {
			fmt.Fprintf(w, "%s  %s:\n", ind, k)
			for _, vvv := range vv {
				fmt.Fprintf(w, "  %s - %s\n", ind, vvv)
			}
		}
	case string:
		fmt.Fprintf(w, " %q\n", v)
	default:
		fmt.Fprintf(w, " %v\n", v)
	}
}

func getAllFlags() []flag.FlagGroup {
	// remoteFlags should contain Client and Server flags.
	// NewClientFlags doesn't initialize `Listen` field
	remoteFlags := flag.NewClientFlags()
	remoteFlags.Listen = flag.ServerListenFlag.Clone()

	// These flags don't work from config file.
	// Clear configName to skip them later.
	globalFlags := flag.NewGlobalFlagGroup()
	globalFlags.ConfigFile.ConfigName = ""
	globalFlags.ShowVersion.ConfigName = ""
	globalFlags.GenerateDefaultConfig.ConfigName = ""

	return []flag.FlagGroup{
		globalFlags,
		flag.NewCacheFlagGroup(),
		flag.NewCleanFlagGroup(),
		remoteFlags,
		flag.NewDBFlagGroup(),
		flag.NewImageFlagGroup(),
		flag.NewK8sFlagGroup(),
		flag.NewLicenseFlagGroup(),
		flag.NewMisconfFlagGroup(),
		flag.NewModuleFlagGroup(),
		flag.NewPackageFlagGroup(),
		flag.NewRegistryFlagGroup(),
		flag.NewRegoFlagGroup(),
		flag.NewReportFlagGroup(),
		flag.NewRepoFlagGroup(),
		flag.NewScanFlagGroup(),
		flag.NewSecretFlagGroup(),
		flag.NewVulnerabilityFlagGroup(),
	}

}
