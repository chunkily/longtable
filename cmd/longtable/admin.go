package main

import (
	"flag"
	"fmt"

	"longtable/internal/config"
)

// runAdmin prints the way into the admin page, and can issue a new one.
//
// This exists because the startup line is not enough on its own. A
// console scrolls, and on Windows one opened by double-clicking the
// binary closes when the process does — so a Host who wants the admin
// page tomorrow has nowhere to look. It is also the answer to having
// pasted the URL somewhere regrettable, since `-new` invalidates what
// was shared.
func runAdmin(args []string) error {
	const synopsis = "longtable admin [-db path] [-new]"
	fset := flag.NewFlagSet("admin", flag.ExitOnError)
	dbPath := addDBFlag(fset, synopsis)
	issueNew := fset.Bool("new", false, "replace the token, so any link already shared stops working")
	fset.Parse(args)

	s, closeDB, err := openStore(*dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	cfg, err := loadSettings(s)
	if err != nil {
		return err
	}

	if cfg.AdminAddr == "" {
		// A URL to nowhere would be worse than useless here, since this
		// command exists precisely for the Host who can't find the way in.
		fmt.Println("The admin page is off.")
		fmt.Println("Turn it back on with: longtable config unset admin_addr")
		return nil
	}

	token := ""
	if *issueNew {
		token, err = s.RotateAdminToken()
	} else {
		token, err = s.AdminToken()
	}
	if err != nil {
		return fmt.Errorf("read admin token: %w", err)
	}

	if *issueNew {
		fmt.Println("Issued a new token. Any link you shared before now stops working.")
	}
	fmt.Println(adminURL(cfg.AdminAddr, token))

	// Said here as well as in the hosting guide, because this command is
	// how a Host gets the link they are about to send someone, and the
	// moment to mention that it isn't encrypted is before they do.
	if setting, ok := config.Lookup("admin_addr"); ok && setting.Warn != nil {
		if warning := setting.Warn(cfg.AdminAddr); warning != "" {
			fmt.Println()
			fmt.Println(warning)
		}
	}
	return nil
}
