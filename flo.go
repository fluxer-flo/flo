// Package flo provides a high-level way to use the Fluxer API.
package flo

import "runtime/debug"

var libVersion = func() string {
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	for _, dep := range build.Deps {
		if dep.Path == "github.com/fluxer-flo/flo" {
			return dep.Version
		}
	}

	return ""

}()

var defaultUserAgent = func() string {
	version := libVersion
	if version == "" {
		version = "unknown"
	}

	return "flo/" + version
}()
