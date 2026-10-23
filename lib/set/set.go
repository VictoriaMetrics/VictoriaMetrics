package set

import (
	"cmp"
	"slices"
)

type Ordered[T cmp.Ordered] map[T]struct{}

func (s Ordered[T]) Add(v T) { s[v] = struct{}{} }

func (s Ordered[T]) Items() []T {
	result := make([]T, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	slices.Sort(result)
	return result
}
