package cli

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDownloadCmd(app App) *cobra.Command {
	var destDir string
	cmd := &cobra.Command{
		Use:   "download <url>",
		Short: "Baixa um vídeo direto por URL (MVP)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.DownloadURLCmd == nil {
				return errors.New("download não configurado")
			}
			if destDir == "" {
				destDir = viper.GetString("preferences.download_dir")
			}
			return app.DownloadURLCmd(cmd.Context(), args[0], destDir)
		},
	}
	cmd.Flags().StringVar(&destDir, "dir", "", "Diretório de download")
	return cmd
}

