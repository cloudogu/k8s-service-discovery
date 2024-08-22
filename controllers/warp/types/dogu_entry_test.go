package types

import (
	_ "embed"
	"fmt"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:embed testdata/redmine.json
var redmineBytes []byte

func Test_containsString(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		s := "s"
		slice := []string{"s", "sp"}

		// when
		result := containsString(slice, s)

		// then
		assert.True(t, result)
	})

	t.Run("not in slive", func(t *testing.T) {
		// given
		s := "s"
		slice := []string{"a", "sp"}

		// when
		result := containsString(slice, s)

		// then
		assert.False(t, result)
	})
}

func Test_createDoguHref(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		doguStr := "namespace/redmine"

		// when
		result := createDoguHref(doguStr)

		// then
		assert.Equal(t, "/redmine", result)
	})
}

func Test_mapDoguEntry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		doguEntry := doguEntry{Name: "redmine", DisplayName: "HD", Category: "Dev"}

		// when
		entryWithCategory, err := mapDoguEntry(doguEntry)

		// then
		require.NoError(t, err)
		assert.Equal(t, doguEntry.Category, entryWithCategory.Category)
		assert.Equal(t, doguEntry.Description, entryWithCategory.Entry.Title)
		assert.Equal(t, doguEntry.DisplayName, entryWithCategory.Entry.DisplayName)
		assert.Equal(t, TARGET_SELF, entryWithCategory.Entry.Target)
		assert.Equal(t, "/redmine", entryWithCategory.Entry.Href)
	})

	t.Run("error on nameless entry", func(t *testing.T) {
		// given
		doguEntry := doguEntry{Name: "", DisplayName: "HD", Category: "Dev"}

		// when
		_, err := mapDoguEntry(doguEntry)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required for dogu entries")
	})

	t.Run("user name on empty displayname", func(t *testing.T) {
		// given
		doguEntry := doguEntry{Name: "redmine", DisplayName: "", Category: "Dev"}

		// when
		entryWithCategory, err := mapDoguEntry(doguEntry)

		// then
		require.NoError(t, err)
		assert.Equal(t, doguEntry.Category, entryWithCategory.Category)
		assert.Equal(t, doguEntry.Description, entryWithCategory.Entry.Title)
		assert.Equal(t, doguEntry.Name, entryWithCategory.Entry.DisplayName)
		assert.Equal(t, TARGET_SELF, entryWithCategory.Entry.Target)
		assert.Equal(t, "/redmine", entryWithCategory.Entry.Href)
	})
}

func TestDoguConverter_CreateEntryWithCategoryFromDogu(t *testing.T) {
	redmineDogu := readRedmineDogu(t)

	type args struct {
		dogu *core.Dogu
		tag  string
	}
	tests := []struct {
		name    string
		args    args
		want    EntryWithCategory
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "should create entry with category with correct tag",
			args: args{dogu: redmineDogu, tag: "warp"},
			want: EntryWithCategory{Entry: Entry{
				DisplayName: "Redmine",
				Href:        "/redmine",
				Title:       "Redmine is a flexible project management web application",
				Target:      1,
			},
				Category: "Development Apps",
			},
			wantErr: assert.NoError,
		},
		{
			name: "should create entry with category with empty tag",
			args: args{dogu: redmineDogu, tag: ""},
			want: EntryWithCategory{Entry: Entry{
				DisplayName: "Redmine",
				Href:        "/redmine",
				Title:       "Redmine is a flexible project management web application",
				Target:      1,
			},
				Category: "Development Apps",
			},
			wantErr: assert.NoError,
		},
		{
			name:    "should return empty entry with category on wrong tag",
			args:    args{dogu: redmineDogu, tag: "wrong tag"},
			want:    EntryWithCategory{},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &DoguConverter{}
			got, err := dc.CreateEntryWithCategoryFromDogu(tt.args.dogu, tt.args.tag)
			if !tt.wantErr(t, err, fmt.Sprintf("CreateEntryWithCategoryFromDogu(%v, %v)", tt.args.dogu, tt.args.tag)) {
				return
			}
			assert.Equalf(t, tt.want, got, "CreateEntryWithCategoryFromDogu(%v, %v)", tt.args.dogu, tt.args.tag)
		})
	}
}
