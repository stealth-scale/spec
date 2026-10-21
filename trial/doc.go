// Copyright Stealth Scale B.V. 2026
// SPDX-License-Identifier: MIT

// Package trial tries the working tree of a definition in the checkouts of its implementations.
//
// [Try] writes the copy of the working tree into the checkout of each implementation that the
// definition registers, and runs the test of its entry there. The checkouts are the directories
// beside the definition with the names of their repositories, such as ../assert-go for
// dokimasia/assert-go. Try first moves the copy of each checkout aside, and [Restore] moves it
// back, so a trial ends with the checkouts as they were. A trial needs no network.
//
// # Errors
//
// A checkout that fails has its error in [Row.Err], and the other checkouts still run.
//
// # Dependency position
//
// Imports context, errors, fmt, io, os, os/exec and path/filepath from the standard library, and
// go.stealthscale.io/spec, go.stealthscale.io/spec/definition,
// go.stealthscale.io/spec/implementation and go.stealthscale.io/spec/vendored from this module.
package trial
