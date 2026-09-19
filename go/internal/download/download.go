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
	GGUFURL = "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q8_0.gguf?download=true"
	GGUFName = "Qwen3.5-0.8B-Q8_0.gguf"
	ReleaseAPI = "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"
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
	zipPath := filepath.Join(binDir, "llama-server.zip")
	url, err := latestLlamaZipURL(backend)
	if err != nil {
		return
	}
	if prog != nil {
		prog("llama-server zip", 0, 1)
	}
	if err = fetchFile(url, zipPath, func(d, t int64) {
		if prog != nil {
			prog("llama-server zip", d, t)
		}
	}); err != nil {
		return
	}
	extractDir := filepath.Join(binDir, "llama.cpp")
	if err = unzip(zipPath, extractDir); err != nil {
		return
	}
	llamaExe, err = findExe(extractDir, "llama-server.exe")
	if err != nil {
		llamaExe, err = findExe(extractDir, "llama-server")
	}
	return
}

func latestLlamaZipURL(flavor string) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, ReleaseAPI, nil)
	req.Header.Set("User-Agent", "local-jev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github HTTP %d: %s", resp.StatusCode, b)
	}
	var rel struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	var vulkan, cuda12, cuda, cpu string
	for _, a := range rel.Assets {
		n := strings.ToLower(a.Name)
		if !strings.Contains(n, "win") || !strings.HasSuffix(n, ".zip") || !strings.Contains(n, "x64") {
			continue
		}
		switch {
		case strings.Contains(n, "vulkan"):
			vulkan = a.URL
		case strings.Contains(n, "cu12") || strings.Contains(n, "cuda-12") || strings.Contains(n, "cuda12"):
			cuda12 = a.URL
		case strings.Contains(n, "cuda"):
			cuda = a.URL
		case strings.Contains(n, "cpu") || strings.Contains(n, "avx2"):
			if cpu == "" {
				cpu = a.URL
			}
		}
	}
	switch strings.ToLower(flavor) {
	case "cuda":
		if cuda12 != "" {
			return cuda12, nil
		}
		if cuda != "" {
			return cuda, nil
		}
		if vulkan != "" {
			return vulkan, nil
		}
	case "cpu":
		if cpu != "" {
			return cpu, nil
		}
		if vulkan != "" {
			return vulkan, nil
		}
	default:
		if vulkan != "" {
			return vulkan, nil
		}
		if cpu != "" {
			return cpu, nil
		}
	}
	return "", fmt.Errorf("no Windows llama-server zip in latest release")
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
	buf := make([]byte, 256*1024)
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
