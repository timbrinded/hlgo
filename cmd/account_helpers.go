package cmd

import "github.com/timbrinded/hlgo/pkg/output"

func requireConfirm(cmdName string, confirmed bool, dryRun bool) error {
	if confirmed || dryRun {
		return nil
	}
	return output.NewCLIError(output.ErrValidation, cmdName+" requires --confirm (or --yes)").
		WithDetails("hint", "add --confirm to execute, or --dry-run to preview")
}
