package types

import (
	"github.com/cloudogu/cesapp-lib/registry/mocks"
	coreosclient "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

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

func Test_isKeyNotFound(t *testing.T) {
	t.Run("return true on code key not found", func(t *testing.T) {
		// given
		err := coreosclient.Error{Code: coreosclient.ErrorCodeKeyNotFound}

		// when
		result := isKeyNotFound(err)

		// then
		assert.True(t, result)
	})

	t.Run("return false on wrong error type", func(t *testing.T) {
		// given
		err := assert.AnError

		// when
		result := isKeyNotFound(err)

		// then
		assert.False(t, result)
	})

	tests := []struct {
		name string
		key  int
		want bool
	}{
		{name: "error on !key not found", key: coreosclient.ErrorCodeDirNotEmpty, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeTestFailed, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeNotFile, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeNotDir, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeNodeExist, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeRootROnly, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeUnauthorized, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodePrevValueRequired, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeTTLNaN, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeIndexNaN, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeInvalidField, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeInvalidForm, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeRaftInternal, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeLeaderElect, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeWatcherCleared, want: false},
		{name: "error on !key not found", key: coreosclient.ErrorCodeEventIndexCleared, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := coreosclient.Error{Code: tt.key}
			assert.Equalf(t, tt.want, isKeyNotFound(err), "isKeyNotFound(%v)")
		})
	}
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

func Test_readAndUnmarshalDogu(t *testing.T) {
	doguEntry := doguEntry{Name: "official/redmine", DisplayName: "Redmine", Category: "Development Apps", Description: "Redmine"}
	doguStr := "{\n  \"Name\": \"official/redmine\",\n  \"Version\": \"1.0.0-1\",\n  \"DisplayName\": \"Redmine\",\n  \"Description\": \"Redmine\",\n  \"Category\": \"Development Apps\",\n  \"Tags\": [\n    \"warp\",\n    \"pm\",\n    \"projectmanagement\",\n    \"issue\",\n    \"task\"\n  ],\n  \"Logo\": \"https://cloudogu.com/images/dogus/redmine.png\",\n  \"Url\": \"http://www.redmine.org\",\n  \"Image\": \"registry.cloudogu.com/official/redmine\",\n  \"Dependencies\": [\n    {\n      \"type\": \"dogu\",\n      \"name\": \"postgresql\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"cas\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"nginx\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"postfix\"\n    }\n  ],\n  \"Configuration\": [\n    {\n      \"Name\": \"logging/root\",\n      \"Description\": \"Set the root log level to one of ERROR, WARN, INFO, DEBUG.\",\n      \"Optional\": true,\n      \"Default\": \"INFO\",\n      \"Validation\": {\n        \"Type\": \"ONE_OF\",\n        \"Values\": [\n          \"WARN\",\n          \"DEBUG\",\n          \"INFO\",\n          \"ERROR\"\n        ]\n      }\n    },\n    {\n      \"Name\": \"container_config/memory_limit\",\n      \"Description\": \"Limits the container's memory usage. Use a positive integer value followed by one of these units [b,k,m,g] (byte, kibibyte, mebibyte, gibibyte).\",\n      \"Optional\": true,\n      \"Validation\": {\n        \"Type\": \"BINARY_MEASUREMENT\"\n      }\n    },\n    {\n      \"Name\": \"container_config/swap_limit\",\n      \"Description\": \"Limits the container's swap memory usage. Use zero or a positive integer value followed by one of these units [b,k,m,g] (byte, kibibyte, mebibyte, gibibyte). 0 will disable swapping.\",\n      \"Optional\": true,\n      \"Validation\": {\n        \"Type\": \"BINARY_MEASUREMENT\"\n      }\n    },\n    {\n      \"Name\": \"etcd_redmine_config\",\n      \"Description\": \"Applies default configuration to redmine.\",\n      \"Optional\": true\n    }\n  ],\n  \"Volumes\": [\n    {\n      \"Name\": \"files\",\n      \"Path\": \"/usr/share/webapps/redmine/files\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": true\n    },\n    {\n      \"Name\": \"plugins\",\n      \"Path\": \"/var/tmp/redmine/plugins\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": false\n    },\n    {\n      \"Name\": \"logs\",\n      \"Path\": \"/usr/share/webapps/redmine/log\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": false\n    }\n  ],\n  \"ServiceAccounts\": [\n    {\n      \"Type\": \"postgresql\"\n    }\n  ],\n  \"HealthChecks\": [\n    {\n      \"Type\": \"tcp\",\n      \"Port\": 3000\n    },\n    {\n      \"Type\": \"state\"\n    }\n  ],\n  \"ExposedCommands\": [\n    {\n      \"Name\": \"post-upgrade\",\n      \"Command\": \"/post-upgrade.sh\"\n    },\n    {\n      \"Name\": \"upgrade-notification\",\n      \"Command\": \"/upgrade-notification.sh\"\n    },\n    {\n      \"Name\": \"pre-upgrade\",\n      \"Command\": \"/pre-upgrade.sh\"\n    },\n    {\n      \"Name\": \"delete-plugin\",\n      \"Command\": \"/delete-plugin.sh\"\n    }\n  ]\n}"
	t.Run("success with specific tag (warp)", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return(doguStr, nil)
		converter := DoguConverter{}

		// when
		entryWithCategory, err := converter.ReadAndUnmarshalDogu(registryMock, "dogu/redmine", "warp")

		// then
		require.NoError(t, err)
		assert.Equal(t, doguEntry.Category, entryWithCategory.Category)
		assert.Equal(t, doguEntry.Description, entryWithCategory.Entry.Title)
		assert.Equal(t, doguEntry.DisplayName, entryWithCategory.Entry.DisplayName)
		assert.Equal(t, TARGET_SELF, entryWithCategory.Entry.Target)
		assert.Equal(t, "/redmine", entryWithCategory.Entry.Href)
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("success without specific tag", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return(doguStr, nil)
		converter := DoguConverter{}

		// when
		entryWithCategory, err := converter.ReadAndUnmarshalDogu(registryMock, "dogu/redmine", "")

		// then
		require.NoError(t, err)
		assert.Equal(t, doguEntry.Category, entryWithCategory.Category)
		assert.Equal(t, doguEntry.Description, entryWithCategory.Entry.Title)
		assert.Equal(t, doguEntry.DisplayName, entryWithCategory.Entry.DisplayName)
		assert.Equal(t, TARGET_SELF, entryWithCategory.Entry.Target)
		assert.Equal(t, "/redmine", entryWithCategory.Entry.Href)
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("failed to read dogu as bytes", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("", assert.AnError)
		converter := DoguConverter{}

		// when
		_, err := converter.ReadAndUnmarshalDogu(registryMock, "dogu/redmine", "warp")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read key dogu/redmine/current from etcd")
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("failed to unmarshal dogu", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return("fdsfsdf", nil)
		converter := DoguConverter{}

		// when
		_, err := converter.ReadAndUnmarshalDogu(registryMock, "dogu/redmine", "warp")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshall json from etcd")
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("return empty entry with category if the tag is not found", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return(doguStr, nil)
		converter := DoguConverter{}

		// when
		entryWithCategory, err := converter.ReadAndUnmarshalDogu(registryMock, "dogu/redmine", "dontbethere")

		// then
		require.NoError(t, err)
		assert.Equal(t, EntryWithCategory{}, entryWithCategory)
		mock.AssertExpectationsForObjects(t, registryMock)
	})
}

func Test_readDoguAsBytes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		doguStr := "{\n  \"Name\": \"official/redmine\",\n  \"Version\": \"1.0.0-1\",\n  \"DisplayName\": \"Redmine\",\n  \"Description\": \"Redmine is a flexible project management web application\",\n  \"Category\": \"Development Apps\",\n  \"Tags\": [\n    \"warp\",\n    \"pm\",\n    \"projectmanagement\",\n    \"issue\",\n    \"task\"\n  ],\n  \"Logo\": \"https://cloudogu.com/images/dogus/redmine.png\",\n  \"Url\": \"http://www.redmine.org\",\n  \"Image\": \"registry.cloudogu.com/official/redmine\",\n  \"Dependencies\": [\n    {\n      \"type\": \"dogu\",\n      \"name\": \"postgresql\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"cas\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"nginx\"\n    },\n    {\n      \"type\": \"dogu\",\n      \"name\": \"postfix\"\n    }\n  ],\n  \"Configuration\": [\n    {\n      \"Name\": \"logging/root\",\n      \"Description\": \"Set the root log level to one of ERROR, WARN, INFO, DEBUG.\",\n      \"Optional\": true,\n      \"Default\": \"INFO\",\n      \"Validation\": {\n        \"Type\": \"ONE_OF\",\n        \"Values\": [\n          \"WARN\",\n          \"DEBUG\",\n          \"INFO\",\n          \"ERROR\"\n        ]\n      }\n    },\n    {\n      \"Name\": \"container_config/memory_limit\",\n      \"Description\": \"Limits the container's memory usage. Use a positive integer value followed by one of these units [b,k,m,g] (byte, kibibyte, mebibyte, gibibyte).\",\n      \"Optional\": true,\n      \"Validation\": {\n        \"Type\": \"BINARY_MEASUREMENT\"\n      }\n    },\n    {\n      \"Name\": \"container_config/swap_limit\",\n      \"Description\": \"Limits the container's swap memory usage. Use zero or a positive integer value followed by one of these units [b,k,m,g] (byte, kibibyte, mebibyte, gibibyte). 0 will disable swapping.\",\n      \"Optional\": true,\n      \"Validation\": {\n        \"Type\": \"BINARY_MEASUREMENT\"\n      }\n    },\n    {\n      \"Name\": \"etcd_redmine_config\",\n      \"Description\": \"Applies default configuration to redmine.\",\n      \"Optional\": true\n    }\n  ],\n  \"Volumes\": [\n    {\n      \"Name\": \"files\",\n      \"Path\": \"/usr/share/webapps/redmine/files\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": true\n    },\n    {\n      \"Name\": \"plugins\",\n      \"Path\": \"/var/tmp/redmine/plugins\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": false\n    },\n    {\n      \"Name\": \"logs\",\n      \"Path\": \"/usr/share/webapps/redmine/log\",\n      \"Owner\": \"1000\",\n      \"Group\": \"1000\",\n      \"NeedsBackup\": false\n    }\n  ],\n  \"ServiceAccounts\": [\n    {\n      \"Type\": \"postgresql\"\n    }\n  ],\n  \"HealthChecks\": [\n    {\n      \"Type\": \"tcp\",\n      \"Port\": 3000\n    },\n    {\n      \"Type\": \"state\"\n    }\n  ],\n  \"ExposedCommands\": [\n    {\n      \"Name\": \"post-upgrade\",\n      \"Command\": \"/post-upgrade.sh\"\n    },\n    {\n      \"Name\": \"upgrade-notification\",\n      \"Command\": \"/upgrade-notification.sh\"\n    },\n    {\n      \"Name\": \"pre-upgrade\",\n      \"Command\": \"/pre-upgrade.sh\"\n    },\n    {\n      \"Name\": \"delete-plugin\",\n      \"Command\": \"/delete-plugin.sh\"\n    }\n  ]\n}"
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return(doguStr, nil)

		// when
		bytes, err := readDoguAsBytes(registryMock, "dogu/redmine")

		// then
		require.NoError(t, err)
		assert.Equal(t, []byte(doguStr), bytes)
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("no version should not return an error", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		testErr := coreosclient.Error{Code: coreosclient.ErrorCodeKeyNotFound}
		registryMock.On("Get", "dogu/redmine/current").Return("", testErr)

		// when
		bytes, err := readDoguAsBytes(registryMock, "dogu/redmine")

		// then
		require.NoError(t, err)
		require.Nil(t, bytes)
		require.Nil(t, err)
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("no version should not return an error", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		testErr := coreosclient.Error{Code: coreosclient.ErrorCodeKeyNotFound}
		registryMock.On("Get", "dogu/redmine/current").Return("", testErr)

		// when
		bytes, err := readDoguAsBytes(registryMock, "dogu/redmine")

		// then
		require.NoError(t, err)
		require.Nil(t, bytes)
		require.Nil(t, err)
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("failed to get version", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("", assert.AnError)

		// when
		_, err := readDoguAsBytes(registryMock, "dogu/redmine")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read key dogu/redmine/current from etcd")
		mock.AssertExpectationsForObjects(t, registryMock)
	})

	t.Run("failed to get dogu with version", func(t *testing.T) {
		// given
		registryMock := &mocks.WatchConfigurationContext{}
		registryMock.On("Get", "dogu/redmine/current").Return("1.0.0-1", nil)
		registryMock.On("Get", "dogu/redmine/1.0.0-1").Return("", assert.AnError)

		// when
		_, err := readDoguAsBytes(registryMock, "dogu/redmine")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read dogu/redmine with version 1.0.0-1")
		mock.AssertExpectationsForObjects(t, registryMock)
	})
}

func Test_unmarshalDogu(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		doguJsonStr := "{\n  \"Name\": \"redmine\",\n  \"DisplayName\": \"display\",\n  \"Description\": \"desc\",\n  \"Category\": \"category\",\n  \"Tags\": [\n    \"warp\",\n    \"test\"\n  ]\n}"

		// when
		result, err := unmarshalDogu([]byte(doguJsonStr))

		// then
		require.NoError(t, err)
		assert.Equal(t, "redmine", result.Name)
		assert.Equal(t, "display", result.DisplayName)
		assert.Equal(t, "desc", result.Description)
		assert.Equal(t, "category", result.Category)
		assert.Equal(t, "warp", result.Tags[0])
		assert.Equal(t, "test", result.Tags[1])
	})

	t.Run("fail on wrong struct", func(t *testing.T) {
		// given
		doguJsonStr := "fsdfsd"

		// when
		_, err := unmarshalDogu([]byte(doguJsonStr))

		// then
		require.Error(t, err)
	})
}
