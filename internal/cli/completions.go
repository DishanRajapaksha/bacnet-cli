package cli

import (
	"fmt"
	"strings"
)

var commands = []string{
	"init-config", "validate-config", "test-connection", "status", "discover",
	"read", "objects", "write", "watch", "routers", "completions", "version", "help",
}

func (a *App) completions(args []string) error {
	if len(args) != 1 {
		return &usageError{message: "completions requires bash or zsh"}
	}
	switch args[0] {
	case "bash":
		fmt.Fprintf(a.out, `_bacnet_cli() {
  local cur
  cur="${COMP_WORDS[COMP_CWORD]}"
  COMPREPLY=( $(compgen -W "%s" -- "$cur") )
}
complete -F _bacnet_cli bacnet-cli
`, strings.Join(commands, " "))
	case "zsh":
		fmt.Fprintf(a.out, `#compdef bacnet-cli
_arguments '1:command:(%s)' '*::args:->args'
`, strings.Join(commands, " "))
	default:
		return &usageError{message: "completions requires bash or zsh"}
	}
	return nil
}
