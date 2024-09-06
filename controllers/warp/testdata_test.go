package warp

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

func readJenkinsDogu(t *testing.T) *core.Dogu {
	t.Helper()
	dogu := &core.Dogu{}
	err := json.Unmarshal(jenkinsBytes, dogu)
	if err != nil {
		t.Fatal(err.Error())
	}

	return dogu
}
func parseVersion(t *testing.T, version string) *core.Version {
	t.Helper()
	v, err := core.ParseVersion(version)
	if err != nil {
		t.Fatal(err.Error())
	}

	return &v
}
