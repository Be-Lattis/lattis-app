package canvas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif" // DecodeConfig des GIF
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Images, formes et zones de texte posées sur une feuille, comme les
// graphiques : ancrées à une cellule (coin haut gauche) avec un décalage et
// une taille en pixels, elles suivent les lignes et colonnes insérées ou
// supprimées avant elles (sheetrows.go, « déplacer avec les cellules »).
// Les images elles-mêmes sont dans _lattis_images, une ligne par contenu
// (empreinte SHA-256) : deux objets d'une même image la partagent, et une
// image copiée-collée n'est pas recopiée.
//
// L'image est écrite hors du journal d'annulation (INSERT OR IGNORE, sans
// inverse) : annuler l'insertion retire l'objet, l'image reste dans la base
// de travail pour un rétablissement. L'enregistrement (archive/scrub.go)
// ne garde que les images qu'un objet cite.

// Genres d'objets.
const (
	ObjImage     = "image"
	ObjRect      = "rect"
	ObjRoundRect = "roundRect"
	ObjEllipse   = "ellipse"
	ObjLine      = "line"
	ObjArrow     = "arrow"
	ObjTriangle  = "triangle"
	ObjText      = "text" // zone de texte
)

var objectKinds = map[string]bool{ObjImage: true, ObjRect: true, ObjRoundRect: true, ObjEllipse: true, ObjLine: true, ObjArrow: true, ObjTriangle: true, ObjText: true}

// Bornes des images.
const (
	// MaxImageBytes : taille d'une image enregistrée dans le classeur.
	MaxImageBytes = 10 << 20
	// MaxSharedImageBytes : taille d'une image dans un classeur partagé :
	// la mise en page publiée (remote/workbook/layout.go) est un seul objet
	// renvoyé à chaque changement ; les images y sont gardées petites.
	MaxSharedImageBytes = 1 << 20
	// MaxSharedImagesBytes : toutes les images d'un classeur partagé.
	MaxSharedImagesBytes = 4 << 20
	// MaxImageSide : côté d'une image (px) ; l'interface réduit les plus
	// grandes avant de les envoyer.
	MaxImageSide = 8000
	// DefaultImageWidth : largeur posée au plus (px) d'une image insérée.
	DefaultImageWidth = 480
)

// ObjectConfig est la configuration d'un objet (JSON de _lattis_objects).
type ObjectConfig struct {
	// Kind : image, rect, roundRect, ellipse, line, arrow, triangle, text.
	Kind string `json:"kind"`
	// Name : nom donné par un script (facultatif).
	Name string `json:"name,omitempty"`
	// Image : empreinte de l'image (_lattis_images) ; Alt : texte de
	// remplacement (nom du fichier d'origine).
	Image string `json:"image,omitempty"`
	Alt   string `json:"alt,omitempty"`
	// Décalage dans la cellule d'ancrage et taille (px).
	OffX   int `json:"offX,omitempty"`
	OffY   int `json:"offY,omitempty"`
	Width  int `json:"width"`
	Height int `json:"height"`
	// Z : rang de premier plan (le plus grand devant).
	Z int `json:"z,omitempty"`
	// Fill et Line : couleurs « #rrggbb » du fond et du contour ; vide :
	// aucun. LineWidth : épaisseur du contour (px).
	Fill      string  `json:"fill,omitempty"`
	Line      string  `json:"line,omitempty"`
	LineWidth float64 `json:"lineWidth,omitempty"`
	// Transparency : transparence du fond, de 0 (opaque) à 100 %.
	Transparency int `json:"transparency,omitempty"`
	// FlipH, FlipV : sens d'une ligne ou d'une flèche (par défaut du coin
	// haut gauche vers le coin bas droit, la pointe au bout).
	FlipH bool `json:"flipH,omitempty"`
	FlipV bool `json:"flipV,omitempty"`
	// Texte d'une forme ou d'une zone de texte, et sa mise en forme.
	Text   string  `json:"text,omitempty"`
	Bold   bool    `json:"bold,omitempty"`
	Italic bool    `json:"italic,omitempty"`
	Size   float64 `json:"size,omitempty"`   // points ; 0 : 11
	Color  string  `json:"color,omitempty"`  // « #rrggbb » ; vide : noir
	Align  string  `json:"align,omitempty"`  // left, center, right
	VAlign string  `json:"valign,omitempty"` // top, middle, bottom
}

