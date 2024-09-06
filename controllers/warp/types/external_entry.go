package types

import (
	"fmt"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type externalEntry struct {
	DisplayName string `yaml:"DisplayName"`
	URL         string `yaml:"URL"`
	Description string `yaml:"Description"`
	Category    string `yaml:"Category"`
}

// EntryWithCategory is a dto for entries with a Category
type EntryWithCategory struct {
	Entry    Entry
	Category string
}

// ExternalConverter is used to read external links from the configuration and convert them to a warp menu category object.
type ExternalConverter struct{}

// ReadAndUnmarshalExternal reads a specific external link from the configuration and converts it to an entry with a category.
func (ec *ExternalConverter) ReadAndUnmarshalExternal(value string) (EntryWithCategory, error) {
	return unmarshalExternal([]byte(value))
}

func unmarshalExternal(externalBytes []byte) (EntryWithCategory, error) {
	externalEntry := externalEntry{}
	err := yaml.Unmarshal(externalBytes, &externalEntry)
	if err != nil {
		return EntryWithCategory{}, fmt.Errorf("failed to unmarshall external: %w", err)
	}

	return mapExternalEntry(externalEntry)
}

func mapExternalEntry(entry externalEntry) (EntryWithCategory, error) {
	if entry.DisplayName == "" {
		return EntryWithCategory{}, errors.New("could not find DisplayName on external entry")
	}
	if entry.URL == "" {
		return EntryWithCategory{}, errors.New("could not find URL on external entry")
	}
	if entry.Category == "" {
		return EntryWithCategory{}, errors.New("could not find Category on external entry")
	}
	return EntryWithCategory{
		Entry: Entry{
			DisplayName: entry.DisplayName,
			Title:       entry.Description,
			Href:        entry.URL,
			Target:      TARGET_EXTERNAL,
		},
		Category: entry.Category,
	}, nil
}
