package warp

import (
	"github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfigReader_readSupport(t *testing.T) {
	reader := &ConfigReader{
		configuration: &Configuration{Support: []SupportSource{}},
		registry:      nil,
	}

	supportSources := []SupportSource{{Identifier: "aboutCloudoguToken", External: false, Href: "/local/href"}, {Identifier: "myCloudogu", External: true, Href: "https://ecosystem.cloudogu.com/"}, {Identifier: "docsCloudoguComUrl", External: true, Href: "https://docs.cloudogu.com/"}}

	actual, err := reader.readSupport(supportSources, []string{"docsCloudoguComUrl"})

	if err != nil {
		t.Fail()
	}

	expectedCategories := Categories{{Title: "Support", Entries: []Entry{
		{Title: "aboutCloudoguToken", Target: TARGET_SELF, Href: "/local/href"},
		{Title: "myCloudogu", Target: TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"}}}}

	assert.Equal(t, expectedCategories, actual, "readSupport did not return the correct Category of two entries")

	// test with empty filter
	actual, err = reader.readSupport(supportSources, []string{})
	expectedCategories = Categories{
		{Title: "Support", Entries: []Entry{
			{Title: "aboutCloudoguToken", Target: TARGET_SELF, Href: "/local/href"},
			{Title: "myCloudogu", Target: TARGET_EXTERNAL, Href: "https://ecosystem.cloudogu.com/"},
			{Title: "docsCloudoguComUrl", Target: TARGET_EXTERNAL, Href: "https://docs.cloudogu.com/"}}}}

	// test with complete filter
	actual, err = reader.readSupport(supportSources, []string{"myCloudogu", "aboutCloudoguToken", "docsCloudoguComUrl"})
	expectedCategories = Categories{}

	assert.Equal(t, 0, expectedCategories.Len())
	assert.Equal(t, expectedCategories, actual, "readSupport did not return the correct Category of three entries")
}

func TestConfigReader_getDisabledSupportIdentifiers(t *testing.T) {
	mockRegistry := &mocks.WatchConfigurationContext{}
	mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)

	reader := &ConfigReader{
		configuration: &Configuration{Support: []SupportSource{}},
		registry:      mockRegistry,
	}

	identifiers, err := reader.getDisabledSupportIdentifiers()
	assert.Empty(t, err)
	assert.Equal(t, []string{"lorem", "ipsum"}, identifiers)
}

func TestConfigReader_readFromConfig(t *testing.T) {
	mockRegistry := &mocks.WatchConfigurationContext{}
	mockRegistry.On("GetChildrenPaths", "/path/to/etcd/key").Return([]string{"/path/to/etcd/key"}, nil)
	mockRegistry.On("Get", "/path/to/etcd/key").Return("", nil)
	mockRegistry.On("Get", "/config/_global/disabled_warpmenu_support_entries").Return("[\"lorem\", \"ipsum\"]", nil)

	reader := &ConfigReader{
		configuration: &Configuration{Support: []SupportSource{}},
		registry:      mockRegistry,
	}

	testSources := []Source{{Path: "/path/to/etcd/key", Type: "externals", Tag: "tag"}}
	testSupportSoureces := []SupportSource{{Identifier: "supportSrc", External: true, Href: "path/to/external"}}

	actual, err := reader.readFromConfig(&Configuration{Sources: testSources, Support: testSupportSoureces})

	assert.Empty(t, err)
	assert.NotEmpty(t, actual)
}
