package runtime

// SupportedMuEdVersions returns the µEd API versions this build supports, in
// registration order (oldest first). Backed by DefaultMuEdRegistry.
func SupportedMuEdVersions() []string {
	return defaultMuEdRegistry.Versions()
}

// MuEdIsVersionSupported reports whether the given µEd API version is supported.
func MuEdIsVersionSupported(version string) bool {
	return defaultMuEdRegistry.Supports(version)
}

// MuEdResolveVersion maps a requested µEd API version to a concrete supported
// one: the default version when requested is empty, the request itself when
// supported, otherwise the latest supported version.
func MuEdResolveVersion(requested string) string {
	return defaultMuEdRegistry.Resolve(requested)
}
