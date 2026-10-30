package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"
)

// Les clés publiées dans le code sont toutes lisibles : une clé mal
// collée ne doit pas désactiver la vérification sans bruit.
func TestPublishedKeys(t *testing.T) {
	for _, s := range publishedKeys {
		if _, err := ParsePublicKey(s); err != nil {
			t.Errorf("clé publique %q : %v", s, err)
		}
	}
	if len(ManifestKeys()) != len(publishedKeys) {
		t.Error("une clé publiée est ignorée")
	}
}

func TestSignVerifyManifest(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	body := []byte(`{"version":"0.3.0-beta"}`)
	sig := SignManifest(priv, body)
	if !VerifyManifest(body, sig, []ed25519.PublicKey{other, pub}) {
		t.Fatal("signature valide refusée")
	}
	if VerifyManifest(body, sig, []ed25519.PublicKey{other}) {
		t.Error("signature d'une autre clé acceptée")
	}
	if VerifyManifest([]byte(`{"version":"9.9.9"}`), sig, []ed25519.PublicKey{pub}) {
		t.Error("manifeste modifié accepté")
	}
	if VerifyManifest(body, []byte("pas du base64 !"), []ed25519.PublicKey{pub}) || VerifyManifest(body, nil, []ed25519.PublicKey{pub}) {
		t.Error("signature illisible acceptée")
	}
	if VerifyManifest(body, sig, nil) {
		t.Error("sans clé, rien n'est vérifié")
	}
	// Clé privée : graine (keygen) ou clé complète ; clé publique : 32 octets.
	seed := base64.StdEncoding.EncodeToString(priv.Seed())
	for _, s := range []string{seed, " " + seed + "\n", base64.StdEncoding.EncodeToString(priv)} {
		k, err := ParsePrivateKey(s)
		if err != nil || !k.Public().(ed25519.PublicKey).Equal(pub) {
			t.Errorf("clé privée %q : %v", s, err)
		}
	}
	bad := append([]byte{}, priv...)
	bad[40] ^= 1
	if _, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(bad)); err == nil {
		t.Error("clé complète incohérente acceptée")
	}
	if _, err := ParsePublicKey(base64.StdEncoding.EncodeToString(pub[:31])); err == nil {
		t.Error("clé publique tronquée acceptée")
	}
}

// signRelease publie latest.json.sig pour la version tag (comme la CI).
func (f *fakeGitHub) signRelease(tag string, key ed25519.PrivateKey) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.files["/download/"+tag+"/"+ManifestName]
	p := "/download/" + tag + "/" + SignatureName
	f.files[p] = SignManifest(key, m)
	for i := range f.releases {
		if f.releases[i].Tag == tag {
			f.releases[i].Assets = append(f.releases[i].Assets, Asset{Name: SignatureName, URL: f.srv.URL + p})
		}
	}
}

func TestSignedManifestRequired(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	payload := []byte("installeur signé")
	sum := sha256.Sum256(payload)
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetSetup: payload})
	gh.signRelease("v0.3.0-beta", priv)
	// Non signée, avec SHA256SUMS.txt : acceptée sans clé, refusée avec.
	gh.publish("v0.3.1-beta", true, true, map[string][]byte{AssetSetup: payload, SumsName: []byte(hex.EncodeToString(sum[:]) + "  " + AssetSetup + "\n")})
	// Signée par une autre clé.
	gh.publish("v0.3.2-beta", true, true, map[string][]byte{AssetSetup: payload})
	gh.signRelease("v0.3.2-beta", other)
	// Signée, mais latest.json remplacé ensuite (empreinte d'un autre fichier).
	gh.publish("v0.3.3-beta", true, true, map[string][]byte{AssetSetup: payload})
	gh.signRelease("v0.3.3-beta", priv)
	gh.files["/download/v0.3.3-beta/"+ManifestName] = []byte(`{"version":"0.3.3-beta","tag":"v0.3.3-beta","files":{"windows-setup":{"name":"` + AssetSetup + `","sha256":"` + hex.EncodeToString(make([]byte, 32)) + `"}}}`)
	// Sans latest.json, seulement SHA256SUMS.txt.
	gh.publish("v0.3.4-beta", true, false, map[string][]byte{AssetSetup: payload, SumsName: []byte(hex.EncodeToString(sum[:]) + "  " + AssetSetup + "\n")})

	ctx := context.Background()
	c := gh.client("0.2.0-beta")
	c.Keys = []ed25519.PublicKey{pub}
	expect := func(tag, wantErr string) {
		t.Helper()
		rel, err := c.ReleaseByTag(ctx, tag)
		if err != nil {
			t.Fatal(err)
		}
		e, err := c.Expected(ctx, rel, AssetSetup)
		if wantErr == "" {
			if err != nil || e.SHA256 != hex.EncodeToString(sum[:]) {
				t.Errorf("%s : %+v %v", tag, e, err)
			}
			return
		}
		if string(msgID(err)) != wantErr {
			t.Errorf("%s : %v, attendu %s", tag, err, wantErr)
		}
	}
	expect("v0.3.0-beta", "")
	expect("v0.3.1-beta", "update.unsigned")
	expect("v0.3.2-beta", "update.badSignature")
	expect("v0.3.3-beta", "update.badSignature")
	expect("v0.3.4-beta", "update.unsigned")

	// Sans clé (versions publiées jusqu'ici) : comme avant.
	c.Keys = nil
	expect("v0.3.1-beta", "")
	expect("v0.3.4-beta", "")

	// Jusqu'à l'installeur : téléchargé et contrôlé avec le manifeste signé.
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Client.Keys = []ed25519.PublicKey{pub}
	u.Init()
	u.st.Latest = "v0.3.0-beta"
	u.phase = PhaseAvailable
	if err := u.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(u.st.Pending.File); string(b) != string(payload) {
		t.Errorf("installeur : %q", b)
	}
	u2 := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u2.Client.Keys = []ed25519.PublicKey{pub}
	u2.Init()
	u2.st.Latest = "v0.3.1-beta"
	u2.phase = PhaseAvailable
	if err := u2.Prepare(ctx); msgID(err) != "update.unsigned" || u2.st.Pending != nil {
		t.Errorf("version non signée : %v", err)
	}
}
