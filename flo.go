// Package flo provides a high-level way to use the Fluxer API.
package flo

import "runtime/debug"

func libVersion() string {
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

}

// DefaultUserAgent is the default user agent used for REST requests and the browser string when connecting to the gateway.
var DefaultUserAgent = func() string {
	version := libVersion()
	if version == "" {
		version = "unknown"
	}

	return "flo/" + version
}()
