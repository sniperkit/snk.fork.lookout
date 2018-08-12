/*
Sniperkit-Bot
- Date: 2018-08-12 11:57:50.86147846 +0200 CEST m=+0.186676333
- Status: analyzed
*/

package main

import (
	"github.com/sniperkit/snk.fork.lookout/util/cli"
)

var (
	name    = "lookout"
	version = "undefined"
	build   = "undefined"
)

var app = cli.New(name)

func main() {
	app.RunMain()
}
