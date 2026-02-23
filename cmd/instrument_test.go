package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestValidateOutputFile(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid diff file in existing directory",
			path:    filepath.Join(os.TempDir(), "output.diff"),
			wantErr: false,
		},
		{
			name:    "wrong extension",
			path:    filepath.Join(os.TempDir(), "output.txt"),
			wantErr: true,
			errMsg:  "output file must have a .diff extension",
		},
		{
			name:    "no extension",
			path:    filepath.Join(os.TempDir(), "output"),
			wantErr: true,
			errMsg:  "output file must have a .diff extension",
		},
		{
			name:    "nonexistent directory",
			path:    "/nonexistent/directory/output.diff",
			wantErr: true,
			errMsg:  "output file directory does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutputFile(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestSetOutputFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		outputFilePath string
		appPath        string
		wantPath       string
		wantErr        bool
	}{
		{
			name:           "empty output uses default in app directory",
			outputFilePath: "",
			appPath:        tmpDir,
			wantPath:       filepath.Join(tmpDir, defaultDiffFileName),
			wantErr:        false,
		},
		{
			name:           "custom valid path",
			outputFilePath: filepath.Join(tmpDir, "custom.diff"),
			appPath:        tmpDir,
			wantPath:       filepath.Join(tmpDir, "custom.diff"),
			wantErr:        false,
		},
		{
			name:           "custom path with wrong extension",
			outputFilePath: filepath.Join(tmpDir, "custom.txt"),
			appPath:        tmpDir,
			wantErr:        true,
		},
		{
			name:           "custom path in nonexistent directory",
			outputFilePath: "/fake/dir/custom.diff",
			appPath:        tmpDir,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setOutputFilePath(tt.outputFilePath, tt.appPath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, got)
			}
		})
	}
}

func TestPadding(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  int
	}{
		{
			name:  "shorter than width",
			input: "hello",
			width: 10,
			want:  5,
		},
		{
			name:  "equal to width",
			input: "0123456789",
			width: 10,
			want:  0,
		},
		{
			name:  "longer than width",
			input: "this is a very long string",
			width: 10,
			want:  0,
		},
		{
			name:  "empty string",
			input: "",
			width: 10,
			want:  10,
		},
		{
			name:  "zero width",
			input: "hello",
			width: 0,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padding(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("padding(%q, %d) = %d, want %d", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestInitialModel(t *testing.T) {
	m := initialModel("/test/path", "output.diff")

	if m.pkgPath != "/test/path" {
		t.Errorf("expected pkgPath %q, got %q", "/test/path", m.pkgPath)
	}
	if m.outputFile != "output.diff" {
		t.Errorf("expected outputFile %q, got %q", "output.diff", m.outputFile)
	}
	if m.stepDesc != "Loading packages..." {
		t.Errorf("expected stepDesc %q, got %q", "Loading packages...", m.stepDesc)
	}
	if m.totalSteps != 8 {
		t.Errorf("expected totalSteps %d, got %d", 8, m.totalSteps)
	}
	if m.done {
		t.Error("expected done to be false")
	}
	if m.err != nil {
		t.Error("expected err to be nil")
	}
}

func TestModelView(t *testing.T) {
	t.Run("shows error when present", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		m.err = errMsg(os.ErrNotExist)
		view := m.View()
		if !strings.Contains(view, "Error:") {
			t.Errorf("expected view to contain 'Error:', got %q", view)
		}
	})

	t.Run("shows spinner and step description", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		m.stepDesc = "Instrumenting application"
		view := m.View()
		if !strings.Contains(view, "Instrumenting application") {
			t.Errorf("expected view to contain step description, got %q", view)
		}
	})
}

func TestModelUpdate(t *testing.T) {
	t.Run("ctrl+c quits", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		_ = updatedModel
		if cmd == nil {
			t.Error("expected quit command, got nil")
		}
	})

	t.Run("progress message updates step", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		m.sub = make(chan tea.Msg, 1)
		m.sub <- completedMsg{} // buffer something so waitForNext doesn't block

		updatedModel, _ := m.Update(progressMsg{desc: "Scanning application"})
		updated := updatedModel.(model)
		if updated.stepDesc != "Scanning application" {
			t.Errorf("expected stepDesc %q, got %q", "Scanning application", updated.stepDesc)
		}
		if updated.currentStep != 1 {
			t.Errorf("expected currentStep 1, got %d", updated.currentStep)
		}
	})

	t.Run("error message sets error and quits", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		testErr := errMsg(os.ErrPermission)
		updatedModel, cmd := m.Update(testErr)
		updated := updatedModel.(model)
		if updated.err == nil {
			t.Error("expected error to be set")
		}
		if cmd == nil {
			t.Error("expected quit command, got nil")
		}
	})

	t.Run("completed message sets done and quits", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		updatedModel, cmd := m.Update(completedMsg{})
		updated := updatedModel.(model)
		if !updated.done {
			t.Error("expected done to be true")
		}
		if cmd == nil {
			t.Error("expected quit command, got nil")
		}
	})

	t.Run("packages loaded message updates packages", func(t *testing.T) {
		m := initialModel("/test/path", "output.diff")
		m.sub = make(chan tea.Msg, 1)
		m.sub <- completedMsg{} // buffer something so waitForNext doesn't block

		updatedModel, _ := m.Update(pkgLoadedMsg(nil))
		updated := updatedModel.(model)
		if updated.stepDesc != "Starting instrumentation..." {
			t.Errorf("expected stepDesc %q, got %q", "Starting instrumentation...", updated.stepDesc)
		}
	})
}

