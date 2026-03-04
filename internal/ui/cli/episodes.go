package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

func newEpisodesCmd(app App) *cobra.Command {
	var animeID string

	cmd := &cobra.Command{
		Use:   "episodes",
		Short: "Lista episódios de um anime",
		Long:  "Lista os episódios disponíveis para o anime indicado por --id. Use o ID retornado por 'ani-br search'.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.EpisodesCmd == nil {
				return errors.New("episodes não configurado")
			}
			if animeID == "" {
				return errors.New("informe --id do anime (veja em 'ani-br search <nome>')")
			}
			return app.EpisodesCmd(cmd.Context(), animeID)
		},
	}

	cmd.Flags().StringVar(&animeID, "id", "", "ID do anime (ex.: URL do AnimesOnline ou ID do AniList)")
	return cmd
}
