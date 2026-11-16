package sequence

import (
	"errors"
	"iter"
)

func ForEach[Item any](items iter.Seq[Item], process func(Item) error) error {
	var failures []error
	for item := range items {
		failures = append(failures, process(item))
	}
	return errors.Join(failures...)
}
