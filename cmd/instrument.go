package cmd

import (
	"errors"
	"fmt"
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
	debug    bool
	diffFile string
)

var instrumentCmd = &cobra.Command{
	Use:   "instrument <path>",
	Short: "add instrumentation",
	Long:  "add instrumentation to an application's source files and write these changes to a diff file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		Instrument(args[0])
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
// validating it.
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

const LoadMode = packages.LoadSyntax | packages.NeedForTest

func Instrument(packagePath string) {
	if packagePath == "" {
		cobra.CheckErr("path argument cannot be empty")
	}

	if _, err := os.Stat(packagePath); err != nil {
		cobra.CheckErr(fmt.Errorf("path argument \"%s\" is invalid: %v", packagePath, err))
	}

	outputFile, err := setOutputFilePath(diffFile, packagePath)
	cobra.CheckErr(err)
	if debug {
		comment.EnableConsolePrinter(packagePath)
	}

	pkgs, err := decorator.Load(&packages.Config{Dir: packagePath, Mode: LoadMode, Tests: true}, defaultPackageName)
	cobra.CheckErr(err)

	manager := parser.NewInstrumentationManager(pkgs, defaultAppName, defaultAgentVariableName, outputFile, packagePath)
	err = manager.CreateDiffFile()
	cobra.CheckErr(err)

	err = manager.DetectDependencyIntegrations()
	cobra.CheckErr(err)

	err = manager.InstrumentApplication()
	cobra.CheckErr(err)

	err = manager.ResolveUnitTests()
	cobra.CheckErr(err)

	err = manager.AddRequiredModules()
	cobra.CheckErr(err)

	// write debug comments before writing diff so that
	// diff file console log is still easy to see
	comment.WriteAll()

	err = manager.WriteDiff()
	cobra.CheckErr(err)
}

func init() {
	instrumentCmd.Flags().BoolVarP(&debug, "debug", "D", defaultDebug, "enable debugging output")
	instrumentCmd.Flags().StringVarP(&diffFile, "output", "o", defaultOutputFilePath, "specify diff output file path")
	cobra.MarkFlagFilename(instrumentCmd.Flags(), "output", ".diff") // for file completion

	rootCmd.AddCommand(instrumentCmd)
}
