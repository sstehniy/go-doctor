# Go Doctor

`go-doctor` is a Go code health checker for Go repositories.

Its goal is simple: give Go engineers one command that scans a repo, highlights meaningful issues, and returns an easy-to-understand health signal that can be used both locally and in CI.

## Core Idea

Instead of relying on a scattered set of separate tools, `go-doctor` is intended to combine useful checks into one consistent experience:

- find reliability and maintainability issues
- surface actionable findings in one place
- provide a simple repo health score
- support local development and CI workflows

The focus is on practical, high-signal issues that help teams catch problems early, not style debates or risky auto-fixes.

## What It Is For

`go-doctor` is being designed for:

- Go services
- Go libraries
- Go CLIs
- single-module and multi-module repos

The intended user is a Go engineer who wants fast, useful feedback before problems reach production.

## What It Is Not

This project is not meant to be:

- a hosted platform
- an IDE plugin
- a code rewriting tool
- a replacement for every linter
