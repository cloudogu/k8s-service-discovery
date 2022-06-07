package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestCategoryString(t *testing.T) {
	category := Category{Title: "Hitchhiker"}
	assert.Equal(t, "Hitchhiker", fmt.Sprintf("%v", category))
}

func TestTarget_MarshalJSON(t *testing.T) {
	testMarshalJSON(t, TARGET_EXTERNAL, "{\"Target\":\"external\"}")
	testMarshalJSON(t, TARGET_SELF, "{\"Target\":\"self\"}")

	if _, err := json.Marshal(&targetStruct{12}); err == nil {
		t.Errorf("marshal should fail because of an invalid value")
	}
}

func TestEntries_Swap(t *testing.T) {
	// given
	entry1 := Entry{Title: "1"}
	entry2 := Entry{Title: "2"}
	entries := Entries{entry1, entry2}

	// when
	entries.Swap(0, 1)

	// then
	assert.Equal(t, "2", entries[0].Title)
	assert.Equal(t, "1", entries[1].Title)
}

func testMarshalJSON(t *testing.T, target Target, expected string) {
	value := marshal(t, target)
	if value != expected {
		t.Errorf("value %s is not the expected %s", value, expected)
	}
}

func marshal(t *testing.T, target Target) string {
	test := targetStruct{target}
	testJson, err := json.Marshal(&test)
	if err != nil {
		t.Errorf("failed to marshal test struct: %v", err)
	}
	return string(testJson)
}

type targetStruct struct {
	Target Target
}

func TestEntries_Len(t *testing.T) {
	// given
	entryA := Entry{}
	entryB := Entry{}
	entries := Entries{entryA, entryB}

	// when
	length := entries.Len()

	// then
	assert.Equal(t, 2, length)
}

func TestEntries_Less(t *testing.T) {
	// given
	entryA := Entry{DisplayName: "A"}
	entryB := Entry{DisplayName: "B"}
	entries := Entries{entryA, entryB}

	// when
	result := entries.Less(0, 1)

	// then
	assert.True(t, result)
}
