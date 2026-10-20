package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

// fakeGitHub imite l'API GitHub et les fichiers des versions (httptest,
// TLS : le client refuse tout ce qui n'est pas https).
type fakeGitHub struct {
	srv      *httptest.Server
	mu       sync.Mutex
	releases []Release
	files    map[string][]byte // chemin → contenu
	agents   []string
	cookies  int
	hits     map[string]int
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{files: map[string][]byte{}, hits: map[string]int{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.agents = append(f.agents, r.Header.Get("User-Agent"))
	f.cookies += len(r.Cookies())
	f.hits[r.URL.Path]++
	switch {
	case r.URL.Path == "/repos/"+Repo+"/releases":
		_ = json.NewEncoder(w).Encode(f.releases)
	case strings.HasPrefix(r.URL.Path, "/repos/"+Repo+"/releases/tags/"):
		tag := strings.TrimPrefix(r.URL.Path, "/repos/"+Repo+"/releases/tags/")
		for _, rel := range f.releases {
			if rel.Tag == tag {
				_ = json.NewEncoder(w).Encode(rel)
				return
			}
		}
		http.NotFound(w, r)
	case r.URL.Path == "/redirect-http":
		http.Redirect(w, r, "http://example.invalid/x", http.StatusFound)
	default:
		b, ok := f.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		_, _ = w.Write(b)
	}
}

// publish ajoute une version avec ses fichiers et, si manifest, son
// latest.json (comme scripts/release-manifest.sh).
func (f *fakeGitHub) publish(tag string, prerelease, manifest bool, files map[string][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rel := Release{Tag: tag, Prerelease: prerelease, URL: "https://github.com/" + Repo + "/releases/tag/" + tag}
	m := Manifest{Version: strings.TrimPrefix(tag, "v"), Tag: tag, Prerelease: prerelease, ReleaseURL: rel.URL, Files: map[string]ManifestFile{}}
	for name, b := range files {
		p := "/download/" + tag + "/" + name
		f.files[p] = b
		sum := sha256.Sum256(b)
		rel.Assets = append(rel.Assets, Asset{Name: name, URL: f.srv.URL + p, Size: int64(len(b))})
		m.Files[name] = ManifestFile{Name: name, URL: f.srv.URL + p, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(b))}
	}
	if manifest {
		b, _ := json.Marshal(m)
		p := "/download/" + tag + "/" + ManifestName
		f.files[p] = b
		rel.Assets = append(rel.Assets, Asset{Name: ManifestName, URL: f.srv.URL + p, Size: int64(len(b))})
	}
	f.releases = append(f.releases, rel)
}

func (f *fakeGitHub) client(version string) *Client {
	return &Client{HTTP: f.srv.Client(), API: f.srv.URL, Version: version}
}

func TestNewestChannel(t *testing.T) {
	rel := func(tag string, pre, draft bool) Release { return Release{Tag: tag, Prerelease: pre, Draft: draft} }
	list := []Release{
		rel("v0.2.0-beta", true, false),
		rel("v0.3.0-beta", true, false),
		rel("v0.3.0-beta.2", true, false),
		rel("v0.4.0-beta", true, true), // brouillon
		rel("nightly", true, false),    // pas une version
		rel("v0.2.1", false, false),
	}
	cur := func(s string) Version { v, _ := ParseVersion(s); return v }
	if r := Newest(list, cur("0.2.0-beta")); r == nil || r.Tag != "v0.3.0-beta.2" {
		t.Errorf("bêta : %+v, attendu v0.3.0-beta.2", r)
	}
	if r := Newest(list, cur("0.2.0")); r == nil || r.Tag != "v0.2.1" {
		t.Errorf("publiée : %+v, attendu v0.2.1 (pas de pré-version)", r)
	}
	if r := Newest(list, cur("0.3.0-beta.2")); r != nil {
		t.Errorf("déjà à jour : %+v", r)
	}
	if r := Newest(list, cur("1.0.0")); r != nil {
		t.Errorf("jamais de retour en arrière : %+v", r)
	}
}

func TestAssetFor(t *testing.T) {
	for k, want := range map[Kind]string{KindUser: AssetUserSetup, KindMachine: AssetMachineSetup, KindPortable: AssetPortable} {
		if got, ok := AssetFor(k); !ok || got != want {
			t.Errorf("%s → %q, attendu %q", k, got, want)
		}
	}
	for _, k := range []Kind{KindMSIX, KindOther, ""} {
		if _, ok := AssetFor(k); ok || k.Automatic() {
			t.Errorf("%s : pas de mise à jour automatique", k)
		}
	}
	if !KindMachine.NeedsElevation() || KindUser.NeedsElevation() || KindPortable.NeedsElevation() {
		t.Error("seule l'installation par machine demande l'élévation")
	}
}

func TestKindFor(t *testing.T) {
	in := Installs{User: `C:\Users\Ana\AppData\Local\Programs\Lattis`, Machine: `C:\Program Files\Lattis\Lattis\`}
	cases := []struct {
		exe      string
		packaged bool
		want     Kind
	}{
		{`C:\Users\Ana\AppData\Local\Programs\Lattis\lattis.exe`, false, KindUser},
		{`c:\program files\lattis\lattis\LATTIS.EXE`, false, KindMachine},
		{`D:\Outils\Lattis-windows-x64-portable.exe`, false, KindPortable},
		{`C:\Program Files\WindowsApps\Lattis_1.0.0.0_x64\lattis.exe`, true, KindMSIX},
	}
	for _, c := range cases {
		if got := kindFor(c.exe, in, c.packaged); got != c.want {
			t.Errorf("%s → %s, attendu %s", c.exe, got, c.want)
		}
	}
	if got := kindFor(`C:\x\lattis.exe`, Installs{}, false); got != KindPortable {
		t.Errorf("sans installation → %s", got)
	}
}

func TestInstallDir(t *testing.T) {
	cases := [][3]string{
		{`C:\Apps\Lattis`, `"C:\Other\uninstall.exe"`, `C:\Apps\Lattis`},
		{``, `"C:\Program Files\Lattis\Lattis\uninstall.exe"`, `C:\Program Files\Lattis\Lattis`},
		{``, `C:\Users\Ana\AppData\Local\Programs\Lattis\uninstall.exe /S`, `C:\Users\Ana\AppData\Local\Programs\Lattis`},
		{``, ``, ``},
	}
	for _, c := range cases {
		if got := installDir(c[0], c[1]); got != c[2] {
			t.Errorf("installDir(%q, %q) = %q, attendu %q", c[0], c[1], got, c[2])
		}
	}
}

func TestParseManifestAndSums(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	b := []byte(`{"version":"0.3.0-beta","tag":"v0.3.0-beta","files":{"windows-setup":{"name":"Lattis-windows-x64-setup.exe","sha256":"` + strings.ToUpper(sum) + `","size":42}}}`)
	m, err := ParseManifest(b, "v0.3.0-beta")
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := m.File("lattis-windows-x64-setup.exe"); !ok || e.SHA256 != sum || e.Size != 42 {
		t.Errorf("entrée : %+v %v", e, ok)
	}
	if _, ok := m.File(AssetPortable); ok {
		t.Error("fichier absent du manifeste")
	}
	if _, err := ParseManifest(b, "v0.4.0-beta"); err == nil {
		t.Error("manifeste d'une autre version accepté")
	}
	if _, err := ParseManifest([]byte("{"), "v0.3.0-beta"); err == nil {
		t.Error("manifeste illisible accepté")
	}
	sums := []byte(sum + "  Lattis-windows-x64-setup.exe\n" + strings.Repeat("cd", 32) + " *Lattis-windows-x64-portable.exe\nnot a line\n")
	if e, ok := ParseSums(sums, AssetPortable); !ok || e.SHA256 != strings.Repeat("cd", 32) {
		t.Errorf("SHA256SUMS : %+v %v", e, ok)
	}
	if _, ok := ParseSums(sums, AssetMachineSetup); ok {
		t.Error("fichier absent de SHA256SUMS")
	}
}

func TestClientHTTPS(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("setup")})
	c := gh.client("0.2.0-beta")
	list, err := c.Releases(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("Releases : %v %v", list, err)
	}
	if ua := gh.agents[0]; ua != "Lattis/0.2.0-beta ("+runtimeGOOS()+")" || gh.cookies != 0 {
		t.Errorf("User-Agent %q, cookies %d : rien d'autre que la version et le système", ua, gh.cookies)
	}
	// Adresse ou redirection en http:// : refusée.
	if _, err := c.getSmall(context.Background(), strings.Replace(gh.srv.URL, "https", "http", 1)+"/x", "", 10); msgID(err) != "update.notHttps" {
		t.Errorf("http:// : %v", err)
	}
	if _, err := c.getSmall(context.Background(), gh.srv.URL+"/redirect-http", "", 10); err == nil {
		t.Error("redirection vers http:// suivie")
	}
	if _, err := c.getSmall(context.Background(), gh.srv.URL+"/nothing", "", 10); msgID(err) != "update.httpStatus" {
		t.Errorf("404 : %v", err)
	}
}

func TestDownloadVerifies(t *testing.T) {
	gh := newFakeGitHub(t)
	payload := []byte("installeur Lattis 0.3.0-beta")
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: payload})
	c := gh.client("0.2.0-beta")
	ctx := context.Background()
	rel, err := c.ReleaseByTag(ctx, "v0.3.0-beta")
	if err != nil {
		t.Fatal(err)
	}
	want, err := c.Expected(ctx, rel, AssetUserSetup)
	if err != nil || want.Size != int64(len(payload)) {
		t.Fatalf("Expected : %+v %v", want, err)
	}
	a, _ := rel.Asset(AssetUserSetup)
	dir := t.TempDir()
	dest := filepath.Join(dir, "setup.exe")
	if err := c.Download(ctx, a.URL, dest, want); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != string(payload) {
		t.Errorf("contenu : %q", b)
	}
	// Mauvaise empreinte : refusé, rien ne reste.
	bad := want
	bad.SHA256 = strings.Repeat("0", 64)
	dest2 := filepath.Join(dir, "bad.exe")
	if err := c.Download(ctx, a.URL, dest2, bad); msgID(err) != "update.checksumMismatch" {
		t.Errorf("empreinte : %v", err)
	}
	// Mauvaise taille : refusé.
	bad = want
	bad.Size++
	if err := c.Download(ctx, a.URL, dest2, bad); msgID(err) != "update.sizeMismatch" {
		t.Errorf("taille : %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*")); len(left) != 1 {
		t.Errorf("fichiers restants : %v", left)
	}
	// Sans empreinte : jamais téléchargé.
	if err := c.Download(ctx, a.URL, dest2, Expected{Name: "x"}); msgID(err) != "update.noChecksum" {
		t.Errorf("sans empreinte : %v", err)
	}
}

func TestExpectedFallsBackToSums(t *testing.T) {
	gh := newFakeGitHub(t)
	payload := []byte("portable")
	sum := sha256.Sum256(payload)
	gh.publish("v0.3.0-beta", true, false, map[string][]byte{
		AssetPortable: payload,
		SumsName:      []byte(hex.EncodeToString(sum[:]) + "  " + AssetPortable + "\n"),
	})
	c := gh.client("0.2.0-beta")
	rel, _ := c.ReleaseByTag(context.Background(), "v0.3.0-beta")
	e, err := c.Expected(context.Background(), rel, AssetPortable)
	if err != nil || e.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("SHA256SUMS : %+v %v", e, err)
	}
	if _, err := c.Expected(context.Background(), rel, AssetUserSetup); msgID(err) != "update.noChecksum" {
		t.Errorf("fichier sans empreinte : %v", err)
	}
}

func TestLaunched(t *testing.T) {
	st := State{}
	if prev, up := st.Launched("0.2.0-beta"); up || prev != "" {
		t.Error("premier lancement : pas une mise à jour")
	}
	if _, up := st.Launched("0.2.0-beta"); up {
		t.Error("même version : pas une mise à jour")
	}
	st.Pending = &Pending{Version: "0.3.0-beta", File: "x"}
	st.ApplyError = "refusé"
	st.Latest = "v0.3.0-beta"
	if prev, up := st.Launched("0.3.0-beta"); !up || prev != "0.2.0-beta" {
		t.Errorf("mise à jour : %q %v", prev, up)
	}
	if st.Pending != nil || st.ApplyError != "" || st.Latest != "" {
		t.Errorf("mise à jour appliquée, à oublier : %+v", st)
	}
}

// newTestUpdater : un Updater relié au faux GitHub, état et téléchargements
// dans un dossier temporaire.
func newTestUpdater(t *testing.T, gh *fakeGitHub, current string, kind Kind) *Updater {
	dir := t.TempDir()
	exe := filepath.Join(dir, "app", "lattis.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("ancienne version"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Updater{
		Current:   current,
		Client:    gh.client(current),
		StateFile: filepath.Join(dir, "config", "update.json"),
		Dir:       filepath.Join(dir, "cache", "updates"),
		Exe:       exe,
		Kind:      kind,
	}
}

func TestFirstLaunchAfterUpdate(t *testing.T) {
	gh := newFakeGitHub(t)
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Init()
	if s := u.Status(msg.FR); s.Updated != "" {
		t.Errorf("premier lancement : %+v", s)
	}
	// Lancement suivant, nouvelle version.
	u2 := newTestUpdater(t, gh, "0.3.0-beta", KindUser)
	u2.StateFile = u.StateFile
	u2.Init()
	s := u2.Status(msg.FR)
	if s.Updated != "0.3.0-beta" || s.UpdatedPage != "https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.0-beta" {
		t.Errorf("après la mise à jour : %+v", s)
	}
	u2.DismissUpdated()
	if s := u2.Status(msg.FR); s.Updated != "" {
		t.Errorf("avis fermé : %+v", s)
	}
	// Le lancement d'après ne l'annonce plus.
	u3 := newTestUpdater(t, gh, "0.3.0-beta", KindUser)
	u3.StateFile = u.StateFile
	u3.Init()
	if s := u3.Status(msg.FR); s.Updated != "" {
		t.Errorf("annoncée une seule fois : %+v", s)
	}
}

func TestCheckPrepareInstaller(t *testing.T) {
	gh := newFakeGitHub(t)
	user, machine := []byte("setup utilisateur"), []byte("setup machine")
	gh.publish("v0.2.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("vieux")})
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: user, AssetMachineSetup: machine, AssetPortable: []byte("exe")})
	ctx := context.Background()
	for _, c := range []struct {
		kind Kind
		want []byte
	}{{KindUser, user}, {KindMachine, machine}} {
		u := newTestUpdater(t, gh, "0.2.0-beta", c.kind)
		var events []Status
		u.OnChange = func(s Status) { events = append(events, s) }
		u.Init()
		if err := u.Check(ctx); err != nil {
			t.Fatal(err)
		}
		s := u.Status(msg.FR)
		if s.Phase != PhaseAvailable || s.Version != "0.3.0-beta" || s.Elevation != (c.kind == KindMachine) {
			t.Fatalf("%s : après la recherche %+v", c.kind, s)
		}
		if err := u.Prepare(ctx); err != nil {
			t.Fatalf("%s : %v", c.kind, err)
		}
		s = u.Status(msg.FR)
		if s.Phase != PhaseReady || s.Version != "0.3.0-beta" || s.Page == "" {
			t.Fatalf("%s : prête %+v", c.kind, s)
		}
		p := u.st.Pending
		if b, _ := os.ReadFile(p.File); string(b) != string(c.want) {
			t.Errorf("%s : fichier %s = %q", c.kind, p.File, b)
		}
		if p.Dir != filepath.Dir(u.Exe) || p.Kind != c.kind {
			t.Errorf("%s : %+v", c.kind, p)
		}
		phases := []Phase{}
		for _, e := range events {
			phases = append(phases, e.Phase)
		}
		if strings.Join(phasesText(phases), ",") != "available,downloading,ready" {
			t.Errorf("%s : évènements %v", c.kind, phases)
		}
		// Relancée avant d'avoir appliqué : toujours prête, fichier contrôlé.
		u2 := newTestUpdater(t, gh, "0.2.0-beta", c.kind)
		u2.StateFile, u2.Dir = u.StateFile, u.Dir
		u2.Init()
		if s := u2.Status(msg.FR); s.Phase != PhaseReady {
			t.Errorf("%s : au relancement %+v", c.kind, s)
		}
		// Fichier modifié depuis : oublié.
		os.WriteFile(p.File, []byte("altéré"), 0o644)
		u3 := newTestUpdater(t, gh, "0.2.0-beta", c.kind)
		u3.StateFile, u3.Dir = u.StateFile, u.Dir
		u3.Init()
		if s := u3.Status(msg.FR); s.Phase == PhaseReady {
			t.Errorf("%s : fichier altéré encore proposé", c.kind)
		}
	}
}

func phasesText(p []Phase) []string {
	out := []string{}
	for _, x := range p {
		out = append(out, string(x))
	}
	return out
}

func TestPrepareRejectsBadChecksum(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("bon")})
	// Le fichier servi n'est plus celui du manifeste.
	gh.files["/download/v0.3.0-beta/"+AssetUserSetup] = []byte("mau")
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Init()
	ctx := context.Background()
	if err := u.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := u.Prepare(ctx); msgID(err) != "update.checksumMismatch" {
		t.Fatalf("Prepare : %v", err)
	}
	s := u.Status(msg.EN)
	if s.Phase != PhaseFailed || !strings.Contains(s.Error, "checksum") {
		t.Errorf("échec : %+v", s)
	}
	if u.st.Pending != nil {
		t.Error("rien ne doit être prêt")
	}
	if left, _ := filepath.Glob(filepath.Join(u.Dir, "*", "*")); len(left) != 0 {
		t.Errorf("fichiers laissés : %v", left)
	}
}

func TestPrepareSignatureRequired(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("non signé")})
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Verify = func(string) error { return fmt.Errorf("pas de signature") }
	u.Init()
	ctx := context.Background()
	_ = u.Check(ctx)
	if err := u.Prepare(ctx); msgID(err) != "update.notSigned" {
		t.Fatalf("Prepare : %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(u.Dir, "*", "*")); len(left) != 0 {
		t.Errorf("fichier non signé gardé : %v", left)
	}
}

func TestPreparePortable(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetPortable: []byte("nouvelle version")})
	u := newTestUpdater(t, gh, "0.2.0-beta", KindPortable)
	u.Init()
	ctx := context.Background()
	_ = u.Check(ctx)
	if err := u.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	p := u.st.Pending
	if p == nil || p.File != u.Exe+".new" {
		t.Fatalf("portable : téléchargé à côté de l'exécutable, %+v", p)
	}
	// Remplacement (ce que fait apply sous Windows, sans relance).
	if err := replaceExe(u.Exe, p.File); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(u.Exe); string(b) != "nouvelle version" {
		t.Errorf("exécutable : %q", b)
	}
	if b, _ := os.ReadFile(u.Exe + ".old"); string(b) != "ancienne version" {
		t.Errorf("ancien : %q", b)
	}
	// Lancement suivant : l'ancien est rangé.
	u2 := newTestUpdater(t, gh, "0.3.0-beta", KindPortable)
	u2.Exe, u2.StateFile = u.Exe, u.StateFile
	u2.Init()
	if _, err := os.Stat(u.Exe + ".old"); !os.IsNotExist(err) {
		t.Error(".old non supprimé")
	}
	if s := u2.Status(msg.FR); s.Updated != "0.3.0-beta" || s.Phase != PhaseIdle {
		t.Errorf("après : %+v", s)
	}
}

func TestOtherKindIsManual(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, nil)
	u := newTestUpdater(t, gh, "0.2.0-beta", KindOther)
	u.Init()
	_ = u.Check(context.Background())
	s := u.Status(msg.FR)
	if s.Phase != PhaseManual || s.Page != "https://github.com/Be-Lattis/lattis-app/releases/tag/v0.3.0-beta" {
		t.Errorf("hors Windows : %+v", s)
	}
	if err := u.Prepare(context.Background()); err != nil || u.Status(msg.FR).Phase != PhaseManual {
		t.Error("rien à télécharger hors Windows")
	}
}

func TestDisabledAndDev(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("x")})
	on := true
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Enabled = func() bool { return on }
	u.Init()
	_ = u.Check(context.Background())
	on = false
	if s := u.Status(msg.FR); s.Phase != PhaseIdle {
		t.Errorf("recherche coupée : plus de proposition, %+v", s)
	}
	dev := newTestUpdater(t, gh, "dev", KindUser)
	if dev.Checkable() {
		t.Error("une compilation dev ne cherche pas")
	}
	msix := newTestUpdater(t, gh, "0.2.0-beta", KindMSIX)
	if msix.Checkable() {
		t.Error("MSIX : mis à jour par Windows")
	}
	// Run sans recherche possible : rend la main aussitôt.
	done := make(chan struct{})
	go func() { dev.Run(context.Background(), time.Hour); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run d'une compilation dev bloqué")
	}
	gh.mu.Lock()
	defer gh.mu.Unlock()
	if gh.hits["/repos/"+Repo+"/releases"] != 1 {
		t.Errorf("requêtes : %v", gh.hits)
	}
}

func TestCheckEveryAndRestartWindow(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetUserSetup: []byte("x")})
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Now = func() time.Time { return now }
	u.Init()
	if !u.due() {
		t.Error("jamais cherché : à faire")
	}
	_ = u.Check(context.Background())
	now = now.Add(CheckEvery - time.Minute)
	if u.due() {
		t.Error("cherché il y a moins de 12 h")
	}
	now = now.Add(2 * time.Minute)
	if !u.due() {
		t.Error("cherché il y a plus de 12 h")
	}
	u.RequestRestart()
	if !(!u.restartAt.IsZero() && u.now().Sub(u.restartAt) < restartWindow) {
		t.Error("relance demandée")
	}
}

func msgID(err error) msg.ID {
	var e *msg.Error
	if errors.As(err, &e) {
		return e.ID
	}
	return ""
}

func runtimeGOOS() string { return runtime.GOOS }
