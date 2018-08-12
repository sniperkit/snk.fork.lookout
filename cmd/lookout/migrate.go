/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package main

import (
	"github.com/golang-migrate/migrate"
	log "gopkg.in/src-d/go-log.v1"

	"github.com/sniperkit/snk.fork.lookout/store"
	"github.com/sniperkit/snk.fork.lookout/util/cli"
)

func init() {
	if _, err := app.AddCommand("migrate", "performs a DB migration up to the latest version", "",
		&MigrateCommand{}); err != nil {
		panic(err)
	}
}

type MigrateCommand struct {
	cli.LogOptions
	cli.DBOptions
}

func (c *MigrateCommand) Execute(args []string) error {
	m, err := store.NewMigrateDSN(c.DB)
	if err != nil {
		return err
	}

	err = m.Up()
	switch err {
	case nil:
		log.Infof("The DB was upgraded")
	case migrate.ErrNoChange:
		log.Infof("The DB is up to date")
	default:
		return err
	}

	return nil
}
