package workbook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/lalamefine/lattis/internal/msg"
)

// Sources poussées du Data Lake (contrat client de lattisd, §9.1 ;
// migration 0027) : un tableau d'un classeur local publié seul, sans le
// classeur (clic droit sur un tableau › Partager au Data Lake Lattis… ›
// « Publier uniquement ce tableau »). lattisd crée un classeur technique
// (kind = 'source', dont l'utilisateur est propriétaire) qui ne contient
// que ce tableau, un pipeline « push » qui l'alimente et la donnée du
// catalogue. Le poste envoie ensuite le contenu du tableau en CSV par
// HTTP (Pusher), et adapte les colonnes du tableau technique par les
// fonctions de structure (Writer.AddColumn…). Voir
// docs/collaboration/pipeline-table-source.md.

// Modes d'écriture d'une source poussée (lattis.pipelines.mode).
const (
	PushReplace = "replace" // le tableau du serveur devient celui envoyé
	PushAppend  = "append"  // les lignes envoyées s'ajoutent
	PushUpsert  = "upsert"  // mise à jour par clé (KeyColumns)
)

// PushedSourceSpec décrit une source à créer (lattis.create_pushed_source).
type PushedSourceSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// FolderID : dossier du catalogue ("" : la racine).
	FolderID string `json:"folder_id,omitempty"`
	// SQLName vide : proposé par le serveur à partir du nom.
	SQLName string `json:"sql_name,omitempty"`
	// TableName : nom du tableau sur le serveur (vide : Name).
	TableName string       `json:"table_name,omitempty"`
	Columns   []ColumnSpec `json:"columns"`
	// Mode : PushReplace (vide), PushAppend ou PushUpsert (avec KeyColumns).
	Mode       string       `json:"mode,omitempty"`
	KeyColumns []string     `json:"key_columns"`
	Origin     PushedOrigin `json:"origin"`
}

// PushedOrigin dit d'où vient une source (affichage seulement).
type PushedOrigin struct {
	Workbook        string `json:"workbook"`
	WorkbookLocalID string `json:"workbook_local_id,omitempty"`
	Table           string `json:"table"`
	TableLocalID    string `json:"table_local_id,omitempty"`
}

// PushedSourceCreated est le résultat de lattis.create_pushed_source.
type PushedSourceCreated struct {
	PipelineID string `json:"pipeline_id"`
	DatasetID  string `json:"dataset_id"`
	WorkbookID string `json:"workbook_id"`
	TableID    string `json:"table_id"`
	SQLName    string `json:"sql_name"`
}

// PushReject est une valeur refusée par le pipeline.
type PushReject struct {
	Row    int    `json:"row"`
	Column string `json:"column"`
	Value  string `json:"value"`
	Error  string `json:"error"`
}

// PushRun est une exécution du pipeline (lattis.pipeline_runs) : le bilan
// d'un envoi. Status : ok, warning (valeurs refusées), error, running
// (réponse 202 ou exécution commencée : à suivre par Pusher.PushRun avec
// RunID), ou pending (envoi pas encore pris par lattisd).
type PushRun struct {
	ID string `json:"id,omitempty"`
	// RunID : exécution à suivre (réponse 202 d'un serveur avec la
	// migration 0028 ; vide avant : suivi par PushStatus).
	RunID      string       `json:"run_id,omitempty"`
	Status     string       `json:"status"`
	Trigger    string       `json:"trigger,omitempty"`
	RowsRead   int          `json:"rows_read"`
	Inserted   int          `json:"inserted"`
	Updated    int          `json:"updated"`
	Deleted    int          `json:"deleted"`
	Rejected   int          `json:"rejected"`
	Rejects    []PushReject `json:"rejects,omitempty"`
	Error      string       `json:"error,omitempty"`
	StartedAt  string       `json:"started_at,omitempty"`
	FinishedAt string       `json:"finished_at,omitempty"`
}

