package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newSearchCmd(app App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Busca animes (prioridade PT-BR)",
		Long:  "Busca por nome; exibe título, provedor e idioma (dub/leg). Use o ID na saída com 'ani-br play --id <ID> --ep <n>' ou 'ani-br episodes --id <ID>'.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.SearchCmd == nil {
				return errors.New("search não configurado")
			}
			query := args[0]
			return app.SearchCmd(cmd.Context(), query)
		},
	}
	return cmd
}

func PrintLines(ctx context.Context, lines []string) error {
	_ = ctx
	for _, l := range lines {
		fmt.Println(l)
	}
	return nil
}

