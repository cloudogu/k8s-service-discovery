package warp

import (
	"encoding/json"
	"github.com/cloudogu/cesapp-lib/registry"
	coreosclient "github.com/coreos/etcd/client"
	"strings"

	"github.com/pkg/errors"
)

type doguEntry struct {
	Name        string
	DisplayName string
	Description string
	Category    string
	Tags        []string
}

func readAndUnmarshalDogu(registry registry.WatchConfigurationContext, key string, tag string) (EntryWithCategory, error) {
	doguBytes, err := readDoguAsBytes(registry, key)
	if err != nil {
		return EntryWithCategory{}, err
	}

	doguEntry, err := unmarshalDogu(doguBytes)
	if err != nil {
		return EntryWithCategory{}, err
	}

	if tag == "" || containsString(doguEntry.Tags, tag) {
		return mapDoguEntry(doguEntry)
	}

	return EntryWithCategory{}, nil
}

func readDoguAsBytes(registry registry.WatchConfigurationContext, key string) ([]byte, error) {
	resp, err := registry.Get(key + "/current")
	if err != nil {
		// the dogu seems to be unregistered
		if isKeyNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to read key %s from etcd", key)
	}

	version := resp
	resp, err = registry.Get(key + "/" + version)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read version child from key %s", key)
	}

	return []byte(resp), nil
}

func unmarshalDogu(doguBytes []byte) (doguEntry, error) {
	doguEntry := doguEntry{}
	err := json.Unmarshal(doguBytes, &doguEntry)
	if err != nil {
		return doguEntry, errors.Wrap(err, "failed to unmarshall json from etcd")
	}
	return doguEntry, nil
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

func isKeyNotFound(err error) bool {
	if cErr, ok := err.(coreosclient.Error); ok {
		return cErr.Code == coreosclient.ErrorCodeKeyNotFound
	}
	return false
}