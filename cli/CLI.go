package cli

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Default Config Values
const (
	defaultAgentVariableName = "NewRelicAgent"
	defaultPackageName       = "./..."
	defaultPackagePath       = ""
	defaultAppName           = ""
	defaultDiffFileName      = "new-relic-instrumentation.diff"
)

type CLIConfig struct {
	PackagePath       string
	PackageName       string
	AppName           string
	AgentVariableName string
	DiffFile          string
}

func setConfigValue(input *string, defaultValue string) string {
	if input != nil && *input != "" {
		return strings.TrimSpace(*input)
	}
	return defaultValue
}

func NewCLIConfig() *CLIConfig {
	wd, _ := os.Getwd()
	diffFile := filepath.Join(wd, defaultDiffFileName)
	relativePath, _ := filepath.Rel(wd, diffFile)

	cfg := &CLIConfig{
		PackageName: defaultPackageName, // dont touch this
	}

	var pathFlag = flag.String("path", defaultPackagePath, "path to package to instrument")
	var appNameFlag = flag.String("name", defaultAppName, "configure the New Relic application name")
	var diffFlag = flag.String("diff", relativePath, "output diff file path name")
	var agentFlag = flag.String("agent", defaultAgentVariableName, "application variable for New Relic agent")
	flag.Parse()

	cfg.PackagePath = setConfigValue(pathFlag, defaultPackagePath)
	cfg.AppName = setConfigValue(appNameFlag, defaultAppName)
	cfg.DiffFile = setConfigValue(diffFlag, diffFile)
	cfg.AgentVariableName = setConfigValue(agentFlag, defaultAgentVariableName)

	cfg.Validate()
	return cfg
}

func (cfg *CLIConfig) Validate() {
	if cfg.PackagePath == "" {
		log.Fatal("path flag is required")
	}
	_, err := os.Stat(cfg.PackagePath)
	if err != nil {
		log.Fatal(err)
	}
}
