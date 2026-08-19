package main

import (
	"flag"
	"fmt"
)

// runSetBanner and runClearBanner are the other administrative escape
// hatch alongside `room`: a Host reaching into the running server's own
// database rather than editing a file it only reads at startup. That's
// what makes them take effect without a restart — see internal/store's
// banner.go and internal/api's getNotice, which reads the same row on
// every request.
//
// They find the database the same way `room` does: through the config
// file, never a path of their own, so a Host can't end up setting the
// banner in a database the running server has never opened.
func runSetBanner(args []string) error {
	fset := flag.NewFlagSet("set-banner", flag.ExitOnError)
	configPath := addConfigFlag(fset)
	fset.Parse(args)
	if fset.NArg() != 1 {
		// The message has to come after any flags, the same rule
		// room reset-password's room code follows: a flag is only parsed
		// as a flag when it precedes the positional argument.
		return fmt.Errorf("usage: longtable set-banner [-config path] <message>")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	return setBanner(cfg.Database, fset.Arg(0))
}

func runClearBanner(args []string) error {
	fset := flag.NewFlagSet("clear-banner", flag.ExitOnError)
	configPath := addConfigFlag(fset)
	fset.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	return setBanner(cfg.Database, "")
}

func setBanner(dbPath, message string) error {
	s, closeDB, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	if err := s.SetBanner(message); err != nil {
		return fmt.Errorf("set banner: %w", err)
	}

	if message == "" {
		fmt.Println("Banner cleared.")
	} else {
		fmt.Printf("Banner set: %s\n", message)
	}
	return nil
}
