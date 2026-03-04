package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newHistoryCmd(app App) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Mostra histórico",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.HistoryCmd == nil {
				return errors.New("history não configurado")
			}
			return app.HistoryCmd(cmd.Context(), limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Número máximo de itens")
	return cmd
}

func printHistoryLine(i int, line string) {
	fmt.Printf("%02d. %s\n", i, line)
}

