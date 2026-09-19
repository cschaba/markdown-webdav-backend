#!/usr/bin/env bash
# Tags the version that version.go names and pushes the tag; the tag starts the
# release workflow, which builds, tests and publishes the archives.
#
#   tools/release.sh --dry-run   # every check, and what would be done
#   tools/release.sh
#
# Before it: set the version in version.go, move the changelog's [Unreleased]
# entries under a heading for it, commit, and push main. The version is written
# in version.go and nowhere else.
set -euo pipefail
cd "$(dirname "$0")/.."

dry_run=false
case ${1:-} in
    --dry-run) dry_run=true ;;
    "") ;;
    *) echo "usage: tools/release.sh [--dry-run]" >&2; exit 2 ;;
esac

die() { echo "release: $*" >&2; exit 1; }

version=$(go run . -version)
tag="v$version"
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version.go says '$version', which is not MAJOR.MINOR.PATCH"

branch=$(git symbolic-ref --short HEAD)
[[ $branch == "main" ]] || die "on branch $branch; releases are made from main"
[[ -z $(git status --porcelain) ]] || die "the working tree has changes; commit them first"
if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    die "tag $tag exists already; raise the version in version.go"
fi
awk -v v="$version" '$1 == "##" && $2 == "[" v "]" { found = 1 } END { exit found ? 0 : 1 }' CHANGELOG.md \
    || die "CHANGELOG.md has no section [$version]"

git fetch -q origin main
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || die "main differs from origin/main; push (or pull) first - CI should have seen what is released"

echo "release: $tag from $(git rev-parse --short HEAD); running the tests"
go vet ./...
go test ./...
[[ -z $(gofmt -l .) ]] || die "gofmt -l . is not empty"

if $dry_run; then
    echo "release: dry run - would run: git tag -a $tag -m $tag && git push origin $tag"
else
    git tag -a "$tag" -m "$tag"
    git push origin "$tag"
    echo "release: pushed $tag; the workflow publishes it: gh run watch, or the Actions page"
fi
