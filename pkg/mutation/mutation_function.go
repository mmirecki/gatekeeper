package mutation

import (
	"fmt"

	"github.com/open-policy-agent/gatekeeper/pkg/mutation/path/parser"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("mutation")

func Mutate(mutator Mutator, obj *unstructured.Unstructured) error {
	return mutate(mutator, obj.Object, nil, 0)
}

func mutate(m Mutator, current interface{}, previous interface{}, depth int) error {
	if len(m.Path().Nodes)-1 == depth {
		return addValue(m, current, previous, depth)
	}
	pathEntry := m.Path().Nodes[depth]
	switch t := pathEntry.Type(); t {
	case parser.ObjectNode:
		next, ok := current.(map[string]interface{})[pathEntry.(*parser.Object).Reference]
		if !ok {
			next = createMissingElement(m, current, previous, depth)
		}
		if err := mutate(m, next, current, depth+1); err != nil {
			return err
		}
		return nil
	case parser.ListNode:
		elementFound := false
		listPathEntry := pathEntry.(*parser.List)
		glob := listPathEntry.Glob
		key := listPathEntry.KeyField
		for _, listElement := range current.([]interface{}) {
			if glob {
				if err := mutate(m, listElement, current, depth+1); err != nil {
					return err
				}
				elementFound = true
			} else if elementValue, ok := listElement.(map[string]interface{})[key]; ok {
				if *listPathEntry.KeyValue == elementValue {
					if err := mutate(m, listElement, current, depth+1); err != nil {
						return err
					}
					elementFound = true
				}
			}
		}
		// If no matching element in the array was found in non Globbed list, create a new element
		if !listPathEntry.Glob && !elementFound {
			next := createMissingElement(m, current, previous, depth)
			if err := mutate(m, next, current, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("Unrecognized type: %v", t)
	}
	return nil
}

func addValue(m Mutator, current interface{}, previous interface{}, depth int) error {
	// TODO: it should be considered if the value set can be not just a simple string, but json which could be unmarshalled into an object
	pathEntry := m.Path().Nodes[depth]
	switch t := pathEntry.Type(); t {
	case parser.ObjectNode:

		switch m.(type) {
		case *AssignMetadataMutator:
			if elementValue, ok := current.(map[string]interface{})[pathEntry.(*parser.Object).Reference]; ok {
				log.Info("Mutated value already present", "field", pathEntry.(*parser.Object).Reference, "value", elementValue)
				return nil
			}
		}
		value, err := m.Value()
		if err != nil {
			return err
		}
		current.(map[string]interface{})[pathEntry.(*parser.Object).Reference] = value
	case parser.ListNode:
		return addListElementWithValue(m, current, previous, depth)
	}
	return nil
}

func addListElementWithValue(m Mutator, current interface{}, previous interface{}, depth int) error {
	listPathEntry := m.Path().Nodes[depth].(*parser.List)

	if listPathEntry.Glob {
		return fmt.Errorf("Last path entry can not be globbed")
	}
	key := listPathEntry.KeyField
	keyValue := listPathEntry.KeyValue

	for _, listElement := range current.([]interface{}) {
		if elementValue, ok := listElement.(map[string]interface{})[key]; ok && keyValue == elementValue {
			return nil // Element is already present, skip the update
		}
	}
	current = append(current.([]interface{}), map[string]interface{}{key: *keyValue})
	previous.(map[string]interface{})[m.Path().Nodes[depth-1].(*parser.Object).Reference] = current
	return nil
}

func createMissingElement(m Mutator, current interface{}, previous interface{}, depth int) interface{} {
	var next interface{}
	pathEntry := m.Path().Nodes[depth]

	// Create new element of type
	switch m.Path().Nodes[depth+1].Type() {
	case parser.ObjectNode:
		next = make(map[string]interface{})
	case parser.ListNode:
		next = make([]interface{}, 0)
	}

	// Append to element of type
	switch pathEntry.Type() {
	case parser.ObjectNode:
		current.(map[string]interface{})[pathEntry.(*parser.Object).Reference] = next
	case parser.ListNode:
		current = append(current.([]interface{}), next)
		next.(map[string]interface{})[pathEntry.(*parser.List).KeyField] = *pathEntry.(*parser.List).KeyValue
		previous.(map[string]interface{})[m.Path().Nodes[depth-1].(*parser.Object).Reference] = current
	}
	return next
}
