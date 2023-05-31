package cesregistry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreate(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		result, err := Create("my-namespace")

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}
