// Package bridge expose au frontend l'API Go de Lattis.
//
// C'est la seule couche qui connaît Wails : les autres paquets restent du Go
// ordinaire, testable sans fenêtre, et un changement d'API de Wails v3 (encore
// en bêta) ne touche que ce paquet.
package bridge

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

// DemoRows est la taille du tableau de démonstration du prototype.
const DemoRows = 1_000_000

// GridStatus décrit l'état du tableau de démonstration.
type GridStatus struct {
	State string    `json:"state"` // "loading", "generating", "ready" ou "error"
	Done  int       `json:"done"`  // lignes générées
	Total int       `json:"total"`
	Error string    `json:"error,omitempty"`
	Info  grid.Info `json:"info"`
	Path  string    `json:"path"`
}

// GridService sert la grille du prototype (spike 1.1).
type GridService struct {
	hostRef // hôte : bureau (Wails) ou web (platform.go)
	mu      sync.Mutex
	status  GridStatus
	db      *store.Store
	table   *grid.Table
	cancel  context.CancelFunc
}

var errNotReady = errors.New("tableau en cours de préparation")

// startup ouvre la base de travail et prépare le tableau en tâche de
// fond : la fenêtre s'affiche tout de suite et suit l'avancement par Status.
func (s *GridService) startup(ctx context.Context) error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "Lattis", "prototype")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "demo.sqlite")
	db, err := store.Open(path)
	if err != nil {
		return err
	}
	bg, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.db, s.cancel = db, cancel
	s.status = GridStatus{State: "loading", Total: DemoRows, Path: path}
	s.mu.Unlock()
	go s.prepare(bg, db)
	return nil
}

func (s *GridService) prepare(ctx context.Context, db *store.Store) {
	err := func() error {
		ok, err := grid.DemoExists(ctx, db)
		if err != nil {
			return err
		}
		if !ok {
			s.set(func(st *GridStatus) { st.State = "generating" })
			err := grid.CreateDemo(ctx, db, DemoRows, func(done int) {
				s.set(func(st *GridStatus) { st.Done = done })
			})
			if err != nil {
				return err
			}
		}
		s.set(func(st *GridStatus) { st.State, st.Done = "loading", DemoRows })
		t := grid.Demo(db)
		if err := t.Load(ctx); err != nil {
			return err
		}
		s.mu.Lock()
		s.table = t
		s.status.State, s.status.Info = "ready", t.Info()
		s.mu.Unlock()
		return nil
	}()
	if err != nil && ctx.Err() == nil {
		slog.Error("préparation du tableau", "err", err)
		s.set(func(st *GridStatus) { st.State, st.Error = "error", err.Error() })
	}
}

func (s *GridService) set(f func(*GridStatus)) {
	s.mu.Lock()
	f(&s.status)
	s.mu.Unlock()
}

// ServiceShutdown ferme la base.
func (s *GridService) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Status renvoie l'état de préparation du tableau.
func (s *GridService) Status() GridStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *GridService) ready() (*grid.Table, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.table == nil {
		return nil, errNotReady
	}
	return s.table, nil
}

// Window lit count lignes à partir de la ligne first.
func (s *GridService) Window(ctx context.Context, first, count int) (grid.Window, error) {
	t, err := s.ready()
	if err != nil {
		return grid.Window{}, err
	}
	return t.Window(ctx, first, count)
}

// SetCell enregistre une saisie et renvoie la valeur stockée.
func (s *GridService) SetCell(ctx context.Context, id int64, col int, raw string) (any, error) {
	t, err := s.ready()
	if err != nil {
		return nil, err
	}
	return t.SetCell(ctx, id, col, raw)
}

// BenchEnv nomme la variable d'environnement qui lance la mesure automatique
// de la grille : elle donne le fichier où écrire le résultat (JSON), puis
// l'application se ferme. Sert à comparer les moteurs web (WebView2,
// WebKitGTK, WKWebView) sur la même mesure.
const BenchEnv = "LATTIS_BENCH"

// BenchRequested indique si la mesure automatique est demandée.
func (s *GridService) BenchRequested() bool {
	return os.Getenv(BenchEnv) != ""
}

// ReportBench écrit le résultat de la mesure automatique et ferme l'application.
func (s *GridService) ReportBench(result string) error {
	path := os.Getenv(BenchEnv)
	if path == "" {
		return errors.New(BenchEnv + " n'est pas défini")
	}
	if err := os.WriteFile(path, []byte(result+"\n"), 0o644); err != nil {
		return err
	}
	s.sys().Window().Quit()
	return nil
}

// QueryLimit borne le nombre de lignes renvoyées à l'onglet SQL.
const QueryLimit = 1000

// Query exécute une requête SQL en lecture seule (onglet SQL).
func (s *GridService) Query(ctx context.Context, q string) (grid.QueryResult, error) {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	if db == nil {
		return grid.QueryResult{}, errNotReady
	}
	return grid.Query(ctx, db.Read(), q, QueryLimit)
}
