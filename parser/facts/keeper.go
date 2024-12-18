package facts

import "fmt"

// Keeper is a map of facts.
type Keeper map[string]Fact

// NewKeeper creates and initializes a new Keeper.
func NewKeeper() Keeper {
	return make(Keeper)
}

// AddFact adds a new fact to the Keeper.
func (fm Keeper) AddFact(entry Entry) error {
	if entry.Fact == None {
		return fmt.Errorf("invalid fact kind: %s", entry.Fact.String())
	}
	if entry.Fact > maximumFactValue {
		return fmt.Errorf("unknown fact: %d", entry.Fact)
	}
	if entry.Name == "" {
		return fmt.Errorf("empty fact name")
	}

	if _, ok := fm[entry.Name]; ok {
		return fmt.Errorf("fact already exists: %s", entry.Name)
	}

	fm[entry.Name] = entry.Fact
	return nil
}

// GetFact returns a fact from the Keeper.
// If the fact does not exist, it returns None.
func (fm Keeper) GetFact(name string) Fact {
	return fm[name]
}
