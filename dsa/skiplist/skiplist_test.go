package skiplist

import (
	"cmp"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("initializes empty skip list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		require.NotNil(t, sl)
		assert.Equal(t, 0, sl.Len())
		assert.Equal(t, 1, sl.level)
		assert.NotNil(t, sl.head)
		assert.Len(t, sl.head.next, maxLevels)
	})
}

func TestSkipList_Search(t *testing.T) {
	t.Run("returns false and zero value on empty list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		val, found := sl.Search(42)

		assert.False(t, found)
		assert.Equal(t, 0, val)
	})

	t.Run("returns true and value when item exists", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)
		sl.Add(30)

		val, found := sl.Search(20)

		assert.True(t, found)
		assert.Equal(t, 20, val)
	})

	t.Run("returns false when searching for non-existent items", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)
		sl.Add(30)

		testCases := []int{5, 15, 25, 35, 100}
		for _, tc := range testCases {
			val, found := sl.Search(tc)
			assert.False(t, found, "expected not to find %d", tc)
			assert.Equal(t, 0, val)
		}
	})
}

func TestSkipList_Add(t *testing.T) {
	t.Run("inserts single item and updates len", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		sl.Add(10)

		assert.Equal(t, 1, sl.Len())
		val, found := sl.Search(10)
		assert.True(t, found)
		assert.Equal(t, 10, val)
	})

	t.Run("ignores duplicate values without increasing len", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		sl.Add(10)
		sl.Add(20)
		sl.Add(10)

		assert.Equal(t, 2, sl.Len())
		val, found := sl.Search(10)
		assert.True(t, found)
		assert.Equal(t, 10, val)
	})

	t.Run("inserts elements in descending order correctly", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		for i := 50; i >= 10; i -= 10 {
			sl.Add(i)
		}

		assert.Equal(t, 5, sl.Len())
		for i := 10; i <= 50; i += 10 {
			val, found := sl.Search(i)
			assert.True(t, found)
			assert.Equal(t, i, val)
		}
	})

	t.Run("maintains sorted order on level 0", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		values := []int{30, 10, 50, 20, 40}

		for _, v := range values {
			sl.Add(v)
		}

		assert.Equal(t, 5, sl.Len())

		var collected []int
		curr := sl.head.next[0]
		for curr != nil {
			collected = append(collected, curr.value)
			curr = curr.next[0]
		}

		assert.Equal(t, []int{10, 20, 30, 40, 50}, collected)
	})
}

func TestSkipList_Remove(t *testing.T) {
	t.Run("returns false when removing from empty list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])

		removed := sl.Remove(10)

		assert.False(t, removed)
		assert.Equal(t, 0, sl.Len())
	})

	t.Run("returns false when removing non-existent element", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(30)

		removed := sl.Remove(20)

		assert.False(t, removed)
		assert.Equal(t, 2, sl.Len())
	})

	t.Run("removes single existing element", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)

		removed := sl.Remove(10)

		assert.True(t, removed)
		assert.Equal(t, 0, sl.Len())
		assert.Equal(t, 1, sl.level)
		_, found := sl.Search(10)
		assert.False(t, found)
	})

	t.Run("removes first element in multi-element list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)
		sl.Add(30)

		removed := sl.Remove(10)

		assert.True(t, removed)
		assert.Equal(t, 2, sl.Len())
		_, found := sl.Search(10)
		assert.False(t, found)

		_, found = sl.Search(20)
		assert.True(t, found)
		_, found = sl.Search(30)
		assert.True(t, found)
	})

	t.Run("removes middle element in multi-element list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)
		sl.Add(30)

		removed := sl.Remove(20)

		assert.True(t, removed)
		assert.Equal(t, 2, sl.Len())
		_, found := sl.Search(20)
		assert.False(t, found)

		_, found = sl.Search(10)
		assert.True(t, found)
		_, found = sl.Search(30)
		assert.True(t, found)
	})

	t.Run("removes last element in multi-element list", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)
		sl.Add(30)

		removed := sl.Remove(30)

		assert.True(t, removed)
		assert.Equal(t, 2, sl.Len())
		_, found := sl.Search(30)
		assert.False(t, found)

		_, found = sl.Search(10)
		assert.True(t, found)
		_, found = sl.Search(20)
		assert.True(t, found)
	})

	t.Run("removes all elements sequentially", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		nums := []int{10, 20, 30, 40, 50}

		for _, n := range nums {
			sl.Add(n)
		}

		for _, n := range nums {
			removed := sl.Remove(n)
			assert.True(t, removed)
			_, found := sl.Search(n)
			assert.False(t, found)
		}

		assert.Equal(t, 0, sl.Len())
		assert.Equal(t, 1, sl.level)
	})

	t.Run("handles re-inserting removed elements", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		sl.Add(10)
		sl.Add(20)

		assert.True(t, sl.Remove(10))
		assert.Equal(t, 1, sl.Len())

		sl.Add(10)
		assert.Equal(t, 2, sl.Len())
		val, found := sl.Search(10)
		assert.True(t, found)
		assert.Equal(t, 10, val)
	})
}

