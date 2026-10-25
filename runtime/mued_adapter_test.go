package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lambda-feedback/shimmy/runtime"
)

// fakeAdapter is a minimal MuEdAdapter used to exercise multi-version registry
// behaviour without a second real µEd version.
type fakeAdapter struct{ version string }

func (a fakeAdapter) Version() string { return a.version }
func (a fakeAdapter) DecodeEvaluate([]byte) (map[string]any, runtime.Command, error) {
	return map[string]any{"from": a.version}, runtime.CommandEvaluate, nil
}
func (a fakeAdapter) EncodeEvaluateFeedback(runtime.Command, map[string]any) ([]map[string]any, error) {
	return []map[string]any{{"from": a.version}}, nil
}
func (a fakeAdapter) EncodeHealth(map[string]any) map[string]any {
	return map[string]any{"from": a.version}
}
func (a fakeAdapter) DecodeChat([]byte) (map[string]any, error) {
	return map[string]any{"from": a.version}, nil
}
func (a fakeAdapter) EncodeChat(map[string]any) (map[string]any, error) {
	return map[string]any{"from": a.version}, nil
}
func (a fakeAdapter) EncodeChatHealth(map[string]any) map[string]any {
	return map[string]any{"from": a.version}
}

func TestMuEdRegistry_OrderAndResolution(t *testing.T) {
	reg := runtime.NewMuEdRegistry()
	reg.Register(fakeAdapter{version: "0.1.0"})
	reg.Register(fakeAdapter{version: "0.2.0"})
	reg.Register(fakeAdapter{version: "0.3.0"})

	assert.Equal(t, []string{"0.1.0", "0.2.0", "0.3.0"}, reg.Versions())
	assert.Equal(t, "0.1.0", reg.Default(), "default is the first registered version, pinned")
	assert.Equal(t, "0.3.0", reg.Latest())

	assert.True(t, reg.Supports("0.2.0"))
	assert.False(t, reg.Supports("9.9.9"))

	assert.Equal(t, "0.1.0", reg.Resolve(""), "empty request resolves to the pinned default")
	assert.Equal(t, "0.2.0", reg.Resolve("0.2.0"), "supported request resolves to itself")
	assert.Equal(t, "0.3.0", reg.Resolve("9.9.9"), "unsupported request resolves to latest")
}

func TestMuEdRegistry_Adapter(t *testing.T) {
	reg := runtime.NewMuEdRegistry()
	reg.Register(fakeAdapter{version: "0.1.0"})
	reg.Register(fakeAdapter{version: "9.9.9"})

	got := reg.Adapter("9.9.9")
	require.NotNil(t, got)
	assert.Equal(t, "9.9.9", got.Version())

	feedback, err := got.EncodeEvaluateFeedback(runtime.CommandEvaluate, nil)
	require.NoError(t, err)
	require.Len(t, feedback, 1)
	assert.Equal(t, "9.9.9", feedback[0]["from"])

	assert.Nil(t, reg.Adapter("0.5.0"), "unknown version has no adapter")
}

func TestMuEdRegistry_ReregisterKeepsPosition(t *testing.T) {
	reg := runtime.NewMuEdRegistry()
	reg.Register(fakeAdapter{version: "0.1.0"})
	reg.Register(fakeAdapter{version: "0.2.0"})
	reg.Register(fakeAdapter{version: "0.1.0"}) // replace, don't reorder

	assert.Equal(t, []string{"0.1.0", "0.2.0"}, reg.Versions())
}

func TestDefaultMuEdRegistry_HasV010(t *testing.T) {
	reg := runtime.DefaultMuEdRegistry()

	assert.Equal(t, []string{"0.1.0"}, reg.Versions())
	assert.Equal(t, []string{"0.1.0"}, runtime.SupportedMuEdVersions())
	assert.True(t, runtime.MuEdIsVersionSupported("0.1.0"))
	assert.False(t, runtime.MuEdIsVersionSupported("99.0.0"))

	assert.Equal(t, "0.1.0", runtime.MuEdResolveVersion(""))
	assert.Equal(t, "0.1.0", runtime.MuEdResolveVersion("0.1.0"))
	assert.Equal(t, "0.1.0", runtime.MuEdResolveVersion("99.0.0"))

	require.NotNil(t, reg.Adapter("0.1.0"))
	assert.Equal(t, "0.1.0", reg.Adapter("0.1.0").Version())
}
