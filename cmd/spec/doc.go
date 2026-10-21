// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Command spec vendors the conformance definitions of polyglot libraries, keeps their copies in
// sync, and verifies that each runner runs every case.
//
// The command runs go.stealthscale.io/spec/internal/command with the arguments, the environment
// and the standard streams of the process. It sends the requests to GitHub through a transport of
// its own, with the proxy of the environment. SIGINT and SIGTERM cancel a run.
//
// # Exit statuses
//
//   - 0 for a run that succeeds.
//   - 1 for a check that fails, and for every error without another status.
//   - 2 for an invalid command line or configuration file, and for a run without a token of GitHub.
//   - 3 for a request to GitHub that fails.
//
// # Dependency position
//
// Imports context, net, net/http, os, os/signal, runtime/debug, syscall and time from the standard
// library, go.stealthscale.io/spec/internal/command from this module, and go.stealthscale.io/cli.
package main
