package download

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	GGUFURL    = "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q8_0.gguf?download=true"
	GGUFName   = "qwen2.5-coder-1.5b-instruct-q8_0.gguf"
	ReleaseAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=5"
)

type Progress func(label string, done, total int64)

func FetchAll(baseDir, backend string, prog Progress) (llamaExe, modelPath string, err error) {
	modelDir := filepath.Join(baseDir, "models")
	binDir := filepath.Join(baseDir, "bin")
	if err = os.MkdirAll(modelDir, 0o755); err != nil {
		return
	}
	if err = os.MkdirAll(binDir, 0o755); err != nil {
		return
	}
	modelPath = filepath.Join(modelDir, GGUFName)
	if fi, statErr := os.Stat(modelPath); statErr == nil && fi.Size() > 100*1024*1024 {
		if prog != nil {
			prog("GGUF (既存確認済み)", fi.Size(), fi.Size())
		}
	} else {
		if prog != nil {
			prog("GGUF", 0, 1)
		}
		if err = fetchFile(GGUFURL, modelPath, func(d, t int64) {
			if prog != nil {
				prog("GGUF", d, t)
			}
		}); err != nil {
			return
		}
	}

	normBackend := strings.ToLower(backend)
	if normBackend == "" || normBackend == "auto" {
		normBackend = "vulkan"
	}

	extractDir := filepath.Join(binDir, "llama.cpp-"+normBackend)
	if found, findErr := findExe(extractDir, "llama-server.exe"); findErr == nil {
		llamaExe = found
		if prog != nil {
			prog("llama-server (既存確認済み)", 1, 1)
		}
		return
	}

	zipPath := filepath.Join(binDir, "llama-server-"+normBackend+".zip")
	url, cudartURL, err := latestLlamaZipURL(normBackend)
	if err != nil {
		return "", "", err
	}
	if prog != nil {
		prog(fmt.Sprintf("llama-server [%s]", normBackend), 0, 1)
	}
	if err = fetchFile(url, zipPath, func(d, t int64) {
		if prog != nil {
			prog(fmt.Sprintf("llama-server [%s]", normBackend), d, t)
		}
	}); err != nil {
		return "", "", err
	}
	if err = unzip(zipPath, extractDir); err != nil {
		return "", "", err
	}

	if cudartURL != "" {
		cudartZip := filepath.Join(binDir, "cudart.zip")
		if prog != nil {
			prog("CUDA runtime DLLs", 0, 1)
		}
		if err = fetchFile(cudartURL, cudartZip, func(d, t int64) {
			if prog != nil {
				prog("CUDA runtime DLLs", d, t)
			}
		}); err == nil {
			_ = unzip(cudartZip, extractDir)
		}
	}

	llamaExe, err = findExe(extractDir, "llama-server.exe")
	if err != nil {
		llamaExe, err = findExe(extractDir, "llama-server")
	}
	return
}

func latestLlamaZipURL(flavor string) (zipURL string, cudartURL string, err error) {
	req, _ := http.NewRequest(http.MethodGet, ReleaseAPI, nil)
	req.Header.Set("User-Agent", "local-jev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("github HTTP %d: %s", resp.StatusCode, b)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", err
	}

	var bestAssets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	for _, r := range releases {
		if !r.Draft && len(r.Assets) > 0 {
			bestAssets = r.Assets
			break
		}
	}

	if len(bestAssets) == 0 {
		return "", "", fmt.Errorf("no releases found on llama.cpp repository")
	}

	var vulkan, cuda12, cudart, rocm, sycl, cpu string
	for _, a := range bestAssets {
		n := strings.ToLower(a.Name)
		if !strings.Contains(n, "win") || !strings.HasSuffix(n, ".zip") || strings.Contains(n, "arm64") {
			continue
		}
		switch {
		case strings.Contains(n, "cudart"):
			cudart = a.URL
		case strings.Contains(n, "vulkan"):
			vulkan = a.URL
		case strings.Contains(n, "cuda-12") || strings.Contains(n, "cuda12") || strings.Contains(n, "cu12"):
			cuda12 = a.URL
		case strings.Contains(n, "rocm") || strings.Contains(n, "hip"):
			rocm = a.URL
		case strings.Contains(n, "sycl"):
			sycl = a.URL
		case strings.Contains(n, "cpu") || strings.Contains(n, "avx2"):
			if cpu == "" {
				cpu = a.URL
			}
		}
	}

	switch strings.ToLower(flavor) {
	case "cuda":
		if cuda12 != "" {
			return cuda12, cudart, nil
		}
		if vulkan != "" {
			return vulkan, "", nil
		}
	case "hip", "rocm":
		if rocm != "" {
			return rocm, "", nil
		}
		if vulkan != "" {
			return vulkan, "", nil
		}
	case "sycl", "intel":
		if sycl != "" {
			return sycl, "", nil
		}
		if vulkan != "" {
			return vulkan, "", nil
		}
	case "cpu":
		if cpu != "" {
			return cpu, "", nil
		}
		if vulkan != "" {
			return vulkan, "", nil
		}
	default: // vulkan or auto
		if vulkan != "" {
			return vulkan, "", nil
		}
		if cpu != "" {
			return cpu, "", nil
		}
	}

	return "", "", fmt.Errorf("no matching Windows llama-server zip found for backend %q", flavor)
}

func fetchFile(url, dest string, prog func(done, total int64)) error {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "local-jev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download HTTP %d for %s", resp.StatusCode, url)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 1024*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			done += int64(n)
			if prog != nil {
				prog(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}

func unzip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		name := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(name), filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(name) != filepath.Clean(dest) {
			return fmt.Errorf("illegal zip path %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(name, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func findExe(root, name string) (string, error) {
	var found string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), name) {
			found = p
			return io.EOF
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("%s not found in %s", name, root)
	}
	return found, nil
}
