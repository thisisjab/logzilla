package skiplist

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

// SkipList represents a probabilistic alternative to balanced trees,
// maintaining elements in sorted order with O(log n) average search and insertion time.
type SkipList[T any] struct {
	len     int
	level   int
	head    *node[T]
	compare func(a, b T) int
}

// New creates and initializes an empty SkipList with the given comparison function.
// The compare function should return a negative number if a < b, 0 if a == b, and a positive number if a > b.
func New[T any](compare func(a, b T) int) *SkipList[T] {
	return &SkipList[T]{
		compare: compare,
		level:   1,
		head:    newNode(*new(T), maxLevels),
	}
}

// Len returns the number of elements in the skip list.
func (s *SkipList[T]) Len() int {
	return s.len
}

// Search looks for value v in the skip list.
// Returns the matched element and true if found, or the zero value of T and false otherwise.
func (s *SkipList[T]) Search(v T) (T, bool) {
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
func (s *SkipList[T]) Add(v T) {
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
func (s *SkipList[T]) Remove(value T) bool {
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

// randomLevel generates a random height for a new node using a geometric distribution (p=0.5).
func randomLevel() int {
	l := 1
	for l < maxLevels && rand.Int()%2 == 0 {
		l++
	}
	return l
}
