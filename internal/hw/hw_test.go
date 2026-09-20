// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

package hw

import "testing"

func TestRecommendNVIDIA32K(t *testing.T) {
	i := Recommend("NVIDIA GeForce RTX 4070", 12<<30, 32<<30, 16<<30)
	if i.Backend != "cuda" || i.Context != 32768 || i.GPULayers != 99 {
		t.Fatalf("%+v", i)
	}
}

func TestRecommendRadeonVulkan(t *testing.T) {
	i := Recommend("AMD Radeon 860M", 0, 64<<30, 20<<30)
	if i.Backend != "vulkan" || i.Context != 32768 {
		t.Fatalf("%+v", i)
	}
}

func TestRecommendLowRAM(t *testing.T) {
	i := Recommend("Intel UHD Graphics", 1<<30, 8<<30, 2<<30)
	if i.Context != 8192 {
		t.Fatalf("want 8192 got %d", i.Context)
	}
}
