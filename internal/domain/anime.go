package domain

import (
	"context"
	"errors"
	"time"
)

// Language representa o idioma de áudio ou legenda.
type Language string

const (
	LanguageUnknown  Language = ""
	LanguagePTBRDub  Language = "pt-BR-dub"
	LanguagePTBRSub  Language = "pt-BR-sub"
	LanguageEN       Language = "en"
	LanguageES       Language = "es"
	LanguageJP       Language = "ja"
)

// Quality representa a qualidade de vídeo preferida.
type Quality string

const (
	QualityAuto   Quality = "auto"
	Quality480p   Quality = "480p"
	Quality720p   Quality = "720p"
	Quality1080p  Quality = "1080p"
	Quality1440p  Quality = "1440p"
	Quality2160p  Quality = "2160p"
)

// ProviderID identifica a origem dos dados de anime.
type ProviderID string

// StreamType descreve o tipo de stream de mídia.
type StreamType string

const (
	StreamTypeUnknown StreamType = ""
	StreamTypeHLS     StreamType = "hls"
	StreamTypeMP4     StreamType = "mp4"
)

// Anime representa uma obra de anime em qualquer provider.
type Anime struct {
	ID               string
	Title            string
	NormalizedTitle  string
	Synopsis         string
	Provider         ProviderID
	PreferredLang    Language
	PosterURL        string
	TotalEpisodes    int
	Seasons          []Season
	AlternateTitles  []string
	Genres           []string
	LastUpdatedAt    time.Time
	ProviderMetadata map[string]any
}

// Season representa uma temporada de um anime.
type Season struct {
	Number   int
	Title    string
	Episodes []Episode
}

// Episode representa um episódio de anime.
type Episode struct {
	ID            string
	AnimeID       string
	SeasonNumber  int
	EpisodeNumber int
	Title         string
	Synopsis      string
	Duration      time.Duration
	Streams       []Stream
	Subtitles     []Subtitle
	AirDate       *time.Time
}

// Stream representa uma opção de reprodução de vídeo.
// Headers opcionais (ex.: User-Agent, Referer) são usados pelo player para bypass 403.
type Stream struct {
	URL          string
	Quality      Quality
	Language     Language
	Type         StreamType
	Provider     ProviderID
	IsDefault    bool
	IsDubbed     bool
	IsSubtitled  bool
	SelectionTag string
	Headers      map[string]string
}

// Subtitle representa uma trilha de legenda.
type Subtitle struct {
	URL      string
	Language Language
	Format   string
	Default  bool
}

// AnimeRepository define as operações de leitura de animes e episódios.
// Implementações concretas vivem em camadas externas (infra/provider).
type AnimeRepository interface {
	SearchAnime(ctx context.Context, query string, lang Language) ([]Anime, error)
	GetEpisodes(ctx context.Context, animeID string) ([]Episode, error)
}

// HistoryRepository abstrai o histórico de animes assistidos.
type HistoryRepository interface {
	AddEntry(ctx context.Context, entry HistoryEntry) error
	ListEntries(ctx context.Context, limit int) ([]HistoryEntry, error)
}

// ProgressRepository abstrai o progresso de episódios.
type ProgressRepository interface {
	SaveProgress(ctx context.Context, progress Progress) error
	GetProgress(ctx context.Context, episodeID string) (Progress, error)
}

// PlayerPort representa a porta de saída para controle do player de mídia.
type PlayerPort interface {
	PlayEpisode(ctx context.Context, ep Episode, preferredLang Language, preferredQuality Quality) error
}

// StreamResolver resolve a URL de vídeo e headers a partir da página do player (ex.: AnimesOnline).
// Implementações ficam em infra; o usecase usa quando o episódio não traz Streams preenchidos.
type StreamResolver interface {
	ResolveStream(ctx context.Context, playerPageURL string) (Stream, error)
}

// HistoryEntry representa um item de histórico de visualização.
type HistoryEntry struct {
	AnimeID       string
	EpisodeID     string
	Provider      ProviderID
	WatchedAt     time.Time
	Completed     bool
	LastPosition  time.Duration
	PreferredLang Language
}

// Progress representa o progresso detalhado de um episódio.
type Progress struct {
	EpisodeID string
	Position  time.Duration
	Duration  time.Duration
	UpdatedAt time.Time
}

// Erros de domínio sentinela.
var (
	ErrAnimeNotFound    = errors.New("anime não encontrado")
	ErrEpisodeNotFound  = errors.New("episódio não encontrado")
	ErrStreamUnavailable = errors.New("nenhum stream disponível para o episódio")
	ErrProgressNotFound = errors.New("progresso não encontrado")
)

