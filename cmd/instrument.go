package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"golang.org/x/tools/go/packages"
)

const (
	defaultAgentVariableName = "NewRelicAgent"
	defaultPackageName       = "./..."
	defaultPackagePath       = ""
	defaultAppName           = ""
	defaultOutputFilePath    = ""
	defaultDiffFileName      = "new-relic-instrumentation.diff"
)

var (
	diffFile    string
	excludeDirs string
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

// Bubble Tea Model
type model struct {
	spinner     spinner.Model
	stepDesc    string
	totalSteps  int
	currentStep int
	done        bool
	err         error
	packages    []*decorator.Package
	pkgPath     string
	sub         chan tea.Msg
	outputFile  string
}

// Messages
// Messages
type progressMsg struct {
	desc string
}
type pkgLoadedMsg []*decorator.Package
type errMsg error
type completedMsg struct{}

func initialModel(pkgPath, outputFile string) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#1CE783"))
	return model{
		spinner:    s,
		stepDesc:   "Loading packages...",
		totalSteps: 8,
		pkgPath:    pkgPath,
		outputFile: outputFile,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForNext(m.sub),
	)
}

func Instrument(packagePath string, patterns ...string) {
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

	// If debug mode is enabled or no terminal is available (CI/CD), run in text mode (no TUI)
	if debug || !term.IsTerminal(int(os.Stdout.Fd())) {
		runTextMode(packagePath, patterns, outputFile)
		return
	}

	// Normal TUI mode
	runTUIMode(packagePath, patterns, outputFile)
}

// runTextMode runs the instrumentation pipeline with plain text output to stdout.
// This is used when the TUI is unavailable (e.g. CI/CD, piped output) or when
// the --debug flag is enabled. It delegates to instrumentPackages for the core
// logic and handles printing status and exit on error.
func runTextMode(packagePath string, patterns []string, outputFile string) {
	fmt.Printf("Instrumentation started for %s\n", packagePath)
	fmt.Printf("Output file: %s\n\n", outputFile)

	if err := instrumentPackages(packagePath, patterns, outputFile); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone! Changes written to: %s\nTip: Apply these changes with: git apply %s\n", outputFile, outputFile)
}

// instrumentPackages loads Go packages from packagePath, runs the full
// instrumentation pipeline, and writes the resulting diff to outputFile.
// It returns an error rather than exiting the process, making it safe to
// call from both runTextMode (which handles os.Exit) and from tests.
func instrumentPackages(packagePath string, patterns []string, outputFile string) error {
	loadPatterns := patterns
	if len(loadPatterns) == 0 {
		loadPatterns = []string{defaultPackageName}
	}

	fmt.Println(" -> Loading packages...")
	pkgs, err := decorator.Load(&packages.Config{Dir: packagePath, Mode: LoadMode, Tests: true}, loadPatterns...)
	if err != nil {
		return fmt.Errorf("loading packages: %w", err)
	}

	manager := parser.NewInstrumentationManager(pkgs, defaultAppName, defaultAgentVariableName, outputFile, packagePath)

	steps := []struct {
		desc string
		fn   func() error
	}{
		{"Creating diff file", manager.CreateDiffFile},
		{"Detecting dependencies", manager.DetectDependencyIntegrations},
		{"Tracing package calls", manager.TracePackageCalls},
		{"Scanning application", manager.ScanApplication},
		{"Instrumenting application", manager.InstrumentApplication},
		{"Resolving unit tests", manager.ResolveUnitTests},
		{"Adding required modules", manager.AddRequiredModules},
		{"Writing diff file", func() error {
			comment.WriteAll()
			return manager.WriteDiff(func(msg string) {})
		}},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s: %w", step.desc, err)
		}
	}

	return nil
}

func runTUIMode(packagePath string, patterns []string, outputFile string) {
	// Channel to receive updates from the worker
	updates := make(chan tea.Msg)

	// Worker goroutine
	go func() {
		loadPatterns := patterns
		if len(loadPatterns) == 0 {
			loadPatterns = []string{defaultPackageName}
		}

		pkgs, err := decorator.Load(&packages.Config{Dir: packagePath, Mode: LoadMode, Tests: true}, loadPatterns...)
		if err != nil {
			updates <- errMsg(err)
			return
		}

		updates <- pkgLoadedMsg(pkgs)

		manager := parser.NewInstrumentationManager(pkgs, defaultAppName, defaultAgentVariableName, outputFile, packagePath)

		steps := []struct {
			desc string
			fn   func() error
		}{
			{"Creating diff file", manager.CreateDiffFile},
			{"Detecting dependencies", manager.DetectDependencyIntegrations},
			{"Tracing package calls", manager.TracePackageCalls},
			{"Scanning application", manager.ScanApplication},
			{"Instrumenting application", manager.InstrumentApplication},
			{"Resolving unit tests", manager.ResolveUnitTests},
			{"Adding required modules", manager.AddRequiredModules},
			{"Writing diff file", func() error {
				comment.WriteAll()
				// Pass a callback to WriteDiff to receive granular progress updates.
				// This callback updates the UI with the name of the file currently being written,
				// avoiding a "stalled" UI during this potentially long-running step.
				return manager.WriteDiff(func(msg string) {
					updates <- progressMsg{desc: msg}
				})
			}},
		}

		for _, step := range steps {
			updates <- progressMsg{desc: step.desc}
			if err := step.fn(); err != nil {
				updates <- errMsg(err)
				return
			}
		}

		updates <- completedMsg{}
		close(updates)
	}()

	initialM := initialModel(packagePath, outputFile)
	initialM.sub = updates

	finalModel, err := tea.NewProgram(initialM).Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(model); ok {
		if m.err != nil {
			os.Exit(1)
		}
		if m.done {
			fmt.Printf("\nDone! Changes written to: %s\nTip: Apply these changes with: git apply %s\n", m.outputFile, m.outputFile)
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case pkgLoadedMsg:
		m.packages = msg
		m.stepDesc = "Starting instrumentation..."
		return m, waitForNext(m.sub)
	case progressMsg:
		m.currentStep++
		m.stepDesc = msg.desc
		// We just update the description, no progress bar to update
		return m, waitForNext(m.sub)
	case errMsg:
		m.err = msg
		return m, tea.Quit
	case completedMsg:
		m.done = true
		return m, tea.Quit
	}

	// If we are strictly in Init, we should return the initial batch.
	// But Update is called for every message.
	// If the message is none of the above (shouldn't happen usually), we return nil.

	// Wait, we need to ensure the first waitForNext is called.
	// We can do it in a special "start" message or just include it in Init.
	return m, nil
}

func waitForNext(sub chan tea.Msg) tea.Cmd {
	if sub == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-sub
		if !ok {
			return nil
		}
		return msg
	}
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}

	pad := strings.Repeat(" ", padding(m.stepDesc, 30))

	// Simple spinner view for all phases
	return fmt.Sprintf("\n %s %s%s\n\n", m.spinner.View(), m.stepDesc, pad)
}

func padding(s string, width int) int {
	l := len(s)
	if l > width {
		return 0
	}
	return width - l
}

func init() {
	instrumentCmd.Flags().StringVarP(&diffFile, "output", "o", defaultOutputFilePath, "specify diff output file path")
	instrumentCmd.Flags().StringVarP(&excludeDirs, "exclude", "e", "", "comma-separated list of folders to exclude from instrumentation")
	cobra.MarkFlagFilename(instrumentCmd.Flags(), "output", ".diff") // for file completion

	rootCmd.AddCommand(instrumentCmd)
}
