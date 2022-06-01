package warp

import (
	"encoding/json"
	"github.com/cloudogu/cesapp-lib/registry"

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

func readAndUnmarshalExternal(registry registry.WatchConfigurationContext, key string) (EntryWithCategory, error) {
	externalBytes, err := readExternalAsBytes(registry, key)
	if err != nil {
		return EntryWithCategory{}, nil
	}

	return unmarshalExternal(externalBytes)
}

func readExternalAsBytes(registry registry.WatchConfigurationContext, key string) ([]byte, error) {
	resp, err := registry.Get(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read key %s from etcd", key)
	}

	return []byte(resp), nil
}

func unmarshalExternal(externalBytes []byte) (EntryWithCategory, error) {
	externalEntry := externalEntry{}
	err := json.Unmarshal(externalBytes, &externalEntry)
	if err != nil {
		return EntryWithCategory{}, errors.Wrap(err, "failed to unmarshall external")
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
