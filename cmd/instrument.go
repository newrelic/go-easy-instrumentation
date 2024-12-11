package cmd

import (
	"errors"
	"fmt"
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
	defaultOutputFilePath    = ""
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

// validateOutputFile checks that the custom output path is valid
func validateOutputFile(path string) error {
	if filepath.Ext(path) != ".diff" {
		return errors.New("output file must have a .diff extension")
	}

	_, err := os.Stat(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("output file directory does not exist: %v", err)
	}

	return nil
}

// setOutputFilePath returns a complete output file path based on the provided
// diffFile flag value. If the flag is empty, the default path will be based
// on the applicationPath.
//
// This will fail if the packagePath is not valid, and must be run after
// validateing it.
func setOutputFilePath(outputFilePath, applicationPath string) (string, error) {
	if outputFilePath == "" {
		outputFilePath = filepath.Join(applicationPath, defaultDiffFileName)
	}

	err := validateOutputFile(outputFilePath)
	if err != nil {
		return "", err
	}

	return outputFilePath, nil
}

func Instrument() {
	if packagePath == "" {
		log.Fatal("--path is required")
	}

	if _, err := os.Stat(packagePath); err != nil {
		cobra.CheckErr(fmt.Errorf("--path \"%s\" is invalid: %v", packagePath, err))
	}

	outputFile, err := setOutputFilePath(diffFile, packagePath)
	if err != nil {
		cobra.CheckErr(err)
	}

	if debug {
		comment.EnableConsolePrinter()
	}

	pkgs, err := decorator.Load(&packages.Config{Dir: packagePath, Mode: packages.LoadSyntax}, defaultPackageName)
	if err != nil {
		log.Fatal(err)
	}

	manager := parser.NewInstrumentationManager(pkgs, appName, agentVariableName, outputFile, packagePath)
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
	instrumentCmd.Flags().BoolVar(&debug, "debug", defaultDebug, "enable debugging output")
	instrumentCmd.Flags().StringVar(&agentVariableName, "agent", defaultAgentVariableName, "set agent application variable name")
	instrumentCmd.Flags().StringVar(&packagePath, "path", defaultPackagePath, "specify package path")
	instrumentCmd.Flags().StringVar(&appName, "name", defaultAppName, "set application name for telemetry reporting")
	instrumentCmd.Flags().StringVar(&diffFile, "diff", defaultOutputFilePath, "specify diff output file path")
	cobra.MarkFlagFilename(instrumentCmd.Flags(), "diff", ".diff") // for file completion

	rootCmd.AddCommand(instrumentCmd)
}
