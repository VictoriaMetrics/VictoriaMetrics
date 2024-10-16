// Copyright (c) 2023 Shivaram Lingamneni <slingamn@cs.stanford.edu>
// released under the MIT license

package ringbuf

type Buffer[T any] struct {
	// three possible states:
	// empty: start == end == -1
	// partially full: start != end
	// full: start == end > 0
	// if entries exist, they go from `start` to `(end - 1) % length`
	buffer      []T
	start       int
	end         int
	maximumSize int
}

func NewExpandableBuffer[T any](initialSize, maximumSize int) (result *Buffer[T]) {
	result = new(Buffer[T])
	result.Initialize(initialSize, maximumSize)
	return
}

func (hist *Buffer[T]) Initialize(initialSize, maximumSize int) {
	if maximumSize == 0 {
		panic("maximum size cannot be 0")
	}
	hist.buffer = make([]T, initialSize)
	hist.start = -1
	hist.end = -1
	hist.maximumSize = maximumSize
}

// Add adds an item to the buffer
func (list *Buffer[T]) Add(item T) {
	list.maybeExpand()

	var pos int
	if list.start == -1 { // empty
		pos = 0
		list.start = 0
		list.end = 1 % len(list.buffer)
	} else if list.start != list.end { // partially full
		pos = list.end
		list.end = (list.end + 1) % len(list.buffer)
	} else if list.start == list.end { // full
		pos = list.end
		list.end = (list.end + 1) % len(list.buffer)
		list.start = list.end // advance start as well, overwriting first entry
	}

	list.buffer[pos] = item
}

func (list *Buffer[T]) Pop() (item T, success bool) {
	length := list.Length()
	if length == 0 {
		return item, false
	} else {
		pos := list.prev(list.end)
		item = list.buffer[pos]
		list.buffer[pos] = *new(T) // TODO verify that this doesn't allocate
		if length > 1 {
			list.end = pos
		} else {
			// reset to empty buffer
			list.start = -1
			list.end = -1
		}
		return item, true
	}
}

func (list *Buffer[T]) Range(ascending bool, rangeFunction func(item *T) (stopIteration bool)) {
	if list.start == -1 || len(list.buffer) == 0 {
		return
	}

	var pos, stop int
	if ascending {
		pos = list.start
		stop = list.prev(list.end)
	} else {
		pos = list.prev(list.end)
		stop = list.start
	}

	for {
		if shouldStop := rangeFunction(&list.buffer[pos]); shouldStop {
			return
		}

		if pos == stop {
			return
		}

		if ascending {
			pos = list.next(pos)
		} else {
			pos = list.prev(pos)
		}
	}
}

type Predicate[T any] func(item *T) (matches bool)

func (list *Buffer[T]) Match(ascending bool, predicate Predicate[T], limit int) []T {
	var results []T
	rangeFunc := func(item *T) (stopIteration bool) {
		if predicate(item) {
			results = append(results, *item)
			return limit > 0 && len(results) >= limit
		} else {
			return false
		}
	}
	list.Range(ascending, rangeFunc)
	return results
}

func (list *Buffer[T]) prev(index int) int {
	switch index {
	case 0:
		return len(list.buffer) - 1
	default:
		return index - 1
	}
}

func (list *Buffer[T]) next(index int) int {
	switch index {
	case len(list.buffer) - 1:
		return 0
	default:
		return index + 1
	}
}

func (list *Buffer[T]) maybeExpand() {
	length := list.Length()
	if length < len(list.buffer) {
		return // we have spare capacity already
	}

	if len(list.buffer) == list.maximumSize {
		return // cannot expand any further
	}

	newSize := roundUpToPowerOfTwo(length + 1)
	if list.maximumSize < newSize {
		newSize = list.maximumSize
	}
	list.resize(newSize)
}

// return n such that v <= n and n == 2**i for some i
func roundUpToPowerOfTwo(v int) int {
	// http://graphics.stanford.edu/~seander/bithacks.html
	v -= 1
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	return v + 1
}

func (hist *Buffer[T]) Length() int {
	if hist.start == -1 {
		return 0
	} else if hist.start < hist.end {
		return hist.end - hist.start
	} else {
		return len(hist.buffer) - (hist.start - hist.end)
	}
}

func (list *Buffer[T]) resize(size int) {
	newbuffer := make([]T, size)

	if list.start == -1 {
		// indices are already correct and nothing needs to be copied
	} else if size == 0 {
		// this is now the empty list
		list.start = -1
		list.end = -1
	} else {
		currentLength := list.Length()
		start := list.start
		end := list.end
		// if we're truncating, keep the latest entries, not the earliest
		if size < currentLength {
			start = list.end - size
			if start < 0 {
				start += len(list.buffer)
			}
		}
		if start < end {
			copied := copy(newbuffer, list.buffer[start:end])
			list.start = 0
			list.end = copied % size
		} else {
			lenInitial := len(list.buffer) - start
			copied := copy(newbuffer, list.buffer[start:])
			copied += copy(newbuffer[lenInitial:], list.buffer[:end])
			list.start = 0
			list.end = copied % size
		}
	}

	list.buffer = newbuffer
}

func (hist *Buffer[T]) Clear() {
	hist.Range(true, func(item *T) bool {
		var zero T
		*item = zero
		return false
	})
	hist.start = -1
	hist.end = -1
}
