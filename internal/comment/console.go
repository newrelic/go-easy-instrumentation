package comment

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

type ConsolePrinter struct {
	appRoot  string
	comments []string
}

// initialize this if you want to use it at the start of the program
var printer *ConsolePrinter

func EnableConsolePrinter(applicationPath string) {
	printer = &ConsolePrinter{
		appRoot: filepath.Base(applicationPath),
	}
}

func WriteAll() {
	if printer != nil {
		printer.Flush()
	}
}

// Add appends a new comment to the consolePrints slice.
// This function is used to add comments that will be printed to console.
// The message is the main comment, and additionalInfo is a list of optional
// comments that will be printed on new lines below the main comment.
func (p *ConsolePrinter) Add(pkg *decorator.Package, node dst.Node, header, message string, additionalInfo ...string) {
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

	p.comments = append(p.comments, b.String())
}

// Flush logs all the comments in the consolePrints slice.
func (p *ConsolePrinter) Flush() {
	if p == nil {
		return
	}

	for _, c := range p.comments {
		log.Println(c)
	}
	p.comments = []string{}
}
