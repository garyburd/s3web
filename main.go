package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/garyburd/s3web/deploys3"
	"github.com/garyburd/s3web/serve"
)

var commands = []struct {
	name  string
	fs    *flag.FlagSet
	usage string
	run   func()
}{
	{"serve", serve.FlagSet, serve.Usage, serve.Run},
	{"deploys3", deploys3.FlagSet, deploys3.Usage, deploys3.Run},
}

func main() {
	log.SetFlags(0)
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return
	}
	for _, c := range commands {
		if args[0] == c.name {
			c.fs.Usage = func() {
				log.Println(c.usage)
				c.fs.PrintDefaults()
				os.Exit(2)
			}
			c.fs.Parse(args[1:])
			c.run()
			return
		}
	}
	flag.Usage()
}

func printUsage() {
	var names []string
	for _, c := range commands {
		names = append(names, c.name)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "%s %s\n", os.Args[0], strings.Join(names, "|"))
	flag.PrintDefaults()
}
