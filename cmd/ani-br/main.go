package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/bernardofernandezz/ani-br/internal/domain"
	"github.com/bernardofernandezz/ani-br/internal/infra/cache"
	"github.com/bernardofernandezz/ani-br/internal/infra/errorx"
	"github.com/bernardofernandezz/ani-br/internal/infra/logging"
	anilistProvider "github.com/bernardofernandezz/ani-br/internal/infra/provider/anilist"
	animesonlineProvider "github.com/bernardofernandezz/ani-br/internal/infra/provider/animesonline"
	mpvplayer "github.com/bernardofernandezz/ani-br/internal/infra/player/mpv"
	"github.com/bernardofernandezz/ani-br/internal/infra/provider"
	"github.com/bernardofernandezz/ani-br/internal/infra/provider/mockptbr"
	"github.com/bernardofernandezz/ani-br/internal/infra/storage/mem"
	"github.com/bernardofernandezz/ani-br/internal/ui/cli"
	"github.com/bernardofernandezz/ani-br/internal/ui/tui"
	"github.com/bernardofernandezz/ani-br/internal/usecase/continuewatching"
	"github.com/bernardofernandezz/ani-br/internal/usecase/download"
	"github.com/bernardofernandezz/ani-br/internal/usecase/history"
	"github.com/bernardofernandezz/ani-br/internal/usecase/playepisode"
	"github.com/bernardofernandezz/ani-br/internal/usecase/searchanime"
	"github.com/bernardofernandezz/ani-br/pkg/httputil"
	"github.com/bernardofernandezz/ani-br/pkg/robots"
	"github.com/spf13/viper"
)

var version = "dev"

func main() {
	ctx := context.Background()

	// Inicializa config antes do wiring para garantir precedência correta.
	_ = cli.InitConfig()

	app, err := buildApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitCode(err))
	}

	root := cli.NewRoot(app)
	root.Version = version

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitCode(err))
	}
}