func TestSkipList_Level0_And_LenIntegrity(t *testing.T) {
	countLevel0 := func(sl *SkipList[int]) (int, []int) {
		var collected []int
		curr := sl.head.next[0]
		for curr != nil {
			collected = append(collected, curr.value)
			curr = curr.next[0]
		}
		return len(collected), collected
	}

	t.Run("empty skip list has zero level 0 nodes and zero len", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		count, nodes := countLevel0(sl)

		assert.Equal(t, 0, sl.Len())
		assert.Equal(t, 0, count)
		assert.Empty(t, nodes)
		assert.Nil(t, sl.head.next[0])
	})

	t.Run("level 0 node count exactly matches len across mutations", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		nums := []int{45, 12, 89, 33, 7, 56, 21, 99, 64, 33, 12}

		for _, n := range nums {
			sl.Add(n)
			count, nodes := countLevel0(sl)
			assert.Equal(t, sl.Len(), count)

			for i := 1; i < len(nodes); i++ {
				assert.Less(t, nodes[i-1], nodes[i])
			}
		}

		toRemove := []int{7, 56, 99, 1000, 33}
		for _, n := range toRemove {
			sl.Remove(n)
			count, nodes := countLevel0(sl)
			assert.Equal(t, sl.Len(), count)

			for i := 1; i < len(nodes); i++ {
				assert.Less(t, nodes[i-1], nodes[i])
			}
		}

		for count, _ := countLevel0(sl); count > 0; count, _ = countLevel0(sl) {
			val := sl.head.next[0].value
			removed := sl.Remove(val)
			assert.True(t, removed)
			newCount, _ := countLevel0(sl)
			assert.Equal(t, sl.Len(), newCount)
		}

		assert.Equal(t, 0, sl.Len())
		assert.Nil(t, sl.head.next[0])
	})
}

func TestSkipList_Generics(t *testing.T) {
	t.Run("supports string values", func(t *testing.T) {
		sl := New[string](cmp.Compare[string])

		words := []string{"banana", "apple", "cherry", "date"}
		for _, w := range words {
			sl.Add(w)
		}

		assert.Equal(t, 4, sl.Len())

		val, found := sl.Search("apple")
		assert.True(t, found)
		assert.Equal(t, "apple", val)

		assert.True(t, sl.Remove("apple"))
		assert.Equal(t, 3, sl.Len())
		_, found = sl.Search("apple")
		assert.False(t, found)

		val, found = sl.Search("grape")
		assert.False(t, found)
		assert.Equal(t, "", val)
	})

	t.Run("supports custom struct with custom comparator", func(t *testing.T) {
		type userRecord struct {
			id   int
			name string
		}

		compareUser := func(a, b userRecord) int {
			return cmp.Compare(a.id, b.id)
		}

		sl := New(compareUser)
		sl.Add(userRecord{id: 3, name: "Charlie"})
		sl.Add(userRecord{id: 1, name: "Alice"})
		sl.Add(userRecord{id: 2, name: "Bob"})

		assert.Equal(t, 3, sl.Len())

		rec, found := sl.Search(userRecord{id: 2})
		assert.True(t, found)
		assert.Equal(t, "Bob", rec.name)

		assert.True(t, sl.Remove(userRecord{id: 2}))
		assert.Equal(t, 2, sl.Len())

		_, found = sl.Search(userRecord{id: 2})
		assert.False(t, found)

		_, found = sl.Search(userRecord{id: 99})
		assert.False(t, found)
	})
}

func TestSkipList_LargeScale(t *testing.T) {
	t.Run("handles large number of random insertions, lookups, and removals", func(t *testing.T) {
		sl := New[int](cmp.Compare[int])
		const count = 1000

		nums := make([]int, count)
		for i := 0; i < count; i++ {
			nums[i] = i * 2
		}

		rand.Shuffle(len(nums), func(i, j int) {
			nums[i], nums[j] = nums[j], nums[i]
		})

		for _, n := range nums {
			sl.Add(n)
		}

		assert.Equal(t, count, sl.Len())

		for i := 0; i < count; i++ {
			val, found := sl.Search(i * 2)
			assert.True(t, found)
			assert.Equal(t, i*2, val)
		}

		for i := 0; i < count; i++ {
			_, found := sl.Search(i*2 + 1)
			assert.False(t, found)
		}

		for i := 0; i < count/2; i++ {
			removed := sl.Remove(nums[i])
			assert.True(t, removed)
		}

		assert.Equal(t, count/2, sl.Len())

		for i := 0; i < count/2; i++ {
			_, found := sl.Search(nums[i])
			assert.False(t, found)
		}

		for i := count / 2; i < count; i++ {
			val, found := sl.Search(nums[i])
			assert.True(t, found)
			assert.Equal(t, nums[i], val)
		}
	})
}

func TestRandomLevel(t *testing.T) {
	t.Run("returns levels within [1, maxLevels]", func(t *testing.T) {
		for i := 0; i < 1000; i++ {
			lvl := randomLevel()
			assert.GreaterOrEqual(t, lvl, 1)
			assert.LessOrEqual(t, lvl, maxLevels)
		}
	})
}

func BenchmarkSkipList(b *testing.B) {
	b.Run("Add", func(b *testing.B) {
		sl := New[int](cmp.Compare[int])
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sl.Add(i)
		}
	})

	b.Run("Search", func(b *testing.B) {
		sl := New[string](cmp.Compare[string])
		for i := 0; i < 1000; i++ {
			sl.Add(strconv.Itoa(i))
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sl.Search(strconv.Itoa(i % 1000))
		}
	})

	b.Run("Remove", func(b *testing.B) {
		b.StopTimer()
		sl := New[int](cmp.Compare[int])
		for i := 0; i < b.N; i++ {
			sl.Add(i)
		}
		b.StartTimer()
		for i := 0; i < b.N; i++ {
			sl.Remove(i)
		}
	})
}
