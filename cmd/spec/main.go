// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec/internal/command"
)

// The bounds of a connection to GitHub.
const (
	dialTimeout         = 30 * time.Second
	keepAlive           = 30 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
	idleConnTimeout     = 90 * time.Second
	maxIdleConns        = 10
)

func main() {
	os.Exit(run())
}

// run runs the command line of the process, and returns its exit status. os.Interrupt and
// syscall.SIGTERM cancel the run, and main exits after the deferred calls of run return. On
// Windows, Ctrl-C and Ctrl-Break deliver os.Interrupt, and the close, logoff and shutdown events of
// the console deliver syscall.SIGTERM.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	//dokimi:mutate-skip sbr-delete: main exits as soon as run returns, so a caller cannot observe the release of the signals
	defer stop()
	// A process without the information of its build gets nil, which the program writes as the
	// version (devel).
	info, _ := debug.ReadBuildInfo()
	program := &command.Program{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        maxIdleConns,
			IdleConnTimeout:     idleConnTimeout,
			TLSHandshakeTimeout: tlsHandshakeTimeout,
		},
		Build: info,
	}
	return program.Main(ctx, cli.OS())
}
