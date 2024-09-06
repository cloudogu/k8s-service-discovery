package types

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_mapExternalEntry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		externalEntry := externalEntry{
			DisplayName: "HD-Display",
			URL:         "URL",
			Description: "Desc",
			Category:    "Category",
		}

		// when
		entryWithCategory, err := mapExternalEntry(externalEntry)

		// then
		require.NoError(t, err)
		require.NotNil(t, entryWithCategory)
		assert.Equal(t, TARGET_EXTERNAL, entryWithCategory.Entry.Target)
		assert.Equal(t, externalEntry.Category, entryWithCategory.Category)
	})

	t.Run("error because displayname is not set", func(t *testing.T) {
		// given
		externalEntry := externalEntry{}

		// when
		_, err := mapExternalEntry(externalEntry)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find DisplayName on external entry")
	})

	t.Run("error because url is not set", func(t *testing.T) {
		// given
		externalEntry := externalEntry{
			DisplayName: "HD-Display",
		}

		// when
		_, err := mapExternalEntry(externalEntry)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find URL on external entry")
	})

	t.Run("error because category is not set", func(t *testing.T) {
		// given
		externalEntry := externalEntry{
			DisplayName: "HD-Display",
			URL:         "URL",
		}

		// when
		_, err := mapExternalEntry(externalEntry)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find Category on external entry")
	})
}

func Test_readAndUnmarshalExternal(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		entryStr := "{\n  \"DisplayName\": \"display\",\n  \"URL\": \"url\",\n  \"Description\": \"desc\",\n  \"Category\": \"category\"\n}"
		expectedEntryWithCategory := EntryWithCategory{
			Entry: Entry{
				DisplayName: "display",
				Href:        "url",
				Title:       "desc",
				Target:      TARGET_EXTERNAL,
			},
			Category: "category",
		}
		externalConverter := ExternalConverter{}

		// when
		result, err := externalConverter.ReadAndUnmarshalExternal(entryStr)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedEntryWithCategory, result)
	})
}

func Test_unmarshalExternal(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		externalEntryStr := "{\n  \"DisplayName\": \"Redmine\",\n  \"URL\": \"/redmine\",\n  \"Description\": \"Redmine\",\n  \"Category\": \"Development Apps\"\n}"

		// when
		entryWithCategory, err := unmarshalExternal([]byte(externalEntryStr))

		// then
		require.NoError(t, err)
		assert.Equal(t, TARGET_EXTERNAL, entryWithCategory.Entry.Target)
		assert.Equal(t, "Development Apps", entryWithCategory.Category)
	})

	t.Run("failed to unmarshal external entry", func(t *testing.T) {
		// given
		externalEntryStr := "fdksjf"

		// when
		_, err := unmarshalExternal([]byte(externalEntryStr))

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshall external")
	})
}
