package flagutil

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
)

// Command represents a command to be executed.
type Command struct {
	// Name is the name of the command.
	Name string

	// Usage must contain usage docs for the given command.
	Usage string

	// Subcommands is an optional subcommands for the given command.
	Subcommands []*Command

	// Action is the action, which must be performed on the given command.
	//
	// args contains arg values for Args.
	Action Action

	// Args is an optional names of arguments passed to the given command.
	Args []string

	EnvArgs []string
}

type Action func(args []string)

// Run processes command-line args and executes the matching command.
func (c *Command) Run() {
	prevArgs := []string{c.Name}
	c.run(os.Args[1:], prevArgs)
}

func (c *Command) run(args, prevArgs []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		if hasHelpFlag(args) {
			c.printUsage(prevArgs, true, true)
			os.Exit(0)
		}
		if c.Action == nil {
			c.printUsage(prevArgs, true, false)
			os.Exit(0)
		}
		// Parse the remaining command-line flags starting with '-' and execute the current command.
		c.parseFlags(args, prevArgs)
		c.Action(nil)
		return
	}
	cmdName := args[0]
	if subCmd := c.getSubcommand(cmdName); subCmd != nil {
		prevArgs = append(prevArgs, cmdName)
		subCmd.run(args[1:], prevArgs)
		return
	}
	if cmdName == "help" {
		if len(args) == 2 {
			// Print help about the next named command
			if subCmd := c.getSubcommand(args[1]); subCmd != nil {
				prevArgs = append(prevArgs, args[1])
				subCmd.printUsage(prevArgs, true, false)
				os.Exit(0)
			}
		}
		c.printUsage(prevArgs, true, false)
		os.Exit(0)
	}
	// No matching subcommand found. Probably this is just an additional arg to the current command.
	cmdArgs := []string{cmdName}
	args = args[1:]
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmdArgs = append(cmdArgs, args[0])
		args = args[1:]
	}
	if c.Action != nil && len(cmdArgs) == len(c.Args) {
		// Execute the current command with additional args.
		c.parseFlags(args, prevArgs)
		c.Action(cmdArgs)
		return
	}
	// Log error about unknown subcommand.
	f := flag.CommandLine.Output()
	fmt.Fprintf(f, "unknown subcommand %s\n", cmdName)
	c.printUsage(prevArgs, true, false)
	os.Exit(2)
}

func (c *Command) getSubcommand(cmdName string) *Command {
	for _, subCmd := range c.Subcommands {
		if subCmd.Name == cmdName {
			return subCmd
		}
	}
	return nil
}

func (c *Command) parseFlags(args, prevArgs []string) {
	flag.CommandLine.Usage = func() {
		c.printUsage(prevArgs, false, false)
	}
	envflag.ParseFlagSet(flag.CommandLine, args)
}

func (c *Command) printUsage(prevArgs []string, showSubcommands, showFlags bool) {
	f := flag.CommandLine.Output()
	prefix := strings.Join(prevArgs, " ")
	if len(c.Args) > 0 {
		prefix += " <" + strings.Join(c.Args, "> <") + ">"
	}
	fmt.Fprintf(f, "%s: %s\n", prefix, c.Usage)
	if showSubcommands && len(c.Subcommands) > 0 {
		fmt.Fprintf(f, "\nsubcommands:\n")
		for _, subCmd := range c.Subcommands {
			fmt.Fprintf(f, "\t%s: %s\n", subCmd.Name, subCmd.Usage)
		}
	}
	if showFlags {
		fmt.Fprintf(f, "\ncommand-line flags:\n")
		flag.CommandLine.PrintDefaults()
	} else {
		fmt.Fprintf(f, "\nRun '%s -help' in order to see the description for all the available flags\n", os.Args[0])
	}
}
