package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newPlayCmd(app App) *cobra.Command {
	var animeID string
	var episode int

	cmd := &cobra.Command{
		Use:   "play",
		Short: "Reproduz um episódio via mpv",
		Long:  "Reproduz no mpv o episódio indicado. Use 'ani-br search <nome>' para obter IDs e 'ani-br episodes --id <ID>' para listar episódios. Para uso interativo (buscar e escolher no terminal), execute 'ani-br' sem subcomando.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.PlayCmd == nil {
				return errors.New("play não configurado")
			}
			if animeID == "" {
				return fmt.Errorf("informe --id com o ID do anime (ex.: ani-br search naruto)")
			}
			if episode <= 0 {
				return fmt.Errorf("informe --ep com o número do episódio (>= 1). Use 'ani-br episodes --id %s' para listar", animeID)
			}
			return app.PlayCmd(cmd.Context(), animeID, episode)
		},
	}

	cmd.Flags().StringVar(&animeID, "id", "", "ID do anime (retornado por 'ani-br search')")
	cmd.Flags().IntVar(&episode, "ep", 1, "Número do episódio a reproduzir")

	return cmd
}

