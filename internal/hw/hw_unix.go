// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build !windows

package hw

func Detect() Info {
	return Recommend("", 0, 0, 0)
}
