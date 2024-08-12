package types

import (
	"encoding/json"
	"github.com/cloudogu/cesapp-lib/core"
	"testing"
)

func readRedmineDogu(t *testing.T) *core.Dogu {
	t.Helper()
	dogu := &core.Dogu{}
	err := json.Unmarshal(redmineBytes, dogu)
	if err != nil {
		t.Fatal(err.Error())
	}

	return dogu
}