// PushedSource est une source poussée de l'utilisateur
// (lattis.my_pushed_sources, GET /api/push/<id>).
type PushedSource struct {
	PipelineID string       `json:"pipeline_id"`
	Name       string       `json:"name"`
	Mode       string       `json:"mode"`
	KeyColumns []string     `json:"key_columns"`
	Origin     PushedOrigin `json:"origin"`
	WorkbookID string       `json:"workbook_id"`
	TableID    string       `json:"table_id"`
	Enabled    bool         `json:"enabled"`
	// Pending : un envoi attend d'être écrit.
	Pending  bool `json:"pending"`
	Datasets []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		SQLName string `json:"sql_name"`
	} `json:"datasets"`
	LastRun *PushRun `json:"last_run"`
}

// PushedSources est l'API SQL des sources poussées.
type PushedSources interface {
	// PushedSourcesSupported dit si le serveur a la migration 0027.
	PushedSourcesSupported(ctx context.Context) (bool, error)
	// CreatePushedSource crée le classeur technique, le pipeline et la
	// donnée (publieurs de données), sans ligne : le premier envoi suit.
	CreatePushedSource(ctx context.Context, spec PushedSourceSpec) (PushedSourceCreated, error)
	// PushedSources liste les sources poussées de l'utilisateur.
	PushedSources(ctx context.Context) ([]PushedSource, error)
	// DropPushedSource retire la source : donnée (*withdrawn:<id>),
	// pipeline et classeur technique.
	DropPushedSource(ctx context.Context, pipelineID string) error
	// UpdatePushedSource change le mode, la clé ou suspend la source
	// (migration 0028 ; avant : 42883, voir Unsupported).
	UpdatePushedSource(ctx context.Context, pipelineID string, c PushedSourceChange) error
}

// PushedSourceChange modifie une source poussée (lattis.update_pushed_source) ;
// nil : inchangé.
type PushedSourceChange struct {
	Mode       *string   `json:"mode,omitempty"`
	KeyColumns *[]string `json:"key_columns,omitempty"`
	Enabled    *bool     `json:"enabled,omitempty"`
}

func (s pgServer) UpdatePushedSource(ctx context.Context, pipelineID string, c PushedSourceChange) error {
	return s.exec(ctx, `SELECT lattis.update_pushed_source($1::uuid, $2::jsonb)`, pipelineID, c)
}

func (s pgServer) PushedSourcesSupported(ctx context.Context) (bool, error) {
	var ok bool
	err := s.with(ctx, usageWrite, func(c *pgx.Conn) error {
		return c.QueryRow(ctx, `SELECT to_regprocedure('lattis.create_pushed_source(jsonb)') IS NOT NULL`).Scan(&ok)
	})
	return ok, err
}

