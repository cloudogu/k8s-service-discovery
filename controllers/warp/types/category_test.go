package types

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCategories_Len(t *testing.T) {
	// given
	a := &Category{}
	b := &Category{}
	categories := Categories{a, b}

	// when
	length := categories.Len()

	// then
	assert.Equal(t, 2, length)
}

func TestCategories_Less(t *testing.T) {
	t.Run("less with different orders", func(t *testing.T) {
		// given
		a := &Category{Order: 1}
		b := &Category{Order: 100}
		categories := Categories{a, b}

		// when
		isLess := categories.Less(0, 1)

		// then
		assert.Equal(t, false, isLess)
	})

	t.Run("should orientate on title with same orders", func(t *testing.T) {
		// given
		a := &Category{Order: 100, Title: "A"}
		b := &Category{Order: 100, Title: "B"}
		categories := Categories{a, b}

		// when
		isLess := categories.Less(0, 1)

		// then
		assert.Equal(t, true, isLess)
	})
}

func TestCategories_Swap(t *testing.T) {
	// given
	a := &Category{Order: 1}
	b := &Category{Order: 100}
	categories := Categories{a, b}

	// when
	categories.Swap(0, 1)

	// then
	assert.Equal(t, categories[0], b)
	assert.Equal(t, categories[1], a)
}

func TestCategories_insertCategories(t *testing.T) {
	// given
	a := &Category{Order: 1, Title: "a"}
	b := &Category{Order: 100, Title: "b"}
	categories := Categories{a, b}
	aa := &Category{Order: 1, Title: "aa"}
	bb := &Category{Order: 100, Title: "bb"}
	addCategories := Categories{aa, bb}

	// when
	categories.InsertCategories(addCategories)

	// then
	assert.Equal(t, 4, len(categories))
	assert.Equal(t, a, categories[0])
	assert.Equal(t, b, categories[1])
	assert.Equal(t, aa, categories[2])
	assert.Equal(t, bb, categories[3])
}

func TestCategories_insertCategory(t *testing.T) {
	t.Run("new title does not exists", func(t *testing.T) {
		// given
		a := &Category{Order: 1, Title: "a"}
		b := &Category{Order: 100, Title: "b"}
		categories := Categories{a, b}
		add := &Category{Order: 50, Title: "c"}

		// when
		categories.InsertCategory(add)

		// then
		assert.Equal(t, 3, len(categories))
		assert.Equal(t, add, categories[2])
	})

	t.Run("add entries on same title", func(t *testing.T) {
		// given
		aEntry := Entry{Title: "a"}
		aEntries := Entries{aEntry}
		a := &Category{Order: 1, Title: "a", Entries: aEntries}
		b := &Category{Order: 100, Title: "b"}
		categories := Categories{a, b}
		addEntry := Entry{Title: "add"}
		addEntries := Entries{addEntry}
		add := &Category{Order: 50, Title: "a", Entries: addEntries}

		// when
		categories.InsertCategory(add)

		// then
		assert.Equal(t, 2, len(categories))
		assert.Equal(t, aEntry, categories[0].Entries[0])
		assert.Equal(t, addEntry, categories[0].Entries[1])
	})
}

func TestCategory_String(t *testing.T) {
	// given
	a := &Category{Order: 1, Title: "title"}

	// when
	str := a.String()

	// then
	assert.Equal(t, "title", str)
}
