package types

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
)

type externalEntry struct {
	DisplayName string
	URL         string
	Description string
	Category    string
}

// EntryWithCategory is a dto for entries with a Category
type EntryWithCategory struct {
	Entry    Entry
	Category string
}

// ExternalConverter is used to read external links from the configuration and convert them to a warp menu category object.
type ExternalConverter struct{}

// ReadAndUnmarshalExternal reads a specific external link from the configuration and converts it to an entry with a category.
func (ec *ExternalConverter) ReadAndUnmarshalExternal(registry WatchConfigurationContext, key string) (EntryWithCategory, error) {
	externalBytes, err := readExternalAsBytes(registry, key)
	if err != nil {
		return EntryWithCategory{}, nil
	}

	return unmarshalExternal(externalBytes)
}

func readExternalAsBytes(registry WatchConfigurationContext, key string) ([]byte, error) {
	resp, err := registry.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read key %s from etcd: %w", key, err)
	}

	return []byte(resp), nil
}

func unmarshalExternal(externalBytes []byte) (EntryWithCategory, error) {
	externalEntry := externalEntry{}
	err := json.Unmarshal(externalBytes, &externalEntry)
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
