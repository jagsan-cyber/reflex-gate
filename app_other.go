// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build !windows

package main

import "os/exec"

func setHideWindow(cmd *exec.Cmd) {}
