package bridge

import (
	"context"
	"path/filepath"
	"slices"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
)

// Démo publique (lattis-engine --public-demo, servie par le Serveur Lattis
// sur /demo/ sans connexion) : un moteur jetable par navigateur. Rien n'est
// gardé ni ne sort :
//
//   - pas de Serveur Lattis (session, publication, Data Lake), pas
//     d'assistant IA, pas de trousseau (mots de passe et clés en mémoire) ;
//   - pas de connexion sortante (query.Guard.Offline, posé par le moteur) ;
//   - Enregistrer télécharge le classeur dans le navigateur ;
//   - les récents sont les classeurs de démonstration (DemoPresets), jamais
//     la liste du poste, qui n'est pas écrite ; pas de récupération proposée.

// publicDemo : moteur d'une démo publique (UsePublicDemo).
var publicDemo atomic.Bool

// errDemo refuse ce que la démo n'offre pas.
var errDemo = msg.New("demo.unavailable", nil)

// UsePublicDemo passe les services en démo publique (au démarrage du moteur,
// avant toute session) : coffres en mémoire, sans trousseau. Faux : retour
// au comportement habituel (tests ; les coffres restent en mémoire).
func UsePublicDemo(on bool) {
	if on {
		query.DefaultSecrets = query.NewSecrets(false, "")
		ai.DefaultKeys = ai.NewKeys(false)
	}
	publicDemo.Store(on)
}

// PublicDemo dit si le moteur est une démo publique.
func PublicDemo() bool { return publicDemo.Load() }

// DemoPreset est un classeur de démonstration proposé dans les récents de
// la démo publique : Preset (internal/demo) et sa taille affichée.
type DemoPreset struct {
	Preset string
	Size   string
}

// DemoPresets : les classeurs proposés par Fichier › Ouvrir › Récents dans
// la démo publique (demande de Louis : test small et test medium).
var DemoPresets = []DemoPreset{{"sml", "small"}, {"med", "medium"}}

// demoRecent : les récents de la démo publique.
func demoRecent() []RecentFile {
	out := make([]RecentFile, 0, len(DemoPresets))
	for _, p := range DemoPresets {
		out = append(out, RecentFile{Demo: p.Preset, Title: p.Size})
	}
	return out
}

// OpenPreset ouvre un classeur de démonstration des récents de la démo
// publique (DemoPresets), à la place du classeur ouvert ; comme OpenSample,
// il n'est jamais enregistré de lui-même. Hors démo publique, ou un autre
// preset : refusé (pas de gros classeur fabriqué à la demande d'une page).
func (s *DocumentService) OpenPreset(ctx context.Context, preset string) (DemoResult, error) {
	if !publicDemo.Load() || !slices.ContainsFunc(DemoPresets, func(p DemoPreset) bool { return p.Preset == preset }) {
		return DemoResult{}, errDemo
	}
	r := &demoRun{preset: preset}
	return s.openPreset(ctx, r)
}

// demoSave : Enregistrer, dans la démo publique, télécharge le classeur
// (écrit dans le répertoire de la session, puis envoyé au navigateur) ;
// rien n'est écrit ailleurs.
func (s *DocumentService) demoSave(ctx context.Context, d *doc.Document) (DocState, error) {
	if d.Path() == "" || d.ReadOnly() {
		// Premier enregistrement : SaveFile écrit le fichier dans la session
		// et le fait télécharger.
		st, err := s.saveAs(ctx, d, docBaseName(d)+".lattis")
		if err == nil && st.Conflict == nil && !st.Dirty {
			st.Notice = s.say("doc.demoDownloaded", nil)
		}
		return st, err
	}
	if err := saveDoc(ctx, d, ""); err != nil {
		return s.saveFailed(ctx, d, err)
	}
	src := d.Path()
	if _, err := s.sys().SaveFile(FileDialog{Title: "Enregistrer le classeur", Filename: filepath.Base(src), Ext: ".lattis"}, func(dst string) error {
		if samePath(dst, src) {
			return nil
		}
		return copyTo(src, dst)
	}); err != nil {
		return DocState{}, err
	}
	st, err := s.saved(ctx, d)
	if err == nil {
		st.Notice = s.say("doc.demoDownloaded", nil)
	}
	return st, err
}