func buildApp() (cli.App, error) {
	// Config já é inicializada pelo PersistentPreRunE do cobra, mas precisamos de
	// defaults para wiring (viper já terá defaults mesmo antes de ReadInConfig).

	httpClient := httputil.NewClient(httputil.DefaultConfig())
	robotsChecker := robots.NewChecker(nil)

	logger, cleanup, err := logging.NewLogger(logging.Config{Verbose: viper.GetBool("verbose")})
	if err != nil {
		return cli.App{}, err
	}
	_ = cleanup
	slog.SetDefault(logger)

	timeout, _ := time.ParseDuration(viper.GetString("providers.timeout"))
	ani := anilistProvider.New(httpClient)

	animesonlineBaseURL := viper.GetString("providers.animesonline_base_url")
	if animesonlineBaseURL == "" {
		animesonlineBaseURL = "https://animesonlinecc.to"
	}
	userAgent := viper.GetString("providers.user_agent")
	animesonline, err := animesonlineProvider.New(httpClient, animesonlineBaseURL, userAgent)
	if err != nil {
		return cli.App{}, err
	}

	baseURL := viper.GetString("providers.mockptbr_base_url")
	mock, err := mockptbr.New(baseURL, httpClient, robotsChecker)
	if err != nil {
		return cli.App{}, err
	}

	// Prioridade PT-BR: AnimesOnline primeiro, depois AniList (metadados), depois mock.
	reg := provider.NewRegistry(timeout, animesonline, ani, mock)

	c, err := cache.NewLRUCache[searchanime.SearchKey, []domain.Anime](viper.GetInt("cache.max_size"))
	if err != nil {
		return cli.App{}, err
	}
	ttl, _ := time.ParseDuration(viper.GetString("cache.ttl"))
	searchSvc := searchanime.New(reg, c, ttl)

	historyStore := mem.NewHistoryStore()
	progressStore := mem.NewProgressStore()
	historySvc := history.New(historyStore)
	continueSvc := continuewatching.New(historyStore, progressStore)
	downloadSvc := download.New(httpClient, 4)

	player, err := mpvplayer.New()
	if err != nil {
		return cli.App{}, err
	}
	// animesonline implementa domain.StreamResolver; episódios sem Streams são resolvidos na hora do play.
	playSvc := playepisode.New(reg, player, animesonline)

	app := cli.App{
		SearchCmd: func(ctx context.Context, query string) error {
			animes, err := searchSvc.Execute(ctx, query, domain.Language(viper.GetString("preferences.language")))
			if err != nil {
				if errors.Is(err, domain.ErrAnimeNotFound) {
					fmt.Println("Nenhum anime encontrado.")
					return nil
				}
				return err
			}
			fmt.Println("Título | Provedor | Idioma")
			fmt.Println("-------|----------|--------")
			for _, a := range animes {
				lang := string(a.PreferredLang)
				if lang == "" {
					lang = "pt-BR"
				}
				fmt.Printf("%s | %s | %s\n  id=%s\n", a.Title, a.Provider, lang, a.ID)
			}
			fmt.Println("\nPara listar episódios: ani-br episodes --id <id>")
			fmt.Println("Para reproduzir: ani-br play --id <id> --ep <n>")
			return nil
		},
		EpisodesCmd: func(ctx context.Context, animeID string) error {
			episodes, err := reg.GetEpisodes(ctx, animeID)
			if err != nil {
				if errors.Is(err, domain.ErrEpisodeNotFound) {
					fmt.Println("Nenhum episódio encontrado para este anime.")
					return nil
				}
				return err
			}
			fmt.Printf("Episódios para %s:\n", animeID)
			for _, ep := range episodes {
				fmt.Printf("  %d - %s\n", ep.EpisodeNumber, ep.Title)
			}
			fmt.Println("\nPara reproduzir: ani-br play --id", animeID, "--ep <número>")
			return nil
		},
		RunTUI: func(ctx context.Context) error {
			lang := domain.Language(viper.GetString("preferences.language"))
			q := domain.Quality(viper.GetString("preferences.quality"))
			playFn := func(pctx context.Context, animeID string, episodeNumber int) error {
				return playSvc.PlayByNumber(pctx, animeID, episodeNumber, lang, q)
			}
			prog := tui.NewProgram(searchSvc, reg, playFn, lang)
			_, err := prog.Run()
			return err
		},
		HistoryCmd: func(ctx context.Context, limit int) error {
			items, err := historySvc.List(ctx, limit)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				fmt.Println("Histórico vazio.")
				return nil
			}
			for i, it := range items {
				fmt.Printf("%02d. anime=%s ep=%s %s\n", i+1, it.AnimeID, it.EpisodeID, it.WatchedAt.Format(time.RFC3339))
			}
			return nil
		},
		ContinueCmd: func(ctx context.Context) error {
			last, err := continueSvc.GetLast(ctx)
			if err != nil {
				if errors.Is(err, domain.ErrAnimeNotFound) {
					fmt.Println("Nada para continuar.")
					return nil
				}
				return err
			}
			p, err := continueSvc.GetProgress(ctx, last.EpisodeID)
			if err != nil && !errors.Is(err, domain.ErrProgressNotFound) {
				return err
			}
			fmt.Printf("Último: anime=%s ep=%s posição=%s\n", last.AnimeID, last.EpisodeID, p.Position)
			return nil
		},
		DownloadURLCmd: func(ctx context.Context, url string, destDir string) error {
			ep := domain.Episode{
				ID:            "url",
				AnimeID:       "url",
				EpisodeNumber: 1,
				Title:         "download",
				Streams: []domain.Stream{
					{URL: url, Quality: domain.QualityAuto, Language: domain.LanguagePTBRDub},
				},
			}
			out, err := downloadSvc.DownloadEpisode(ctx, ep, domain.Language(viper.GetString("preferences.language")), domain.Quality(viper.GetString("preferences.quality")), destDir)
			if err != nil {
				return err
			}
			fmt.Println("Salvo em:", out)
			return nil
		},
		PlayCmd: func(ctx context.Context, animeID string, episode int) error {
			lang := domain.Language(viper.GetString("preferences.language"))
			q := domain.Quality(viper.GetString("preferences.quality"))
			return playSvc.PlayByNumber(ctx, animeID, episode, lang, q)
		},
	}

	return app, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var app *errorx.AppError
	if errors.As(err, &app) {
		if app.Code == errorx.CodeDependencyMissing {
			return 3
		}
		if app.Code == errorx.CodeInvalidArgument {
			return 2
		}
	}
	return 1
}