func (s pgServer) CreatePushedSource(ctx context.Context, spec PushedSourceSpec) (PushedSourceCreated, error) {
	var out PushedSourceCreated
	if spec.KeyColumns == nil {
		spec.KeyColumns = []string{}
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT lattis.create_pushed_source($1::jsonb)::text`, spec).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	return out, err
}

func (s pgServer) PushedSources(ctx context.Context) ([]PushedSource, error) {
	var out []PushedSource
	err := s.jsonCall(ctx, `SELECT lattis.my_pushed_sources()::text`, &out)
	return out, err
}

func (s pgServer) DropPushedSource(ctx context.Context, pipelineID string) error {
	return s.exec(ctx, `SELECT lattis.drop_pushed_source($1::uuid)`, pipelineID)
}

// Pusher envoie le contenu d'un tableau à une source poussée (HTTP,
// contrat §9.1).
type Pusher interface {
	// Push envoie le CSV (UTF-8, première ligne = noms des colonnes) et rend
	// le bilan de l'exécution ; Status « running » : l'écriture dure encore
	// (202), à suivre par PushStatus. Refus : *PushError.
	Push(ctx context.Context, pipelineID string, csv []byte) (PushRun, error)
	// PushStatus lit l'état de la source (dernier envoi, envoi en attente).
	PushStatus(ctx context.Context, pipelineID string) (PushedSource, error)
	// PushRun lit une exécution (GET ?run=, migration 0028) : bilan, ou
	// Status pending / running ; inconnue : *PushError 404. Un serveur
	// d'avant 0028 ignore run et rend l'état de la source : Status vide.
	PushRun(ctx context.Context, pipelineID, runID string) (PushRun, error)
}

// PushError est un refus de POST/GET /api/push (401, 403, 404, 409, 413…).
type PushError struct {
	Status  int
	Code    string
	Message string
}

func (e *PushError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager) ; le texte du serveur tel
// quel s'il n'y a rien à ajouter.
func (e *PushError) Msg() msg.Message {
	args := msg.Args{"detail": e.Message, "status": e.Status}
	switch e.Status {
	case http.StatusConflict:
		return msg.M("lake.pushBusy", args)
	case http.StatusRequestEntityTooLarge:
		return msg.M("lake.pushTooLarge", args)
	case http.StatusUnauthorized:
		return msg.M("lake.pushSessionExpired", args)
	}
	if e.Message != "" {
		return msg.M("lake.pushServerText", args)
	}
	return msg.M("lake.pushStatus", args)
}

// PushClient est le Pusher HTTP : émetteur (issuer de la découverte) et
// jeton d'accès de la session PostgreSQL.
type PushClient struct {
	// Issuer : adresse de l'émetteur (lattisd).
	Issuer string
	// Token rend le jeton d'accès ; Invalidate l'oublie (refusé : 401).
	Token      func(ctx context.Context) (string, error)
	Invalidate func()
	// HTTP : client (nil : délai de 20 minutes, comme lattisd).
	HTTP *http.Client
	// Wait : secondes d'attente du bilan avant 202 (0 : 60).
	Wait int
}

func (c PushClient) url(id string) string {
	return strings.TrimRight(c.Issuer, "/") + "/api/push/" + id
}

// do envoie la requête ; un jeton refusé (401) est rafraîchi une fois.
func (c PushClient) do(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Minute}
	}
	for try := 0; ; try++ {
		tok, err := c.Token(ctx)
		if err != nil {
			return nil, err
		}
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rd)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		if body != nil {
			req.Header.Set("Content-Type", "text/csv; charset=utf-8")
		}
		res, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode == http.StatusUnauthorized && try == 0 && c.Invalidate != nil {
			res.Body.Close()
			c.Invalidate()
			continue
		}
		return res, nil
	}
}

// pushError lit le corps d'une réponse en erreur ({"error", "error_description"}).
func pushError(res *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var e struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
		Message     string `json:"message"`
	}
	_ = json.Unmarshal(b, &e)
	msg := e.Description
	if msg == "" {
		msg = e.Message
	}
	if msg == "" {
		msg = strings.TrimSpace(string(b))
	}
	return &PushError{Status: res.StatusCode, Code: e.Error, Message: msg}
}

func (c PushClient) Push(ctx context.Context, id string, csv []byte) (PushRun, error) {
	var out PushRun
	wait := c.Wait
	if wait <= 0 {
		wait = 60
	}
	res, err := c.do(ctx, http.MethodPost, c.url(id)+"?wait="+strconv.Itoa(wait), csv)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
		return out, pushError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, msg.Wrap(err, "lake.reportUnreadable", msg.Args{"err": err})
	}
	if res.StatusCode == http.StatusAccepted {
		out.Status = "running"
	}
	return out, nil
}

func (c PushClient) PushRun(ctx context.Context, id, run string) (PushRun, error) {
	var out PushRun
	res, err := c.do(ctx, http.MethodGet, c.url(id)+"?run="+run, nil)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, pushError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, msg.Wrap(err, "lake.reportUnreadable", msg.Args{"err": err})
	}
	return out, nil
}

func (c PushClient) PushStatus(ctx context.Context, id string) (PushedSource, error) {
	var out PushedSource
	res, err := c.do(ctx, http.MethodGet, c.url(id), nil)
	if err != nil {
		return out, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return out, pushError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return out, msg.Wrap(err, "lake.stateUnreadable", msg.Args{"err": err})
	}
	return out, nil
}

// PushNotFound dit si err est une source inconnue du serveur (404 ou P0002).
func PushNotFound(err error) bool {
	var pe *PushError
	if errors.As(err, &pe) {
		return pe.Status == http.StatusNotFound
	}
	code, ok := Rejected(err)
	return ok && code == "P0002"
}

// Unsupported dit si err vient d'un serveur qui n'a pas la fonction
// appelée (42883 : undefined_function).
func Unsupported(err error) bool {
	code, ok := Rejected(err)
	return ok && code == "42883"
}
