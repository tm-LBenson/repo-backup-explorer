package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// What we render on the left list.
type BackupEntry struct {
	Name      string // REPO-YYYYMMDD-HHMM
	Path      string // absolute
	Rel       string // "~/<path>"
	SizeBytes int64
	SizeHuman string
}

// Details pane on the right.
type BackupDetail struct {
	Name      string
	Path      string
	AppID     string
	SizeHuman string
}

// BackupsBase => ~/Backups
func BackupsBase() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "Backups")
}

// ListBackups returns newest-first list of backups with the given prefix.
func ListBackups(prefix string) []BackupEntry {
	base := BackupsBase()
	ents, _ := os.ReadDir(base)
	var out []BackupEntry
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix+"-") {
			p := filepath.Join(base, e.Name())
			sz := dirSize(p)
			out = append(out, BackupEntry{
				Name:      e.Name(),
				Path:      p,
				Rel:       "~" + strings.TrimPrefix(p, os.Getenv("HOME")),
				SizeBytes: sz,
				SizeHuman: HumanizeBytes(sz),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

// DescribeBackup fills detail for selected entry.
func DescribeBackup(base, name string) (BackupDetail, error) {
	path := filepath.Join(base, name)
	appid, _ := firstChildDir(filepath.Join(path, "compatdata"))
	size := dirSize(path)
	return BackupDetail{
		Name:      name,
		Path:      path,
		AppID:     appid,
		SizeHuman: HumanizeBytes(size),
	}, nil
}

func CreateBackup(label string) (string, string, error) {
	steamDir := detectSteamDir()
	appid := resolveRepoAppID(steamDir)
	steamid, err := findSteamID(steamDir, appid)
	if err != nil {
		if steamid, err = findAnySteamID(steamDir); err != nil {
			return "", "", err
		}
	}

	// Stop Steam briefly to avoid racey writes
	steamShutdown()

	stamp := time.Now().Format("20060102-1504")
	name := label + "-" + stamp
	dest := filepath.Join(BackupsBase(), name)

	compSrc := filepath.Join(steamDir, "steamapps", "compatdata", appid)
	userSrc := filepath.Join(steamDir, "userdata", steamid, appid)
	compDst := filepath.Join(dest, "compatdata", appid)
	userDst := filepath.Join(dest, "userdata", appid)

	var pieces []string
	if s, err := rsyncCopy(compSrc, compDst); err != nil {
		return "", "", err
	} else if s != "" {
		pieces = append(pieces, "compatdata: "+shortStats(s))
	}
	if s, err := rsyncCopy(userSrc, userDst); err != nil {
		return "", "", err
	} else if s != "" {
		pieces = append(pieces, "userdata: "+shortStats(s))
	}

	// Also capture the Unity LocalLow save folder into dest/saves/Repo (so restore is reliable)
	saveSrc := findRepoSaveInPrefix(compSrc)
	if saveSrc != "" {
		saveDst := filepath.Join(dest, "saves", "Repo")
		if s, err := rsyncCopy(saveSrc, saveDst); err != nil {
			return "", "", err
		} else if s != "" {
			pieces = append(pieces, "saves: "+shortStats(s))
		}
	} else {
		pieces = append(pieces, "saves: (not found)")
	}

	return name, strings.Join(pieces, " | "), nil
}

// RestoreBackup replaces live folders with backup content, returning copy stats.
func RestoreBackup(name string) (string, error) {
	base := BackupsBase()
	path := filepath.Join(base, name)
	if !safeUnder(base, path) {
		return "", fmt.Errorf("bad path")
	}

	appid, err := firstChildDir(filepath.Join(path, "compatdata"))
	if err != nil || appid == "" {
		return "", fmt.Errorf("backup missing compatdata")
	}

	steamDir := detectSteamDir()
	steamid, err := findSteamID(steamDir, appid)
	if err != nil {
		if steamid, err = findAnySteamID(steamDir); err != nil {
			return "", err
		}
	}

	compSrc := filepath.Join(path, "compatdata", appid)
	userSrc := filepath.Join(path, "userdata", appid)
	compDst := filepath.Join(steamDir, "steamapps", "compatdata", appid)
	userDst := filepath.Join(steamDir, "userdata", steamid, appid)

	if !dirNonEmpty(compSrc) && !dirNonEmpty(userSrc) && !exists(filepath.Join(path, "saves", "Repo")) {
		return "", fmt.Errorf("backup is empty (no compatdata/userdata/saves)")
	}

	steamShutdown()

	stamp := time.Now().Format("20060102-1504")
	if exists(compDst) {
		_ = os.Rename(compDst, compDst+".bak-"+stamp)
	}
	if exists(userDst) {
		_ = os.Rename(userDst, userDst+".bak-"+stamp)
	}

	var pieces []string
	if dirNonEmpty(compSrc) {
		if s, err := rsyncCopy(compSrc, compDst); err != nil {
			return "", fmt.Errorf("copy compatdata: %w", err)
		} else if s != "" {
			pieces = append(pieces, "compatdata: "+shortStats(s))
		}
	}
	if dirNonEmpty(userSrc) {
		if s, err := rsyncCopy(userSrc, userDst); err != nil {
			return "", fmt.Errorf("copy userdata: %w", err)
		} else if s != "" {
			pieces = append(pieces, "userdata: "+shortStats(s))
		}
	}

	saveTree := filepath.Join(path, "saves", "Repo")
	if exists(saveTree) {
		liveUsers := filepath.Join(compDst, "pfx", "drive_c", "users")
		userDir, _ := firstChildDir(liveUsers)
		if userDir != "" {
			dstRepo := filepath.Join(liveUsers, userDir, "AppData", "LocalLow", "semiwork", "Repo")
			if exists(dstRepo) {
				_ = os.Rename(dstRepo, dstRepo+".bak-"+stamp)
			}
			_ = os.MkdirAll(dstRepo, 0o755)
			if s, err := rsyncCopy(saveTree, dstRepo); err == nil && s != "" {
				pieces = append(pieces, "saves: "+shortStats(s))
			} else if err != nil {
				return "", fmt.Errorf("copy saves: %w", err)
			}
		}
	} else {
		pieces = append(pieces, "saves: (none)")
	}

	return strings.Join(pieces, " | "), nil
}

func DeleteBackup(name string) error {
	base := BackupsBase()
	path := filepath.Join(base, name)
	if !safeUnder(base, path) {
		return fmt.Errorf("bad path")
	}
	return os.RemoveAll(path)
}

func RenameBackup(oldName, newName, prefix string) error {
	base := BackupsBase()
	oldPath := filepath.Join(base, oldName)
	newPath := filepath.Join(base, newName)
	if !safeUnder(base, oldPath) || !safeUnder(base, newPath) {
		return fmt.Errorf("bad path")
	}
	if !strings.HasPrefix(newName, prefix+"-") {
		return fmt.Errorf("name must start with %s-", prefix)
	}
	if exists(newPath) {
		return fmt.Errorf("a backup with that name already exists")
	}
	return os.Rename(oldPath, newPath)
}

func TotalBackupsSize(prefix string) int64 {
	var total int64
	for _, e := range ListBackups(prefix) {
		total += e.SizeBytes
	}
	return total
}

func OpenBackup(name string) error {
	base := BackupsBase()
	path := filepath.Join(base, name)
	if !safeUnder(base, path) {
		return fmt.Errorf("bad path")
	}
	return exec.Command("xdg-open", path).Start()
}

func findRepoSaveInPrefix(prefixCompat string) string {
	globs := []string{
		filepath.Join(prefixCompat, "pfx", "drive_c", "users", "*", "AppData", "LocalLow", "semiwork", "Repo", "Saves"),
		filepath.Join(prefixCompat, "pfx", "drive_c", "users", "*", "AppData", "LocalLow", "semiwork", "Repo"),
	}
	for _, g := range globs {
		matches, _ := filepath.Glob(g)
		if len(matches) > 0 && exists(matches[0]) {
			if filepath.Base(matches[0]) == "Saves" {
				return filepath.Dir(matches[0])
			}
			return matches[0]
		}
	}
	return ""
}
