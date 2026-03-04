package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newContinueCmd(app App) *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Continua de onde parou",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.ContinueCmd == nil {
				return errors.New("continue não configurado")
			}
			return app.ContinueCmd(cmd.Context())
		},
	}
}

