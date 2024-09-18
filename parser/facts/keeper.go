package facts

import "fmt"

type Keeper map[string]Fact

func NewKeeper() Keeper {
	return make(Keeper)
}

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

func (fm Keeper) GetFact(name string) Fact {
	return fm[name]
}
