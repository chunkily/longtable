package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"longtable/internal/config"
	"longtable/internal/store"
)

// configUsage covers all six, since a Host reaching for one of them
// usually wants to know what the others are.
const configUsage = `Usage:
  longtable config list [-db path]
  longtable config get [-db path] <key>
  longtable config set [-db path] <key> <value>
  longtable config unset [-db path] <key>
  longtable config export [-db path] [-o file]
  longtable config import [-db path] <file>`

// runConfigCommand is the half of ADR-0009 that keeps a Host who works
// in a terminal whole. Everything the admin UI can change is here, and
// the rule is that it stays that way: a setting reachable only by
// clicking has abandoned every Host running headless.
func runConfigCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(configUsage)
	}
	if isHelpArg(args[0]) {
		fmt.Println(configUsage)
		return nil
	}

	switch args[0] {
	case "list":
		return withSettings(args, "longtable config list [-db path]", 0,
			func(s *store.Store, _ []string) error { return configList(s) })

	case "get":
		return withSettings(args, "longtable config get [-db path] <key>", 1,
			func(s *store.Store, rest []string) error { return configGet(s, rest[0]) })

	case "set":
		return withSettings(args, "longtable config set [-db path] <key> <value>", 2,
			func(s *store.Store, rest []string) error { return configSet(s, rest[0], rest[1]) })

	case "unset":
		return withSettings(args, "longtable config unset [-db path] <key>", 1,
			func(s *store.Store, rest []string) error { return configUnset(s, rest[0]) })

	case "export":
		return configExport(args[1:])

	case "import":
		return withSettings(args, "longtable config import [-db path] <file>", 1,
			func(s *store.Store, rest []string) error { return configImport(s, rest[0]) })

	default:
		return fmt.Errorf("unknown config command %q\n%s", args[0], configUsage)
	}
}

// withSettings does the parsing every config command shares: the -db
// flag, an exact argument count, and opening the database.
//
// wantArgs is exact rather than a minimum because a value with a space
// in it — a banner, an assets path — arrives as several arguments when
// it wasn't quoted, and silently using the first word is worse than
// saying so.
func withSettings(args []string, synopsis string, wantArgs int, run func(*store.Store, []string) error) error {
	fset := flag.NewFlagSet("config "+args[0], flag.ExitOnError)
	dbPath := addDBFlag(fset, synopsis)
	fset.Parse(args[1:])

	if fset.NArg() != wantArgs {
		return fmt.Errorf("usage: %s", synopsis)
	}

	s, closeDB, err := openStore(*dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	return run(s, fset.Args())
}

// configList is where the config file's comments went.
//
// ADR-0006 chose a format that could hold comments so a Host could see
// what a setting does without leaving the file, and ADR-0009 deleted the
// file. This and `config export` are the debt being paid: if they are
// terse, a Host is worse off than they were with the commented file.
func configList(s *store.Store) error {
	values, err := s.Settings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	// Padded by hand rather than with a tabwriter, which aligns a run of
	// rows and nothing else — the description lines between them aren't
	// part of the table, so flushing per row would give every setting its
	// own column widths and no alignment at all.
	keyWidth, valueWidth := 0, 0
	for _, setting := range config.Registry {
		keyWidth = max(keyWidth, len(setting.Key))
		valueWidth = max(valueWidth, len(effectiveValue(setting, values)))
	}

	for i, setting := range config.Registry {
		if i > 0 {
			fmt.Println()
		}

		_, isSet := values[setting.Key]
		notes := "default"
		if isSet {
			notes = "default " + setting.Default
		}
		if !setting.Live {
			notes += ", needs a restart"
		}

		fmt.Printf("%-*s  %-*s  (%s)\n",
			keyWidth, setting.Key, valueWidth, effectiveValue(setting, values), notes)
		// A fixed indent rather than one lined up under the value column,
		// which on a long key pushes the prose most of the way across the
		// terminal and wraps it into a narrow ribbon.
		for _, line := range wrapSummary(setting.Summary, 72) {
			fmt.Printf("    %s\n", line)
		}
	}
	return nil
}

// effectiveValue is what the server will actually use: what was stored,
// or the registry's default when nothing was.
func effectiveValue(setting config.Setting, values map[string]string) string {
	if value, ok := values[setting.Key]; ok {
		return value
	}
	return setting.Default
}

// configGet prints the bare value and nothing else, so it is useful
// inside `$(longtable config get addr)`. Everything explanatory belongs
// in list, which is the command a person runs.
func configGet(s *store.Store, key string) error {
	setting, ok := config.Lookup(key)
	if !ok {
		return unknownSetting(key)
	}

	value, isSet, err := s.GetSetting(key)
	if err != nil {
		return fmt.Errorf("read setting: %w", err)
	}
	if !isSet {
		value = setting.Default
	}
	fmt.Println(value)
	return nil
}

// configSet validates before it stores, which is the main thing the
// database bought over the file: a value that cannot work is refused
// while the person who typed it is still looking at it, rather than
// surfacing later as a server that won't start.
func configSet(s *store.Store, key, value string) error {
	setting, ok := config.Lookup(key)
	if !ok {
		return unknownSetting(key)
	}

	canonical, err := setting.Parse(value)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}

	values, err := s.Settings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	previous, wasSet := values[key]
	if !wasSet {
		previous = setting.Default
	}

	// Checked as a whole rather than in isolation, because some rules are
	// about two settings at once — a total storage limit below the
	// per-asset limit would accept fine on its own and leave a server
	// nothing could ever be uploaded to.
	values[key] = canonical
	prospective, err := config.Resolve(values)
	if err != nil {
		return err
	}
	if err := prospective.Validate(); err != nil {
		return err
	}

	if err := s.SetSetting(key, canonical); err != nil {
		return fmt.Errorf("save setting: %w", err)
	}

	if canonical == previous {
		fmt.Printf("%s was already %s\n", key, canonical)
		return nil
	}
	fmt.Printf("%s is now %s (was %s)\n", key, canonical, previous)
	if !setting.Live {
		// Said every time rather than only when a server is running: this
		// command works fine against a stopped one, and a Host who assumes
		// a change took effect has been told something untrue.
		fmt.Println("Restart Longtable for this to take effect.")
	}
	return nil
}

