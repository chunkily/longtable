package admin

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"longtable/internal/config"
	"longtable/internal/listener"
)

// Settings is the half of the store this page needs. An interface rather
// than *store.Store so the admin surface can be tested without a
// database, and so it cannot quietly grow a reach into rooms.
type Settings interface {
	Settings() (map[string]string, error)
	SetSetting(key, value string) error
}

// settingView is one row of the form.
type settingView struct {
	Key     string
	Value   string
	Default string
	Summary string
	// Error is what was wrong with what this field was given, shown
	// beside it rather than at the top: the whole point of validating
	// here is that the person can still see what they typed.
	Error string
	// Warning is allowed-but-worth-saying, from the registry's Warn.
	Warning      string
	NeedsRestart bool
}

type pageData struct {
	Settings []settingView
	GameURL  string
	AdminURL string
	// Saved is set after a successful save, via a redirect, so a refresh
	// doesn't post the form again.
	Saved bool
	// Problem is a fault with the settings *together* rather than with
	// any one of them — two listeners on one port, a storage ceiling
	// under its floor. It has no field to sit beside.
	Problem string
	// Restart is true when something that was saved needs one.
	Restart bool
}

// showSettings renders the form.
func (s *Server) showSettings(w http.ResponseWriter, r *http.Request) {
	values, err := s.settings.Settings()
	if err != nil {
		slog.Error("admin: read settings failed", "error", err)
		http.Error(w, "couldn't read the settings", http.StatusInternalServerError)
		return
	}

	data := pageData{
		Settings: viewsFor(values, nil),
		GameURL:  s.gameURL,
		Saved:    r.URL.Query().Has("saved"),
		Restart:  r.URL.Query().Has("restart"),
	}
	s.render(w, data)
}

// saveSettings validates everything, then writes what changed.
//
// Nothing is written until the whole form is known to be good. A form
// that half-applied would leave a server in a state nobody asked for and
// no single screen describing it — and the cross-setting rules cannot be
// judged field by field anyway.
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "couldn't read that form", http.StatusBadRequest)
		return
	}

	stored, err := s.settings.Settings()
	if err != nil {
		slog.Error("admin: read settings failed", "error", err)
		http.Error(w, "couldn't read the settings", http.StatusInternalServerError)
		return
	}

	// Every value goes through the registry's own Parse — the same call
	// `longtable config set` makes — so the screen and the command line
	// cannot disagree about what a valid value is.
	fieldErrors := map[string]string{}
	submitted := map[string]string{}
	prospective := map[string]string{}
	for key, value := range stored {
		prospective[key] = value
	}

	for _, setting := range config.Registry {
		raw := r.PostForm.Get(setting.Key)
		submitted[setting.Key] = raw

		canonical, err := setting.Parse(raw)
		if err != nil {
			fieldErrors[setting.Key] = err.Error()
			continue
		}
		prospective[setting.Key] = canonical
	}

	problem := ""
	if len(fieldErrors) == 0 {
		resolved, err := config.Resolve(prospective)
		if err != nil {
			problem = err.Error()
		} else if err := resolved.Validate(); err != nil {
			problem = err.Error()
		}
	}

	// The address is applied before anything is stored, because the two
	// can disagree and only one order is safe. Applied first, a bind that
	// fails changes nothing at all — the server is still on the old
	// address and the setting still says so. Stored first, a failed bind
	// leaves the database naming an address the server isn't on and
	// couldn't take, so the *next* start fails too, on a value nobody can
	// see is wrong.
	// appliedLive names the settings this save has already put into
	// effect itself. `Setting.Live` cannot answer for `addr`: it means
	// "the running server picks this up on its own", which is true of the
	// banner and false of an address — the *page* applies that one,
	// because it is inside the process, and `longtable config set addr`
	// from a terminal genuinely cannot. So the restart notice is built
	// from what happened rather than from the flag.
	appliedLive := map[string]bool{}
	if len(fieldErrors) == 0 && problem == "" {
		if next := prospective["addr"]; next != effectiveAddr(stored) {
			switch err := s.moveTo(next); {
			case err == nil:
				appliedLive["addr"] = true
			case errors.Is(err, listener.ErrNeedsRestart):
				// A legitimate change that can't be made live. Left out of
				// appliedLive so the notice below asks for the restart.
			default:
				fieldErrors["addr"] = err.Error()
			}
		}
	}

	if len(fieldErrors) > 0 || problem != "" {
		// Re-rendered with what was typed still in the boxes. Handing back
		// the stored values instead would throw away the work and hide the
		// mistake being complained about.
		s.render(w, pageData{
			Settings: viewsFor(submitted, fieldErrors),
			GameURL:  s.gameURL,
			Problem:  problem,
		})
		return
	}

	// Only what actually changed is written. A save is then a description
	// of the edit rather than of the form, which keeps the stored
	// settings meaning "what somebody chose" — the same thing `config
	// list` reports as changed.
	restart := false
	for _, setting := range config.Registry {
		was := effective(setting, stored)
		now := prospective[setting.Key]
		if now == was {
			continue
		}
		if err := s.settings.SetSetting(setting.Key, now); err != nil {
			slog.Error("admin: save setting failed", "key", setting.Key, "error", err)
			http.Error(w, "couldn't save that", http.StatusInternalServerError)
			return
		}
		if !setting.Live && !appliedLive[setting.Key] {
			restart = true
		}
	}

	// Redirect after posting, so a refresh doesn't save again.
	target := "/?saved"
	if restart {
		target += "&restart"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// viewsFor builds a row per setting, from whatever values are in hand —
// the stored ones on the way in, the submitted ones on the way back.
func viewsFor(values, fieldErrors map[string]string) []settingView {
	views := make([]settingView, 0, len(config.Registry))
	for _, setting := range config.Registry {
		value, ok := values[setting.Key]
		if !ok {
			value = setting.Default
		}

		warning := ""
		if setting.Warn != nil {
			warning = setting.Warn(value)
		}

		views = append(views, settingView{
			Key:          setting.Key,
			Value:        value,
			Default:      setting.Default,
			Summary:      setting.Summary,
			Error:        fieldErrors[setting.Key],
			Warning:      warning,
			NeedsRestart: !setting.Live,
		})
	}
	return views
}

// moveTo asks for the game listener to be moved, when there is one to
// move. A CLI-only build of this page has no listener of its own, and a
// change it cannot apply is one to restart for.
func (s *Server) moveTo(addr string) error {
	if s.rebind == nil {
		return listener.ErrNeedsRestart
	}
	return s.rebind(addr)
}

// effectiveAddr is the address the server is currently meant to be on.
func effectiveAddr(stored map[string]string) string {
	setting, ok := config.Lookup("addr")
	if !ok {
		return ""
	}
	return effective(setting, stored)
}

// effective is what a setting is currently worth: stored, or its default.
func effective(setting config.Setting, values map[string]string) string {
	if value, ok := values[setting.Key]; ok {
		return value
	}
	return setting.Default
}

func (s *Server) render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		// Too late to change the status: something has already been
		// written. Logged so it isn't silent.
		slog.Error("admin: render failed", "error", err)
		fmt.Fprint(w, "\n<!-- render failed -->")
	}
}
