package cli

import (
	"fmt"
	"io"
)

type subcommandRunner func(args []string, stdout, stderr io.Writer) int

type subcommandSpec struct {
	Name string
	Run  subcommandRunner
}

func dispatchSubcommand(
	args []string,
	stdout, stderr io.Writer,
	usageLine string,
	unknownCommandFormat string,
	specs ...subcommandSpec,
) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usageLine)
		return 1
	}

	command := args[0]
	for _, spec := range specs {
		if spec.Name == command {
			return spec.Run(args[1:], stdout, stderr)
		}
	}

	fmt.Fprintf(stderr, unknownCommandFormat, command)
	return 1
}
