package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type App struct {
	SearchCmd      func(ctx context.Context, query string) error
	EpisodesCmd    func(ctx context.Context, animeID string) error
	RunTUI         func(ctx context.Context) error
	HistoryCmd     func(ctx context.Context, limit int) error
	ContinueCmd    func(ctx context.Context) error
	DownloadURLCmd  func(ctx context.Context, url string, destDir string) error
	PlayCmd        func(ctx context.Context, animeID string, episode int) error
}

type RootOptions struct {
	Lang    string
	Quality string
	Verbose bool
}

func NewRoot(app App) *cobra.Command {
	var opts RootOptions

	cmd := &cobra.Command{
		Use:   "ani-br",
		Short: "ani-br — CLI/TUI de Anime PT-BR",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return InitConfig()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if app.RunTUI == nil {
				return errors.New("tui não configurada")
			}
			return app.RunTUI(cmd.Context())
		},
	}

	cmd.PersistentFlags().StringVar(&opts.Lang, "lang", "pt-BR-dub", "Idioma preferido (pt-BR-dub | pt-BR-sub)")
	cmd.PersistentFlags().StringVar(&opts.Quality, "quality", "auto", "Qualidade preferida (auto | 720p | 1080p ...)")
	cmd.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "Habilita logs detalhados")

	_ = viper.BindPFlags(cmd.PersistentFlags())

	cmd.AddCommand(newSearchCmd(app))
	cmd.AddCommand(newEpisodesCmd(app))
	cmd.AddCommand(newPlayCmd(app))
	cmd.AddCommand(newDownloadCmd(app))
	cmd.AddCommand(newHistoryCmd(app))
	cmd.AddCommand(newContinueCmd(app))

	return cmd
}

func InitConfig() error {
	viper.SetEnvPrefix("ANI_BR")
	viper.AutomaticEnv()

	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	appDir := filepath.Join(cfgDir, "ani-br")
	_ = os.MkdirAll(appDir, 0o755)

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(appDir)

	// Defaults hardcoded
	viper.SetDefault("preferences.language", "pt-BR-dub")
	viper.SetDefault("preferences.quality", "auto")
	viper.SetDefault("providers.timeout", "10s")
	viper.SetDefault("providers.max_retries", 3)
	viper.SetDefault("cache.ttl", "5m")
	viper.SetDefault("cache.max_size", 100)
	viper.SetDefault("providers.mockptbr_base_url", "http://localhost:8080")
	viper.SetDefault("providers.animesonline_base_url", "https://animesonlinecc.to")

	// Se não existir, cria um arquivo mínimo.
	cfgFile := filepath.Join(appDir, "config.yaml")
	if _, statErr := os.Stat(cfgFile); statErr != nil {
		_ = os.WriteFile(cfgFile, []byte(defaultConfigYAML()), 0o644)
	}

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("falha ao ler config: %w", err)
		}
	}

	return nil
}

func defaultConfigYAML() string {
	return `player:
  path: mpv
  skip_intro: false
  intro_duration: 0
preferences:
  language: pt-BR-dub
  quality: auto
  subtitle_lang: pt-BR-sub
  download_dir: ~/Downloads
  auto_next: true
  auto_resume: true
providers:
  timeout: 10s
  max_retries: 3
  user_agent: ""
  animesonline_base_url: https://animesonlinecc.to
  mockptbr_base_url: http://localhost:8080
cache:
  ttl: 5m
  max_size: 100
`
}

