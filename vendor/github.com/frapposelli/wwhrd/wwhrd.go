package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

// Initialize and run wwhrd
func main() {
	parser := newCli()
	c, err := parser.Parse()
	if err != nil {
		if _, ok := err.(*flags.Error); ok {
			typ := err.(*flags.Error).Type
			switch {
			case typ == VersionHelp:
				fmt.Println(err.(*flags.Error).Message)
			case typ == flags.ErrHelp:
				parser.WriteHelp(os.Stdout)
			case typ == flags.ErrCommandRequired && len(c[0]) == 0:
				parser.WriteHelp(os.Stdout)
			default:
				log.Info(err.Error() + fmt.Sprint(typ))
				parser.WriteHelp(os.Stdout)
			}
		} else {
			log.Fatalf("Exiting: %s", err.Error())
		}
	}
}
