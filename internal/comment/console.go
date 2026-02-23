package comment

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

type consoleEntry struct {
	header  string
	message string
}

type ConsolePrinter struct {
	appRoot string
	entries []consoleEntry
}

// initialize this if you want to use it at the start of the program
var printer *ConsolePrinter

func EnableConsolePrinter(applicationPath string) {
	printer = &ConsolePrinter{
		appRoot: filepath.Base(applicationPath),
	}
}

func WriteAll() {
	printer.flush()
}

// Add appends a new entry to the console printer.
// The message is the main comment, and additionalInfo is a list of optional
// comments that will be printed on new lines below the main comment.
func (p *ConsolePrinter) add(pkg *decorator.Package, node dst.Node, header, message string, additionalInfo ...string) {
	if p == nil {
		return
	}

	pos := getPosition(pkg, node, p.appRoot)

	b := strings.Builder{}
	b.WriteString(header)
	b.WriteByte(':')
	b.WriteByte(' ')

	if pos != "" {
		b.WriteString(pos)
		b.WriteByte(' ')
	}
	b.WriteString(message)
	for _, info := range additionalInfo {
		b.WriteString("\n")
		b.WriteString(info)
	}

	p.entries = append(p.entries, consoleEntry{header: header, message: b.String()})
}

// Flush logs only Debug-level entries to the console.
func (p *ConsolePrinter) flush() {
	if p == nil {
		return
	}

	for _, e := range p.entries {
		if e.header == DebugConsoleHeader {
			log.Println(e.message)
		}
	}
	p.entries = []consoleEntry{}
}
