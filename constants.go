package flo

import (
	"net/url"
	"runtime/debug"
)

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

var defaultAPIURL = func() *url.URL {
	url, err := url.Parse("https://api.fluxer.app/")
	if err != nil {
		panic(err)
	}

	return url
}()
