/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package cli

// DBOptions contains common flags for commands using the DB
type DBOptions struct {
	DB string `long:"db" default:"postgres://postgres:example@localhost:5432/lookout?sslmode=disable" env:"LOOKOUT_DB" description:"connection string to postgres database"`
}
