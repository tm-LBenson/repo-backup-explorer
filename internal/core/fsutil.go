package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// exists checks if a path exists.
func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// safeUnder ensures p is under base (prevents path traversal).
func safeUnder(base, p string) bool {
	absBase, _ := filepath.Abs(base)
	absP, _ := filepath.Abs(p)
	return strings.HasPrefix(absP, absBase+string(os.PathSeparator))
}

func firstChildDir(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range ents {
		if e.IsDir() {
			return e.Name(), nil
		}
	}
	return "", nil
}

func dirNonEmpty(p string) bool {
	ents, err := os.ReadDir(p)
	return err == nil && len(ents) > 0
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func HumanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// rsyncCopy copies src -> dst and returns rsync stats (shortened later for UI).
func rsyncCopy(src, dst string) (string, error) {
	if !exists(src) {
		return "", nil // ok: nothing to copy
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("rsync", "-aH", "--info=stats2", src+"/", dst+"/")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// shortStats picks useful lines from rsync --info=stats2 output.
func shortStats(s string) string {
	lines := []string{}
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "Number of created files:") ||
			strings.HasPrefix(ln, "Number of regular files transferred:") ||
			strings.HasPrefix(ln, "Total transferred file size:") {
			lines = append(lines, ln)
		}
	}
	return strings.Join(lines, " / ")
}
