package storage

import "math/rand/v2"

// maxLevels defines the maximum number of forward pointer levels in the skip list.
const maxLevels = 16

// node represents an individual element in the skip list with multiple forward pointers.
type node[T any] struct {
	value T
	next  []*node[T]
}

// newNode creates a new node with the specified value and forward pointer capacity.
func newNode[T any](value T, level int) *node[T] {
	return &node[T]{
		value: value,
		next:  make([]*node[T], level),
	}
}

// skipList maintains elements in sorted order with O(log n) average search and insertion time.
type skipList[T any] struct {
	len     int
	level   int
	head    *node[T]
	compare func(a, b T) int
}

// newSkipList creates and initializes an empty skip list with the given comparison function.
func newSkipList[T any](compare func(a, b T) int) *skipList[T] {
	return &skipList[T]{
		compare: compare,
		level:   1,
		head:    newNode(*new(T), maxLevels),
	}
}

// Len returns the number of elements in the skip list.
func (s *skipList[T]) Len() int {
	return s.len
}

// Search looks for value v in the skip list.
func (s *skipList[T]) Search(v T) (T, bool) {
	current := s.head

	for i := s.level - 1; i >= 0; i-- {
		for current.next[i] != nil && s.compare(current.next[i].value, v) < 0 {
			current = current.next[i]
		}
	}

	if current.next[0] != nil && s.compare(current.next[0].value, v) == 0 {
		return current.next[0].value, true
	}

	return *new(T), false
}

// Add inserts a new value v into the skip list.
// If the value already exists according to compare func, the operation is a no-op.
func (s *skipList[T]) Add(v T) {
	update := make([]*node[T], maxLevels)
	current := s.head

	for i := s.level - 1; i >= 0; i-- {
		for current.next[i] != nil && s.compare(current.next[i].value, v) < 0 {
			current = current.next[i]
		}
		update[i] = current
	}

	if current.next[0] != nil && s.compare(current.next[0].value, v) == 0 {
		return
	}

	newLevel := randomLevel()

	if newLevel > s.level {
		for i := s.level; i < newLevel; i++ {
			update[i] = s.head
		}
		s.level = newLevel
	}

	node := newNode(v, newLevel)

	for i := 0; i < newLevel; i++ {
		node.next[i] = update[i].next[i]
		update[i].next[i] = node
	}

	s.len++
}

// Remove deletes the value from the skip list if present and returns true, or false if not found.
func (s *skipList[T]) Remove(value T) bool {
	current := s.head
	update := make([]*node[T], maxLevels)

	for i := s.level - 1; i >= 0; i-- {
		for current.next[i] != nil && s.compare(current.next[i].value, value) < 0 {
			current = current.next[i]
		}
		update[i] = current
	}

	target := current.next[0]

	if target == nil || s.compare(target.value, value) != 0 {
		return false
	}

	for i := 0; i < s.level; i++ {
		if update[i].next[i] != target {
			break
		}
		update[i].next[i] = target.next[i]
	}

	for s.level > 1 && s.head.next[s.level-1] == nil {
		s.level--
	}

	s.len--
	return true
}

// Iterator returns a sequential forward iterator traversing Level 0 from smallest to largest element.
func (s *skipList[T]) Iterator() *skipListIterator[T] {
	return &skipListIterator[T]{
		curr: s.head,
	}
}

// skipListIterator provides forward sequential traversal over Level 0 of a skipList.
type skipListIterator[T any] struct {
	curr *node[T]
}

// Next advances the iterator to the next element. Returns false when iteration is complete.
func (it *skipListIterator[T]) Next() bool {
	if it.curr == nil || it.curr.next[0] == nil {
		it.curr = nil
		return false
	}
	it.curr = it.curr.next[0]
	return true
}

// Value returns the current element value.
func (it *skipListIterator[T]) Value() T {
	if it.curr == nil {
		return *new(T)
	}
	return it.curr.value
}

func randomLevel() int {
	l := 1
	for l < maxLevels && rand.Int()%2 == 0 {
		l++
	}
	return l
}
