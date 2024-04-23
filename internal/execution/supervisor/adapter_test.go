package supervisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAdapterFactory(t *testing.T) {
	cases := []IOInterface{FileIO, StdIO}
	for _, mode := range cases {
		_, err := defaultAdapterFactory[any, any, any](mode, nil)
		assert.NoError(t, err)
	}
}

func TestDefaultAdapterFactory_Fails(t *testing.T) {
	_, err := defaultAdapterFactory[any, any, any]("", nil)
	assert.ErrorIs(t, err, ErrUnsupportedIOMode)
}
