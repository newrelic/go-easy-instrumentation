package facts

import "fmt"

// Entry is a struct that represents a fact entry.
type Entry struct {
	Name string
	Fact Fact
}

func (e Entry) String() string {
	return fmt.Sprintf("{Name: %s, Fact: %s}", e.Name, e.Fact)
}