func configUnset(s *store.Store, key string) error {
	setting, ok := config.Lookup(key)
	if !ok {
		return unknownSetting(key)
	}
	if err := s.UnsetSetting(key); err != nil {
		return fmt.Errorf("clear setting: %w", err)
	}
	fmt.Printf("%s is back to its default, %s\n", key, setting.Default)
	if !setting.Live {
		fmt.Println("Restart Longtable for this to take effect.")
	}
	return nil
}

func configExport(args []string) error {
	const synopsis = "longtable config export [-db path] [-o file]"
	fset := flag.NewFlagSet("config export", flag.ExitOnError)
	dbPath := addDBFlag(fset, synopsis)
	out := fset.String("o", "", "write to this file instead of standard output")
	fset.Parse(args)

	s, closeDB, err := openStore(*dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	values, err := s.Settings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	rendered := config.Export(values)

	if *out == "" {
		fmt.Print(rendered)
		return nil
	}
	if err := os.WriteFile(*out, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *out, err)
	}
	fmt.Printf("Wrote %s\n", *out)
	return nil
}

// configImport replays an export.
//
// Every value goes through the same Parse `config set` uses, so a file
// cannot put something into the database that typing it would have been
// refused. Validation happens across the whole set before anything is
// written, so a file with one bad line leaves the server exactly as it
// was rather than half-changed.
func configImport(s *store.Store, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	imported, err := config.Import(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	values, err := s.Settings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}

	canonical := map[string]string{}
	for key, raw := range imported {
		setting, ok := config.Lookup(key)
		if !ok {
			return fmt.Errorf("%s: %w", path, unknownSetting(key))
		}
		value, err := setting.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, key, err)
		}
		canonical[key] = value
		values[key] = value
	}

	prospective, err := config.Resolve(values)
	if err != nil {
		return err
	}
	if err := prospective.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	for key, value := range canonical {
		if err := s.SetSetting(key, value); err != nil {
			return fmt.Errorf("save %s: %w", key, err)
		}
	}

	fmt.Printf("Read %d settings from %s\n", len(canonical), path)
	fmt.Println("Restart Longtable for them to take effect.")
	return nil
}

// unknownSetting refuses a key nobody has, and names the near miss.
//
// The file this replaced refused an unknown key and drew the line it was
// on, and its own comment called that "the one failure a config file
// invents that a command line never had" — a Host edits, restarts, and
// watches the setting do nothing. Deleting the file must not delete the
// guarantee.
func unknownSetting(key string) error {
	if suggestion := config.Suggest(key); suggestion != "" {
		return fmt.Errorf("unknown setting %q. Did you mean %q?", key, suggestion)
	}
	return fmt.Errorf("unknown setting %q. Run longtable config list to see them all", key)
}

// wrapSummary breaks a setting's description into lines that fit under
// it.
func wrapSummary(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if len(lines[last])+1+len(word) <= width {
			lines[last] += " " + word
			continue
		}
		lines = append(lines, word)
	}
	return lines
}
