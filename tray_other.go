// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build !windows

package main

type TrayManager struct{}

func initTray(a *App) *TrayManager {
	return &TrayManager{}
}

func updateTrayStatus(tip string) {}

func cleanupTray() {}
