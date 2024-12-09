package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

const (
	defaultAgentVariableName = "NewRelicAgent"
	defaultPackageName       = "./..."
	defaultPackagePath       = ""
	defaultAppName           = ""
	defaultDiffFileName      = "new-relic-instrumentation.diff"
	defaultDebug             = false
)

var (
	debug             bool
	agentVariableName string
	packagePath       string
	appName           string
	diffFile          string
)

var instrumentCmd = &cobra.Command{
	Use:   "instrument",
	Short: "add instrumentation",
	Long:  "add instrumentation to existing application source files",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		Instrument()
	},
}

func Instrument() {
	if packagePath == "" {
		log.Fatal("--path is required")
	}

	if _, err := os.Stat(packagePath); err != nil {
		log.Fatalf("--path \"%s\" is invalid: %v", packagePath, err)
	}

	if debug {
		comment.EnableConsolePrinter()
	}

	pkgs, err := decorator.Load(&packages.Config{Dir: packagePath, Mode: packages.LoadSyntax}, defaultPackageName)
	if err != nil {
		log.Fatal(err)
	}

	manager := parser.NewInstrumentationManager(pkgs, appName, agentVariableName, diffFile, packagePath)
	err = manager.CreateDiffFile()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.DetectDependencyIntegrations()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.InstrumentApplication()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.AddRequiredModules()
	if err != nil {
		log.Fatal(err)
	}

	err = manager.WriteDiff()
	if err != nil {
		log.Fatal(err)
	}

	comment.WriteAll()
}

func init() {
	// base the default path to the diff file on the current working directory
	wd, _ := os.Getwd()
	relativePath, err := filepath.Rel(wd, filepath.Join(wd, defaultDiffFileName))
	if err != nil {
		relativePath = defaultDiffFileName
	}

	instrumentCmd.Flags().BoolVar(&debug, "debug", defaultDebug, "enable debugging output")
	instrumentCmd.Flags().StringVar(&agentVariableName, "agent", defaultAgentVariableName, "set agent application variable name")
	instrumentCmd.Flags().StringVar(&packagePath, "path", defaultPackagePath, "specify package path")
	instrumentCmd.Flags().StringVar(&appName, "name", defaultAppName, "set application name for telemetry reporting")
	instrumentCmd.Flags().StringVar(&diffFile, "diff", relativePath, "specify diff output file path")
	rootCmd.AddCommand(instrumentCmd)
}
