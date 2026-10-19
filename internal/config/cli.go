package config

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const usage = `Utilisation : lattis config [commande]

  (aucune), show [--json] [--set chemin=valeur…]
                        configuration effective, provenance de chaque clé
                        et clés imposées par une stratégie (🔒)
  check FICHIER         valide un fichier de configuration avant déploiement
  refresh               télécharge la configuration distante (configUrl)
  keygen PRIVÉE.pem     crée une paire de clés Ed25519 ; écrit la clé privée
                        et affiche la clé publique (remote.publicKey)
  sign PRIVÉE.pem FICHIER
                        signe un fichier de configuration distante (JWS)
`

// Run exécute « lattis config … » ; renvoie le code de sortie.
func Run(args []string, stdout, stderr io.Writer) int {
	cmd := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "show":
		err = runShow(args, stdout)
	case "check":
		if len(args) != 1 {
			err = errors.New("check : un fichier attendu")
			break
		}
		return runCheck(args[0], stdout, stderr)
	case "refresh":
		err = runRefresh(stdout)
	case "keygen":
		if len(args) != 1 {
			err = errors.New("keygen : chemin de la clé privée attendu")
			break
		}
		err = runKeygen(args[0], stdout)
	case "sign":
		if len(args) != 2 {
			err = errors.New("sign : clé privée et fichier attendus")
			break
		}
		err = runSign(args[0], args[1], stdout)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		err = fmt.Errorf("commande inconnue %q", cmd)
	}
	if err != nil {
		fmt.Fprintf(stderr, "lattis config : %v\n\n%s", err, usage)
		return 2
	}
	return 0
}

// SplitSetFlags retire les options --set chemin=valeur (ou --set=…) d'une
// liste d'arguments.
func SplitSetFlags(args []string) (sets, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--set" || a == "-set":
			if i+1 >= len(args) {
				return nil, nil, errors.New("--set : chemin=valeur attendu")
			}
			sets = append(sets, args[i+1])
			i++
		case strings.HasPrefix(a, "--set="):
			sets = append(sets, strings.TrimPrefix(a, "--set="))
		default:
			rest = append(rest, a)
		}
	}
	return sets, rest, nil
}

func runShow(args []string, w io.Writer) error {
	sets, rest, err := SplitSetFlags(args)
	if err != nil {
		return err
	}
	asJSON := false
	for _, a := range rest {
		if a != "--json" {
			return fmt.Errorf("option inconnue %q", a)
		}
		asJSON = true
	}
	c := (&Loader{SetFlags: sets}).Load(context.Background())
	if asJSON {
		eff, err := c.JSON()
		if err != nil {
			return err
		}
		out := map[string]any{
			"effective": json.RawMessage(eff),
			"entries":   c.Explain(),
			"warnings":  c.Warnings,
			"remote":    c.Remote,
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	for _, e := range c.Explain() {
		v, _ := json.Marshal(e.Value)
		lock := ""
		if e.Locked {
			lock = " 🔒"
		}
		fmt.Fprintf(w, "%s = %s%s\n    ← %s (%s)\n", e.Path, v, lock, e.Source, e.Rank)
	}
	if c.Remote.URL != "" {
		if c.Remote.Cached {
			sig := "non signée"
			if c.Remote.Signed {
				sig = "signée"
			}
			fmt.Fprintf(w, "\nConfiguration distante : %s (%s, téléchargée le %s)\n", c.Remote.URL, sig,
				c.Remote.FetchedAt.Local().Format(time.DateTime))
		} else {
			fmt.Fprintf(w, "\nConfiguration distante : %s (pas encore téléchargée : lattis config refresh)\n", c.Remote.URL)
		}
	}
	printWarnings(w, c.Warnings)
	return nil
}

func printWarnings(w io.Writer, ws []Warning) {
	if len(ws) == 0 {
		return
	}
	fmt.Fprintln(w, "\nAvertissements :")
	for _, x := range ws {
		fmt.Fprintf(w, "  - %s\n", x)
	}
}

// runCheck lit un fichier comme le ferait le fichier machine.
func runCheck(path string, stdout, stderr io.Writer) int {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "lattis config : %v\n", err)
		return 1
	}
	t, err := parseTree(b)
	if err != nil {
		fmt.Fprintf(stderr, "%s : %v\n", path, err)
		return 1
	}
	src := &TreeSource{N: path, R: RankMachine, Trust: true, T: t}
	c := Resolve(context.Background(), Options{}, src)
	if _, err := c.Settings(); err != nil {
		fmt.Fprintf(stderr, "%s : %v\n", path, err)
		return 1
	}
	if len(c.Warnings) > 0 {
		printWarnings(stdout, c.Warnings)
		return 1
	}
	fmt.Fprintf(stdout, "%s : valide\n", path)
	return 0
}

func runRefresh(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	l := &Loader{}
	c := l.Load(ctx)
	if c.Remote.URL == "" {
		return errors.New("aucune configuration distante (configUrl) n'est configurée")
	}
	changed, err := l.Refresh(ctx)
	if err != nil {
		return err
	}
	if changed {
		fmt.Fprintf(w, "Configuration distante mise à jour depuis %s\n", c.Remote.URL)
	} else {
		fmt.Fprintf(w, "Configuration distante déjà à jour (%s)\n", c.Remote.URL)
	}
	return nil
}

func runKeygen(privPath string, w io.Writer) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(privPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der}); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Fprintf(w, "Clé privée : %s (à garder secrète)\nremote.publicKey : %s\n", privPath, base64.StdEncoding.EncodeToString(pub))
	return nil
}

func runSign(privPath, docPath string, w io.Writer) error {
	kb, err := os.ReadFile(privPath)
	if err != nil {
		return err
	}
	blk, _ := pem.Decode(kb)
	if blk == nil {
		return errors.New("clé privée PEM illisible")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return errors.New("clé privée Ed25519 attendue")
	}
	doc, err := os.ReadFile(docPath)
	if err != nil {
		return err
	}
	if _, err := parseTree(doc); err != nil {
		return err
	}
	fmt.Fprintln(w, SignJWS(doc, priv))
	return nil
}