// ObjectInfo décrit un objet posé sur une feuille.
type ObjectInfo struct {
	ID     int64        `json:"id"`
	Sheet  int64        `json:"sheet,omitempty"`
	Row    int          `json:"row"`
	Col    int          `json:"col"`
	Config ObjectConfig `json:"config"`
}

// ImageInfo décrit une image enregistrée.
type ImageInfo struct {
	Hash   string `json:"hash"`
	Type   string `json:"type"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int    `json:"bytes"`
}

// ErrObject signale un objet impossible.
var ErrObject = errors.New("objet")

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// IsLine dit si le genre est un trait (ligne, flèche) : sans fond ni texte,
// sa boîte peut être plate.
func IsLine(kind string) bool { return kind == ObjLine || kind == ObjArrow }

// Defaults complète la présentation d'un objet neuf (couleurs non données).
func (c *ObjectConfig) Defaults() {
	switch {
	case c.Kind == ObjImage:
	case c.Kind == ObjText:
		if c.Fill == "" && c.Line == "" {
			c.Fill, c.Line = "#ffffff", "#7f8790"
		}
	case IsLine(c.Kind):
		if c.Line == "" {
			c.Line = "#2f5597"
		}
		if c.LineWidth == 0 {
			c.LineWidth = 2
		}
	default:
		if c.Fill == "" && c.Line == "" {
			c.Fill, c.Line = "#4472c4", "#2f528f"
		}
		if c.Color == "" {
			c.Color = "#ffffff"
		}
		if c.Align == "" {
			c.Align = "center"
		}
		if c.VAlign == "" {
			c.VAlign = "middle"
		}
	}
	if c.Kind != ObjImage && !IsLine(c.Kind) && c.LineWidth == 0 && c.Line != "" {
		c.LineWidth = 1
	}
	if c.Width == 0 && c.Height == 0 {
		switch {
		case c.Kind == ObjText:
			c.Width, c.Height = 160, 60
		case IsLine(c.Kind):
			c.Width, c.Height = 160, 0
		default:
			c.Width, c.Height = 120, 80
		}
	}
}

func (c *ObjectConfig) check() error {
	c.Name = strings.TrimSpace(c.Name)
	if !objectKinds[c.Kind] {
		return msg.Wrap(ErrObject, "object.unknownKind", msg.Args{"kind": c.Kind})
	}
	if len([]rune(c.Name)) > 64 {
		return msg.Wrap(ErrObject, "object.nameTooLong", nil)
	}
	if c.Kind == ObjImage {
		if c.Image == "" {
			return msg.Wrap(ErrObject, "object.noImage", nil)
		}
		// Une image n'a ni texte ni couleurs.
		c.Text, c.Fill, c.Line, c.LineWidth = "", "", "", 0
	} else {
		c.Image = ""
	}
	if IsLine(c.Kind) {
		c.Text, c.Fill = "", ""
	} else {
		c.FlipH, c.FlipV = false, false
	}
	if len(c.Alt) > 300 {
		c.Alt = c.Alt[:300]
	}
	minW, minH := 4, 4
	if IsLine(c.Kind) {
		minW, minH = 0, 0
	}
	if c.Width < minW || c.Height < minH || c.Width > 20000 || c.Height > 20000 || (IsLine(c.Kind) && c.Width+c.Height < 4) {
		return msg.Wrap(ErrObject, "object.invalidSize", nil)
	}
	for _, col := range []string{c.Fill, c.Line, c.Color} {
		if col != "" && !hexColor.MatchString(col) {
			return msg.Wrap(ErrObject, "object.invalidColor", msg.Args{"color": col})
		}
	}
	if c.LineWidth < 0 || c.LineWidth > 50 {
		return msg.Wrap(ErrObject, "object.invalidLineWidth", nil)
	}
	if c.Transparency < 0 || c.Transparency > 100 {
		return msg.Wrap(ErrObject, "object.invalidTransparency", nil)
	}
	if c.Size != 0 && (c.Size < 4 || c.Size > 200) {
		return msg.Wrap(ErrObject, "object.invalidFontSize", nil)
	}
	if len([]rune(c.Text)) > 10000 {
		return msg.Wrap(ErrObject, "object.textTooLong", nil)
	}
	switch c.Align {
	case "", "left", "center", "right":
	default:
		return msg.Wrap(ErrObject, "object.invalidAlign", msg.Args{"align": c.Align})
	}
	switch c.VAlign {
	case "", "top", "middle", "bottom":
	default:
		return msg.Wrap(ErrObject, "object.invalidAlign", msg.Args{"align": c.VAlign})
	}
	return nil
}

// objectLabel : nom de l'action d'insertion d'un objet de ce genre.
func objectLabel(kind string) string {
	switch kind {
	case ObjImage:
		return "Insérer une image"
	case ObjText:
		return "Insérer une zone de texte"
	}
	return "Insérer une forme"
}

// CreateObject pose une forme, une zone de texte ou une image déjà
// enregistrée (cfg.Image) sur une feuille, coin haut gauche en (row, col).
func (m *Model) CreateObject(ctx context.Context, sheet int64, row, col int, cfg ObjectConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, msg.Wrap(ErrObject, "object.outOfSheet", nil)
	}
	cfg.Defaults()
	if err := cfg.check(); err != nil {
		return 0, err
	}
	var id int64
	err := m.Do(ctx, objectLabel(cfg.Kind), func(t *tx) error {
		if cfg.Kind == ObjImage {
			var n int
			if err := t.row(`SELECT count(*) FROM _lattis_images WHERE hash = ?`, cfg.Image).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return msg.Wrap(ErrObject, "object.imageNotFound", nil)
			}
		}
		var err error
		id, err = t.insertObject(sheet, row, col, cfg)
		return err
	})
	return id, err
}

// CreateImage enregistre une image (PNG, JPEG, GIF ou WebP) et la pose sur
// une feuille, en une étape d'annulation. cfg peut donner la taille (sinon
// celle de l'image, réduite à DefaultImageWidth de large) et Alt.
func (m *Model) CreateImage(ctx context.Context, sheet int64, row, col int, data []byte, cfg ObjectConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, msg.Wrap(ErrObject, "object.outOfSheet", nil)
	}
	info, err := checkImage(data)
	if err != nil {
		return 0, err
	}
	cfg.Kind, cfg.Image = ObjImage, info.Hash
	if cfg.Width <= 0 || cfg.Height <= 0 {
		cfg.Width, cfg.Height = FitImage(info.Width, info.Height, DefaultImageWidth)
	}
	if err := cfg.check(); err != nil {
		return 0, err
	}
	var id int64
	err = m.Do(ctx, objectLabel(ObjImage), func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		// Classeur partagé (_lattis_info « remote », remote/workbook/info.go) :
		// images bornées, voir MaxSharedImageBytes.
		var shared int
		if err := t.row(`SELECT count(*) FROM _lattis_info WHERE key = 'remote'`).Scan(&shared); err != nil {
			return err
		}
		if shared > 0 && len(data) > MaxSharedImageBytes {
			return msg.Wrap(ErrObject, "object.imageTooLargeShared", msg.Args{"mb": MaxSharedImageBytes >> 20})
		}
		if shared > 0 {
			var total int
			if err := t.row(`SELECT coalesce(sum(length(data)), 0) FROM _lattis_images WHERE hash <> ? AND hash IN (SELECT json_extract(config, '$.image') FROM _lattis_objects)`, info.Hash).Scan(&total); err != nil {
				return err
			}
			if total+len(data) > MaxSharedImagesBytes {
				return msg.Wrap(ErrObject, "object.imagesTooLargeShared", msg.Args{"mb": MaxSharedImagesBytes >> 20})
			}
		}
		// Hors du journal : voir le commentaire en tête du fichier.
		if _, err := t.Exec(`INSERT OR IGNORE INTO _lattis_images (hash, type, width, height, data) VALUES (?, ?, ?, ?, ?)`, info.Hash, info.Type, info.Width, info.Height, data); err != nil {
			return err
		}
		var err error
		id, err = t.insertObject(sheet, row, col, cfg)
		return err
	})
	return id, err
}

// FitImage donne la taille posée d'une image w × h : la sienne, réduite à
// maxW de large (proportions gardées).
func FitImage(w, h, maxW int) (int, int) {
	if w <= 0 || h <= 0 {
		return 160, 120
	}
	if w > maxW {
		h = max(1, h*maxW/w)
		w = maxW
	}
	return max(w, 4), max(h, 4)
}

func (t *tx) insertObject(sheet int64, row, col int, cfg ObjectConfig) (int64, error) {
	if err := t.guard(sheet); err != nil {
		return 0, err
	}
	var n int
	if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, msg.Wrap(ErrSheet, "sheet.notFound", nil)
	}
	var id int64
	var z int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_objects`).Scan(&id); err != nil {
		return 0, err
	}
	// Un objet neuf vient devant les autres.
	if err := t.row(`SELECT coalesce(max(json_extract(config, '$.z')), 0) + 1 FROM _lattis_objects WHERE sheet = ?`, sheet).Scan(&z); err != nil {
		return 0, err
	}
	cfg.Z = z
	b, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}
	return id, t.apply(
		[]stmt{st(`INSERT INTO _lattis_objects (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, string(b))},
		[]stmt{st(`DELETE FROM _lattis_objects WHERE id = ?`, id)},
	)
}

// SetObject change la présentation d'un objet (couleurs, texte…) ; sa
// place, sa taille, son rang et son image sont gardés.
func (m *Model) SetObject(ctx context.Context, id int64, cfg ObjectConfig) error {
	return m.Do(ctx, "Modifier l'objet", func(t *tx) error {
		if err := t.guardObject(`_lattis_objects`, id); err != nil {
			return err
		}
		old, err := t.object(id)
		if err != nil {
			return err
		}
		o := old.Config
		cfg.Kind, cfg.Image = o.Kind, o.Image
		cfg.OffX, cfg.OffY, cfg.Width, cfg.Height, cfg.Z = o.OffX, o.OffY, o.Width, o.Height, o.Z
		if cfg.Alt == "" {
			cfg.Alt = o.Alt
		}
		if err := cfg.check(); err != nil {
			return err
		}
		return t.writeObject(old, old.Row, old.Col, cfg)
	})
}

// PlaceObject déplace et redimensionne un objet au pixel près, en une
// étape d'annulation.
func (m *Model) PlaceObject(ctx context.Context, id int64, row, col, offX, offY, width, height int) error {
	return m.place(ctx, id, row, col, offX, offY, width, height, nil)
}

// PlaceLine place une ligne ou une flèche par ses extrémités : sa boîte et
// son sens (une extrémité tirée de l'autre côté la retourne), en une étape.
func (m *Model) PlaceLine(ctx context.Context, id int64, row, col, offX, offY, width, height int, flipH, flipV bool) error {
	return m.place(ctx, id, row, col, offX, offY, width, height, &[2]bool{flipH, flipV})
}

func (m *Model) place(ctx context.Context, id int64, row, col, offX, offY, width, height int, flips *[2]bool) error {
	if row < 0 || col < 0 {
		return msg.Wrap(ErrObject, "object.outOfSheet", nil)
	}
	return m.Do(ctx, "Déplacer l'objet", func(t *tx) error {
		if err := t.guardObject(`_lattis_objects`, id); err != nil {
			return err
		}
		old, err := t.object(id)
		if err != nil {
			return err
		}
		cfg := old.Config
		cfg.OffX, cfg.OffY, cfg.Width, cfg.Height = offX, offY, width, height
		if flips != nil {
			cfg.FlipH, cfg.FlipV = flips[0], flips[1]
		}
		if err := cfg.check(); err != nil {
			return err
		}
		return t.writeObject(old, row, col, cfg)
	})
}

// OrderObject met un objet au premier plan (front) ou à l'arrière-plan,
// parmi les objets de sa feuille.
func (m *Model) OrderObject(ctx context.Context, id int64, front bool) error {
	label := "Mettre à l'arrière-plan"
	if front {
		label = "Mettre au premier plan"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guardObject(`_lattis_objects`, id); err != nil {
			return err
		}
		old, err := t.object(id)
		if err != nil {
			return err
		}
		var lo, hi, n int
		if err := t.row(`SELECT coalesce(min(json_extract(config, '$.z')), 0), coalesce(max(json_extract(config, '$.z')), 0), count(*) FROM _lattis_objects WHERE sheet = ? AND id <> ?`, old.Sheet, id).Scan(&lo, &hi, &n); err != nil {
			return err
		}
		cfg := old.Config
		switch {
		case n == 0:
			return nil
		case front && cfg.Z > hi:
			return nil
		case !front && cfg.Z < lo:
			return nil
		case front:
			cfg.Z = hi + 1
		default:
			cfg.Z = lo - 1
		}
		return t.writeObject(old, old.Row, old.Col, cfg)
	})
}

// DeleteObject supprime un objet (l'image reste pour l'annulation).
func (m *Model) DeleteObject(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer l'objet", func(t *tx) error {
		if err := t.guardObject(`_lattis_objects`, id); err != nil {
			return err
		}
		var sheet int64
		var row, col int
		var cfg string
		if err := t.row(`SELECT sheet, row, col, config FROM _lattis_objects WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
			return msg.Wrap(ErrObject, "object.notFound", msg.Args{"id": id})
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_objects WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_objects (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
		)
	})
}