func TestWaitForNext(t *testing.T) {
	t.Run("nil channel returns nil", func(t *testing.T) {
		cmd := waitForNext(nil)
		if cmd != nil {
			t.Error("expected nil command for nil channel")
		}
	})

	t.Run("reads message from channel", func(t *testing.T) {
		ch := make(chan tea.Msg, 1)
		ch <- progressMsg{desc: "test step"}
		cmd := waitForNext(ch)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		msg := cmd()
		if pm, ok := msg.(progressMsg); !ok || pm.desc != "test step" {
			t.Errorf("expected progressMsg with desc 'test step', got %v", msg)
		}
	})

	t.Run("closed channel returns nil message", func(t *testing.T) {
		ch := make(chan tea.Msg)
		close(ch)
		cmd := waitForNext(ch)
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		msg := cmd()
		if msg != nil {
			t.Errorf("expected nil message from closed channel, got %v", msg)
		}
	})
}

// Integration tests for instrumentPackages

func TestInstrumentPackages_BasicGin(t *testing.T) {
	packagePath := filepath.Join("..", "end-to-end-tests", "gin-examples", "basic")
	if _, err := os.Stat(packagePath); err != nil {
		t.Skipf("test fixture not found: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "output.diff")

	err := instrumentPackages(packagePath, nil, outputFile)
	if err != nil {
		t.Fatalf("instrumentPackages failed: %v", err)
	}

	// Verify the diff file was created and has content
	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output diff file is empty, expected instrumentation changes")
	}
}

func TestInstrumentPackages_HttpApp(t *testing.T) {
	packagePath := filepath.Join("..", "end-to-end-tests", "http-app")
	if _, err := os.Stat(packagePath); err != nil {
		t.Skipf("test fixture not found: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "output.diff")

	err := instrumentPackages(packagePath, nil, outputFile)
	if err != nil {
		t.Fatalf("instrumentPackages failed: %v", err)
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output diff file is empty, expected instrumentation changes")
	}
}

func TestInstrumentPackages_CustomPatterns(t *testing.T) {
	packagePath := filepath.Join("..", "end-to-end-tests", "gin-examples", "basic")
	if _, err := os.Stat(packagePath); err != nil {
		t.Skipf("test fixture not found: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "output.diff")

	// Use explicit patterns instead of the default "./..."
	err := instrumentPackages(packagePath, []string{"./..."}, outputFile)
	if err != nil {
		t.Fatalf("instrumentPackages with custom patterns failed: %v", err)
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output diff file is empty, expected instrumentation changes")
	}
}

func TestInstrumentPackages_InvalidPackagePath(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "output.diff")

	err := instrumentPackages("/nonexistent/path/to/package", nil, outputFile)
	if err == nil {
		t.Fatal("expected error for invalid package path, got nil")
	}
	if !strings.Contains(err.Error(), "loading packages") {
		t.Errorf("expected error about loading packages, got: %v", err)
	}
}
