package handlers

import (
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"repo-backup-explorer/internal/core"
)

const (
	gameLabel = "REPO"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", getIndex)
	mux.HandleFunc("/backup", postOnly(postBackup))
	mux.HandleFunc("/restore", postOnly(postRestore))
	mux.HandleFunc("/delete", postOnly(postDelete))
	mux.HandleFunc("/open", postOnly(postOpen))
}


func getIndex(w http.ResponseWriter, r *http.Request) {
	base := core.BackupsBase()
	entries := core.ListBackups(gameLabel)

	sel := r.URL.Query().Get("sel")
	if sel == "" && len(entries) > 0 {
		sel = entries[0].Name
	}

	var detail *core.BackupDetail
	if sel != "" {
		if d, err := core.DescribeBackup(base, sel); err == nil {
			detail = &d
		}
	}

	t := template.Must(template.ParseFiles(templatePath()))
	_ = t.Execute(w, map[string]any{
		"Game":    gameLabel,
		"Entries": entries,
		"SelName": sel,
		"Sel":     detail,
		"Status":  r.URL.Query().Get("ok"),
		"Error":   r.URL.Query().Get("err"),
	})
}

func postBackup(w http.ResponseWriter, r *http.Request) {
	name, summary, err := core.CreateBackup(gameLabel)
	if err != nil {
		redirectErr(w, r, "", err.Error())
		return
	}
	msg := "Backup created"
	if summary != "" {
		msg += " — " + summary
	}
	redirectOK(w, r, name, msg)
}

func postRestore(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := r.FormValue("name")
	summary, err := core.RestoreBackup(name)
	if err != nil {
		redirectErr(w, r, name, err.Error())
		return
	}
	msg := "Restore complete"
	if summary != "" {
		msg += " — " + summary
	}
	redirectOK(w, r, name, msg)
}

func postDelete(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := r.FormValue("name")
	if err := core.DeleteBackup(name); err != nil {
		redirectErr(w, r, "", err.Error())
		return
	}
	redirectOK(w, r, "", "Deleted "+name)
}

func postOpen(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	name := r.FormValue("name")
	_ = core.OpenBackup(name)
	redirectOK(w, r, name, "")
}


func postOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func redirectOK(w http.ResponseWriter, r *http.Request, sel, msg string) {
	q := url.Values{}
	if sel != "" {
		q.Set("sel", sel)
	}
	if msg != "" {
		q.Set("ok", msg)
	}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

func redirectErr(w http.ResponseWriter, r *http.Request, sel, msg string) {
	q := url.Values{}
	if sel != "" {
		q.Set("sel", sel)
	}
	q.Set("err", msg)
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}


func templatePath() string {
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, "web", "templates", "index.html")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "..", "..", "..", "web", "templates", "index.html"),    // binary in cmd/repo-backup-explorer/
		filepath.Join(exeDir, "..", "..", "..", "..", "web", "templates", "index.html"),
		filepath.Join(exeDir, "web", "templates", "index.html"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return filepath.Join(".", "web", "templates", "index.html")
}