// Object lit un objet.
func (m *Model) Object(ctx context.Context, id int64) (ObjectInfo, error) {
	var o ObjectInfo
	err := m.read(ctx, func(t *tx) error {
		var err error
		o, err = t.object(id)
		return err
	})
	return o, err
}

// Objects lit les objets d'une feuille, de l'arrière-plan au premier plan.
func (m *Model) Objects(ctx context.Context, sheet int64) ([]ObjectInfo, error) {
	var out []ObjectInfo
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.objects(sheet)
		return err
	})
	return out, err
}

func (t *tx) object(id int64) (ObjectInfo, error) {
	o := ObjectInfo{ID: id}
	var cfg string
	if err := t.row(`SELECT sheet, row, col, config FROM _lattis_objects WHERE id = ?`, id).Scan(&o.Sheet, &o.Row, &o.Col, &cfg); err != nil {
		return o, msg.Wrap(ErrObject, "object.notFound", msg.Args{"id": id})
	}
	return o, json.Unmarshal([]byte(cfg), &o.Config)
}

// objects lit les objets d'une feuille, du fond vers le premier plan.
func (t *tx) objects(sheet int64) ([]ObjectInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_objects WHERE sheet = ? ORDER BY coalesce(json_extract(config, '$.z'), 0), id`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []ObjectInfo{}
	for rs.Next() {
		o := ObjectInfo{Sheet: sheet}
		var cfg string
		if err := rs.Scan(&o.ID, &o.Row, &o.Col, &cfg); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(cfg), &o.Config) != nil {
			continue // configuration illisible : l'objet n'est pas dessiné
		}
		out = append(out, o)
	}
	return out, rs.Err()
}

func (t *tx) writeObject(old ObjectInfo, row, col int, cfg ObjectConfig) error {
	nb, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	ob, err := json.Marshal(old.Config)
	if err != nil {
		return err
	}
	if old.Row == row && old.Col == col && string(ob) == string(nb) {
		return nil
	}
	q := `UPDATE _lattis_objects SET row = ?, col = ?, config = ? WHERE id = ?`
	return t.apply([]stmt{st(q, row, col, string(nb), old.ID)}, []stmt{st(q, old.Row, old.Col, string(ob), old.ID)})
}

// objectRows donne l'inverse de la suppression des objets d'une feuille.
func (t *tx) objectRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_objects WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg string
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _lattis_objects (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg))
	}
	return undo, rs.Err()
}

// --- Images ---------------------------------------------------------------------

// ImageData lit une image enregistrée : son contenu et son type MIME.
func (m *Model) ImageData(ctx context.Context, hash string) ([]byte, string, error) {
	var data []byte
	var typ string
	err := m.read(ctx, func(t *tx) error {
		err := t.row(`SELECT data, type FROM _lattis_images WHERE hash = ?`, hash).Scan(&data, &typ)
		if errors.Is(err, sql.ErrNoRows) {
			return msg.Wrap(ErrObject, "object.imageNotFound", nil)
		}
		return err
	})
	return data, typ, err
}

// ImageHash donne l'empreinte d'un contenu (clé de _lattis_images).
func ImageHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// imageTypes : types d'images acceptés (le SVG est converti en PNG par
// l'interface : un SVG peut porter des scripts, il n'est jamais gardé tel
// quel).
var imageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// CheckImage vérifie une image (type reconnu à son contenu, taille, côtés)
// et donne sa description.
func CheckImage(data []byte) (ImageInfo, error) { return checkImage(data) }

func checkImage(data []byte) (ImageInfo, error) {
	if len(data) == 0 {
		return ImageInfo{}, msg.Wrap(ErrObject, "object.imageUnreadable", nil)
	}
	if len(data) > MaxImageBytes {
		return ImageInfo{}, msg.Wrap(ErrObject, "object.imageTooLarge", msg.Args{"mb": MaxImageBytes >> 20})
	}
	typ := http.DetectContentType(data)
	if !imageTypes[typ] {
		return ImageInfo{}, msg.Wrap(ErrObject, "object.imageType", nil)
	}
	info := ImageInfo{Hash: ImageHash(data), Type: typ, Bytes: len(data)}
	if typ == "image/webp" {
		w, h, ok := webpSize(data)
		if !ok {
			return ImageInfo{}, msg.Wrap(ErrObject, "object.imageUnreadable", nil)
		}
		info.Width, info.Height = w, h
	} else {
		c, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return ImageInfo{}, msg.Wrap(ErrObject, "object.imageUnreadable", nil)
		}
		info.Width, info.Height = c.Width, c.Height
	}
	if info.Width <= 0 || info.Height <= 0 || info.Width > MaxImageSide || info.Height > MaxImageSide {
		return ImageInfo{}, msg.Wrap(ErrObject, "object.imageSide", msg.Args{"max": MaxImageSide})
	}
	return info, nil
}

// webpSize lit la taille d'une image WebP (en-têtes VP8, VP8L, VP8X).
func webpSize(b []byte) (int, int, bool) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(b[12:16]) {
	case "VP8 ":
		// Trame clé : code de départ 9d 01 2a, puis largeur et hauteur sur 14 bits.
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, false
		}
		w := int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff)
		return w, h, true
	case "VP8L":
		if b[20] != 0x2f {
			return 0, 0, false
		}
		v := binary.LittleEndian.Uint32(b[21:25])
		return int(v&0x3fff) + 1, int(v>>14&0x3fff) + 1, true
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, true
	}
	return 0, 0, false
}
