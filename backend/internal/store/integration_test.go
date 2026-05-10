//go:build integration

package store

// TODO: integration tests are reserved for the //go:build integration tag.
// They will boot a real Postgres via testcontainers-go/postgres and run the
// migrations from ../../migrations before exercising the repos. Skipped in
// the default test run because containers add startup time we don't want
// in the dev loop. Wire up when CI is in place.
