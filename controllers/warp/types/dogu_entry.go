package types

import (
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"
	"strings"

	"github.com/pkg/errors"
)

type WatchConfigurationContext interface {
	registry.WatchConfigurationContext
}

type doguEntry struct {
	Name        string
	DisplayName string
	Description string
	Category    string
	Tags        []string
}

// DoguConverter converts dogus from the configuration to a warp menu category object
type DoguConverter struct{}

// CreateEntryWithCategoryFromDogu returns a doguEntry with category if the dogu has the tag specified as parameter.
func (dc *DoguConverter) CreateEntryWithCategoryFromDogu(dogu *core.Dogu, tag string) (EntryWithCategory, error) {
	doguEntry := doguEntryFromDogu(dogu)

	if tag == "" || containsString(doguEntry.Tags, tag) {
		return mapDoguEntry(doguEntry)
	}

	return EntryWithCategory{}, nil
}

func doguEntryFromDogu(dogu *core.Dogu) doguEntry {
	return doguEntry{
		Name:        dogu.Name,
		DisplayName: dogu.DisplayName,
		Description: dogu.Description,
		Category:    dogu.Category,
		Tags:        dogu.Tags,
	}
}

func mapDoguEntry(entry doguEntry) (EntryWithCategory, error) {
	if entry.Name == "" {
		return EntryWithCategory{}, errors.New("name is required for dogu entries")
	}

	displayName := entry.DisplayName
	if displayName == "" {
		displayName = entry.Name
	}

	return EntryWithCategory{
		Entry: Entry{
			DisplayName: displayName,
			Title:       entry.Description,
			Target:      TARGET_SELF,
			Href:        createDoguHref(entry.Name),
		},
		Category: entry.Category,
	}, nil
}

func createDoguHref(name string) string {
	// remove namespace
	parts := strings.Split(name, "/")
	return "/" + parts[len(parts)-1]
}

// ContainsString returns true if the slice contains the item
func containsString(slice []string, item string) bool {
	for _, sliceItem := range slice {
		if sliceItem == item {
			return true
		}
	}
	return false
}
