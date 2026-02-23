package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func runInteractiveMode(cmd *cobra.Command, args []string) {
	// Parse exclusions from the --exclude flag if provided
	var exclusions []string
	if excludeDirs != "" {
		for _, dir := range strings.Split(excludeDirs, ",") {
			trimmed := strings.TrimSpace(dir)
			if trimmed != "" {
				exclusions = append(exclusions, trimmed)
			}
		}
	}

	files, err := scanGoFiles(".", exclusions)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to scan for Go files: %v", err))
	}

	if len(files) == 0 {
		fmt.Println("No Go files found in the current directory.")
		return
	}

	printFiles(files)

	if promptUser("Do you want to run instrumentation on these files?") {
		// Pass the detected files as patterns to Instrument
		Instrument(".", files...)
	} else {
		fmt.Println("Aborting.")
	}
}

func printFiles(files []string) {
	fmt.Println("Detected Go files:")
	for _, file := range files {
		fmt.Printf(" - %s\n", file)
	}
	fmt.Println()
}

func scanGoFiles(root string, excludedDirs []string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check exclusion
		if info.IsDir() {
			base := info.Name()
			for _, excluded := range excludedDirs {
				// Simple check: if directory name matches excluded name exacty
				// Or if path contains it? User asked to "exclude folders like end-to-end-tests".
				// Let's do a strict component match to be safe, or just check if it matches the name.
				if base == excluded {
					return filepath.SkipDir
				}
			}
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func promptUser(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", question)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
