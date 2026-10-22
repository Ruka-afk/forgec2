// Package buildinfo carries facts about the build that are stamped into the
// binary at link time.
//
// The values here are plain vars so they can be overridden with
// -ldflags "-X github.com/forgec2/forgec2/internal/buildinfo.Repo=<owner>/<name>".
// The release workflow injects github.repository, so a fork publishes and
// updates against its own repository without a code change.
package buildinfo

// Repo is the GitHub repository that publishes the ForgeC2 releases. The
// update checker uses it as the default source for version checks and for
// the release page link the operator is shown.
//
// The default points at the upstream repository; a fork that tags its own
// releases overrides it at build time (see .github/workflows/release.yml).
var Repo = "r0si/forgec2"
