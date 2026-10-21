// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"go.dokimi.dev/ergon/service/forge"
	"go.stealthscale.io/cli"
	"go.stealthscale.io/spec"
	"go.stealthscale.io/spec/definition"
	"go.stealthscale.io/spec/github"
	"go.stealthscale.io/spec/implementation"
	"go.thesmos.sh/core/errs"
)

// name is the name of the program.
const name = "spec"

// help is the help text of the program.
const help = "Vendor the conformance definitions of polyglot libraries, keep the copies in sync, and " +
	"verify that each runner runs every case."

// ExitGitHub is the exit status of a run that needs GitHub and fails to read it or write to it.
const ExitGitHub = 3

// develVersion is the version of a build without the information of its build, as the go command
// writes the version of a build of a working tree.
const develVersion = "(devel)"

// requestTimeout bounds each request of a run to GitHub, the download of an archive included.
const requestTimeout = 5 * time.Minute

// The modes of the files and the directories that the program writes.
const (
	fileMode os.FileMode = 0o644
	dirMode  os.FileMode = 0o755
)

// The cells of the tables that the commands write.
const (
	// shortCommit is the number of the hexadecimal digits of a commit in a table.
	shortCommit = 7

	// none is the cell of a value that a row does not have.
	none = "-"

	// pullMark starts the cell of the number of a pull request.
	pullMark = "#"

	// The cells of a flag.
	yes = "yes"
	no  = "no"
)

// The headings of the columns that two or more tables have.
const (
	implementationColumn = "IMPLEMENTATION"
	definitionColumn     = "DEFINITION"
	versionColumn        = "VERSION"
	commitColumn         = "COMMIT"
	pullColumn           = "PULL"
)

// copyHeader is the header of the table of the copies that the commands sync and check write.
var copyHeader = []string{"COPY", definitionColumn, versionColumn, commitColumn}

// Global is the global options of the program.
type Global struct {
	// Dir is the directory of the repository of a run.
	Dir string `help:"The directory of the repository." default:"." short:"C"`

	// Token authorizes the requests to GitHub.
	Token string `help:"The token of the GitHub API." env:"GITHUB_TOKEN" secret:""`

	// API is the address of the REST API of GitHub, or empty for api.github.com.
	API string `help:"The address of the REST API of GitHub." env:"GITHUB_API_URL"`

	// GraphQL is the address of the GraphQL API of GitHub, or empty for api.github.com.
	GraphQL string `help:"The address of the GraphQL API of GitHub." env:"GITHUB_GRAPHQL_URL"`
}

// Program is the program spec: the commands that vendor a definition, sync its copies and verify
// the reports of its runners.
//
// # Exit statuses
//
//   - 0 for a run that succeeds.
//   - 1 for a check that fails, and for every error without another status.
//   - 2 for an invalid command line, for an error classified as errs.Invalid, such as a
//     configuration file that breaks a rule, and for a run without a token of GitHub.
//   - [ExitGitHub] for a request to GitHub that fails.
//
// # Concurrency
//
// Main is safe for concurrent use.
type Program struct {
	// Transport sends the requests of the program to GitHub.
	Transport http.RoundTripper

	// Build is the information of the build of the program, whose version the flag --version
	// writes after the name of the program, or nil for a build without one.
	Build *debug.BuildInfo
}

// Main runs the command line of env, writes the failure of the run to the standard error of env,
// and returns the exit status of the run, as [Program] states.
func (p *Program) Main(ctx context.Context, env *cli.Env) int {
	app := cli.New[Global](name, help)
	version := develVersion
	if p.Build != nil {
		version = p.Build.Main.Version
	}
	app.Version(name + " " + version)
	app.Command("manifest", manifestHelp, runManifest)
	app.Command("sync", syncHelp, p.runSync)
	app.Command("check", checkHelp, p.runCheck)
	app.Command("verify", verifyHelp, runVerify)
	app.Command("propose", proposeHelp, p.runPropose)
	app.Command("status", statusHelp, p.runStatus)
	app.Command("try", tryHelp, runTry)
	r, err := app.ExecuteRenderer(ctx, env)
	f := app.Failure(err)
	if f == nil {
		return cli.ExitOK
	}
	if env != nil && env.Stderr != nil {
		// The status reports the error also when the failure fails to write. Plain, the renderer of
		// the program, reads no environment.
		_ = r.Error(env.Stderr, nil, f)
	}
	return exitStatus(err)
}

