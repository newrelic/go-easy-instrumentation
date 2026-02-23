package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanGoFiles(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	dirs := []string{
		"pkg",
		"vendor",
		"internal",
		"testdata",
		"nested/deep",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Create Go files in various directories
	goFiles := []string{
		"main.go",
		"pkg/service.go",
		"vendor/dep.go",
		"internal/util.go",
		"testdata/fixture.go",
		"nested/deep/handler.go",
	}
	for _, f := range goFiles {
		path := filepath.Join(tmpDir, f)
		if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	// Also create a non-Go file to ensure it's ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# readme\n"), 0644); err != nil {
		t.Fatalf("failed to create README.md: %v", err)
	}

	t.Run("no exclusions returns all Go files", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != len(goFiles) {
			t.Errorf("expected %d files, got %d: %v", len(goFiles), len(files), files)
		}
		// Ensure no non-Go files are included
		for _, f := range files {
			if !strings.HasSuffix(f, ".go") {
				t.Errorf("non-Go file included: %s", f)
			}
		}
	})

	t.Run("exclude single directory", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, []string{"vendor"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, f := range files {
			if strings.Contains(f, "vendor") {
				t.Errorf("vendor file should have been excluded: %s", f)
			}
		}
		// Should have all files except vendor/dep.go
		if len(files) != len(goFiles)-1 {
			t.Errorf("expected %d files, got %d: %v", len(goFiles)-1, len(files), files)
		}
	})

	t.Run("exclude multiple directories", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, []string{"vendor", "testdata"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, f := range files {
			if strings.Contains(f, "vendor") || strings.Contains(f, "testdata") {
				t.Errorf("excluded file found: %s", f)
			}
		}
		if len(files) != len(goFiles)-2 {
			t.Errorf("expected %d files, got %d: %v", len(goFiles)-2, len(files), files)
		}
	})

	t.Run("exclude nested directory by name", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, []string{"deep"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, f := range files {
			if strings.Contains(f, "deep") {
				t.Errorf("deep directory file should have been excluded: %s", f)
			}
		}
		if len(files) != len(goFiles)-1 {
			t.Errorf("expected %d files, got %d: %v", len(goFiles)-1, len(files), files)
		}
	})

	t.Run("exclude nonexistent directory is a no-op", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, []string{"doesnotexist"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != len(goFiles) {
			t.Errorf("expected %d files, got %d", len(goFiles), len(files))
		}
	})

	t.Run("empty exclusion list same as nil", func(t *testing.T) {
		files, err := scanGoFiles(tmpDir, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != len(goFiles) {
			t.Errorf("expected %d files, got %d", len(goFiles), len(files))
		}
	})
}

func TestParseExcludeDirs(t *testing.T) {
	// This tests the parsing logic used in runInteractiveMode
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single folder",
			input:    "vendor",
			expected: []string{"vendor"},
		},
		{
			name:     "multiple folders",
			input:    "vendor,testdata,build",
			expected: []string{"vendor", "testdata", "build"},
		},
		{
			name:     "with whitespace",
			input:    " vendor , testdata , build ",
			expected: []string{"vendor", "testdata", "build"},
		},
		{
			name:     "trailing comma",
			input:    "vendor,testdata,",
			expected: []string{"vendor", "testdata"},
		},
		{
			name:     "only commas",
			input:    ",,",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []string
			if tt.input != "" {
				for _, dir := range strings.Split(tt.input, ",") {
					trimmed := strings.TrimSpace(dir)
					if trimmed != "" {
						result = append(result, trimmed)
					}
				}
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d exclusions, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("exclusion[%d]: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}
