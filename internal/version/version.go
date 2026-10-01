// Package version reports the gdparser command's version.
package version

// Value is the version the gdparser command reports. Release builds replace it
// through -ldflags; the default covers local builds and `go install`.
var Value = "0.1.0-dev"
