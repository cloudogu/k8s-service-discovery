package types

import "github.com/pkg/errors"

// Entry link in the warp menu
type Entry struct {
	DisplayName string
	Href        string
	Title       string
	Target      Target
}

// Target defines the target of the link
type Target uint8

const (
	// TARGET_SELF means the link is part of the internal system
	TARGET_SELF Target = iota + 1
	// TARGET_EXTERNAL link is outside from the system
	TARGET_EXTERNAL
)

func (target Target) MarshalJSON() ([]byte, error) {
	switch target {
	case TARGET_SELF:
		return target.asJSONString("self"), nil
	case TARGET_EXTERNAL:
		return target.asJSONString("external"), nil
	default:
		return nil, errors.Errorf("unknow target type %d", target)
	}
}

func (target Target) asJSONString(value string) []byte {
	return []byte("\"" + value + "\"")
}

// Entries is a collection of warp entries
type Entries []Entry

func (e Entries) Len() int {
	return len(e)
}

func (e Entries) Less(i, j int) bool {
	return e[i].DisplayName < e[j].DisplayName
}

func (e Entries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}
