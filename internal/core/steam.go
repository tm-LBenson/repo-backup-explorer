package core

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func detectSteamDir() string {
	h, _ := os.UserHomeDir()
	cands := []string{
		filepath.Join(h, ".local", "share", "Steam"),
		filepath.Join(h, ".steam", "steam"),
	}
	for _, p := range cands {
		if exists(filepath.Join(p, "steamapps")) {
			return p
		}
	}
	return cands[0]
}

func resolveRepoAppID(steamDir string) string {
	if id, ok := findAppIDFuzzy(steamDir, "repo"); ok {
		return id
	}
	return "3241660"
}

func findAppIDFuzzy(steamDir, want string) (string, bool) {
	norm := func(s string) string {
		re := regexp.MustCompile(`[^a-z0-9]`)
		return re.ReplaceAllString(strings.ToLower(s), "")
	}
	wantN := norm(want)

	mans, _ := filepath.Glob(filepath.Join(steamDir, "steamapps", "appmanifest_*.acf"))
	reName := regexp.MustCompile(`"name"\s*"(.*)"`)

	for _, m := range mans {
		f, err := os.Open(m)
		if err != nil {
			continue
		}
		s := bufio.NewScanner(f)
		for s.Scan() {
			if reName.MatchString(s.Text()) {
				name := reName.FindStringSubmatch(s.Text())[1]
				if n := norm(name); n == wantN || strings.Contains(n, wantN) {
					base := filepath.Base(m)
					_ = f.Close()
					return strings.TrimSuffix(strings.TrimPrefix(base, "appmanifest_"), ".acf"), true
				}
			}
		}
		_ = f.Close()
	}
	return "", false
}

func findSteamID(steamDir, appid string) (string, error) {
	userDir := filepath.Join(steamDir, "userdata")
	ents, err := os.ReadDir(userDir)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if exists(filepath.Join(userDir, e.Name(), appid)) {
			return e.Name(), nil
		}
	}
	return "", os.ErrNotExist
}

func findAnySteamID(steamDir string) (string, error) {
	userDir := filepath.Join(steamDir, "userdata")
	ents, err := os.ReadDir(userDir)
	if err != nil {
		return "", err
	}
	num := regexp.MustCompile(`^\d+$`)
	for _, e := range ents {
		if e.IsDir() && num.MatchString(e.Name()) {
			return e.Name(), nil
		}
	}
	return "", os.ErrNotExist
}

func steamShutdown() {
	if p, err := exec.LookPath("steam"); err == nil {
		_ = exec.Command(p, "-shutdown").Start()
	}
	h, _ := os.UserHomeDir()
	cands := []string{
		filepath.Join(h, ".steam", "steam", "steam.sh"),
		filepath.Join(h, ".steam", "steam", "ubuntu12_32", "steam"),
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			_ = exec.Command(c, "-shutdown").Start()
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("pgrep", "-x", "steam").Run() != nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
}
