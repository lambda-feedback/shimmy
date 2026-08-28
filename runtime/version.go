package runtime

var SupportedMuEdVersions = []string{"0.1.0"}

func MuEdIsVersionSupported(version string) bool {
	for _, v := range SupportedMuEdVersions {
		if v == version {
			return true
		}
	}
	return false
}

func MuEdResolveVersion(requested string) string {
	if MuEdIsVersionSupported(requested) {
		return requested
	}
	return SupportedMuEdVersions[len(SupportedMuEdVersions)-1]
}
