package download

import (
	"testing"
)

func TestLatestLlamaZipURL(t *testing.T) {
	flavors := []string{"vulkan", "cuda", "rocm", "sycl", "cpu"}
	for _, f := range flavors {
		url, cudart, err := latestLlamaZipURL(f)
		if err != nil {
			t.Errorf("flavor %s failed: %v", f, err)
			continue
		}
		if url == "" {
			t.Errorf("flavor %s returned empty url", f)
		}
		t.Logf("Flavor %s: url=%s cudart=%s", f, url, cudart)
	}
}