// client returns the client of the GitHub API that the global options g configure.
//
// Error modes:
//   - the error of github.New, which wraps forge.ErrToken for options without a token.
func (p *Program) client(g *Global) (*github.Client, error) {
	c, err := github.New(
		&http.Client{Transport: p.Transport, Timeout: requestTimeout},
		github.Config{Token: g.Token, API: g.API, GraphQL: g.GraphQL},
	)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	return c, nil
}

// exitStatus returns the exit status of the error err of a run, as [Program] states.
func exitStatus(err error) int {
	if _, usage := errors.AsType[*cli.Error](err); usage {
		return cli.ExitUsage
	}
	if errs.Classify(err) == errs.Invalid || errors.Is(err, forge.ErrToken) {
		return cli.ExitUsage
	}
	if errors.Is(err, github.ErrGitHub) || errors.Is(err, forge.ErrGitHub) {
		return ExitGitHub
	}
	return cli.ExitFailure
}

// inRoot opens the directory dir as a root, returns the error of f with that root, and closes the
// root.
//
// Error modes:
//   - the error of the file system for a directory that cannot be opened or closed.
//   - the error of f.
func inRoot(dir string, f func(root *os.Root) error) (err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("command: open the repository %s: %w", dir, err)
	}
	//dokimi:mutate-skip sbr-delete: the close of an open root does not fail, and a caller cannot observe a root that is not closed
	defer func() { err = errors.Join(err, root.Close()) }()
	return f(root)
}

// readEntries returns the entries of the section implements of the configuration file of the
// repository at root.
//
// Error modes:
//   - the error of the file system for a configuration file that cannot be read.
//   - the error of implementation.Parse.
func readEntries(root *os.Root) ([]implementation.Entry, error) {
	data, err := root.ReadFile(spec.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("command: read %s: %w", spec.ConfigFile, err)
	}
	entries, err := implementation.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("command: %w", err)
	}
	return entries, nil
}

// readDefinition returns the section definition of the configuration file of the repository at
// dir.
//
// Error modes:
//   - the error of the file system for a repository or a configuration file that cannot be read.
//   - the error of definition.Parse.
func readDefinition(dir string) (definition.Config, error) {
	var c definition.Config
	err := inRoot(dir, func(root *os.Root) error {
		data, err := root.ReadFile(spec.ConfigFile)
		if err != nil {
			return fmt.Errorf("command: read %s: %w", spec.ConfigFile, err)
		}
		if c, err = definition.Parse(data); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		return nil
	})
	return c, err
}

// table writes header and rows to w in columns that two spaces separate, in one write.
//
// Error modes:
//   - an error that wraps the error of w.
func table(w io.Writer, header []string, rows [][]string) error {
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, row := range append([][]string{header}, rows...) {
		// A tabwriter returns the errors of its writer alone, and a bytes.Buffer returns none.
		_, _ = io.WriteString(tw, strings.Join(row, "\t")+"\n")
	}
	_ = tw.Flush()
	if _, err := w.Write(b.Bytes()); err != nil {
		return fmt.Errorf("command: write the table: %w", err)
	}
	return nil
}

// short returns the first digits of commit for a table, and [none] for an empty commit.
func short(commit string) string {
	if commit == "" {
		return none
	}
	return commit[:min(len(commit), shortCommit)]
}

// pullCell returns the cell of the pull request number for a table, and [none] for 0.
func pullCell(number int) string {
	if number == 0 {
		return none
	}
	return pullMark + strconv.Itoa(number)
}

// flagCell returns the cell of the flag set for a table.
func flagCell(set bool) string {
	if set {
		return yes
	}
	return no
}
