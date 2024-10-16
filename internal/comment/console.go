package comment

import (
	"log"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

type ConsolePrinter struct {
	comments []string
}

// initialize this if you want to use it at the start of the program
var printer *ConsolePrinter

func EnableConsolePrinter() {
	printer = &ConsolePrinter{
		comments: []string{},
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

	pos := util.Position(node, pkg)
	b := strings.Builder{}
	b.WriteString(header)
	b.WriteByte(' ')
	if pos != nil {
		b.WriteString(pos.String())
		b.WriteByte(' ')
	}
	b.WriteString(message)
	for _, info := range additionalInfo {
		b.WriteString("\n\t")
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
