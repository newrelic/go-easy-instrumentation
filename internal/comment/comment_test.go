package comment

import (
	"testing"

	"github.com/dave/dst"
)

func TestInfo(t *testing.T) {
	node := &dst.Ident{Name: "hi"}
	Info(nil, node, nil, "message", "additionalInfo")

	decs := node.Decorations()
	if len(decs.Start) != 2 {
		t.Errorf("Expected 2 comments, got %d", len(decs.Start))
	} else {
		expected := []string{
			"// NR INFO: message",
			"// additionalInfo",
		}
		for i, comment := range decs.Start {
			if comment != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], comment)
			}
		}
	}

	nodeWithComments := &dst.Ident{Name: "hi", Decs: dst.IdentDecorations{NodeDecs: dst.NodeDecs{Start: []string{"// existing comment"}}}}
	Info(nil, nodeWithComments, nil, "message", "additionalInfo")
	decs = nodeWithComments.Decorations()
	if len(decs.Start) != 4 {
		t.Errorf("Expected 4 comments, got %d", len(decs.Start))
	} else {
		expected := []string{
			"// NR INFO: message",
			"// additionalInfo",
			"//",
			"// existing comment",
		}
		for i, comment := range decs.Start {
			if comment != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], comment)
			}
		}
	}
}

func TestWarn(t *testing.T) {
	node := &dst.Ident{Name: "hi"}
	Warn(nil, node, nil, "message", "additionalInfo")

	decs := node.Decorations()
	if len(decs.Start) != 2 {
		t.Errorf("Expected 2 comments, got %d", len(decs.Start))
	} else {
		expected := []string{
			"// NR WARN: message",
			"// additionalInfo",
		}
		for i, comment := range decs.Start {
			if comment != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], comment)
			}
		}
	}

	nodeWithComments := &dst.Ident{Name: "hi", Decs: dst.IdentDecorations{NodeDecs: dst.NodeDecs{Start: []string{"// existing comment"}}}}
	Warn(nil, nodeWithComments, nil, "message", "additionalInfo")
	decs = nodeWithComments.Decorations()
	if len(decs.Start) != 4 {
		t.Errorf("Expected 4 comments, got %d", len(decs.Start))
	} else {
		expected := []string{
			"// NR WARN: message",
			"// additionalInfo",
			"//",
			"// existing comment",
		}
		for i, comment := range decs.Start {
			if comment != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], comment)
			}
		}
	}
}
