package api

import (
	"encoding/base64"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// Images, formes et zones de texte d'une feuille (canvas/objects.go).

var pObject = Param{Name: "object", Type: Ref, Desc: "objet (identifiant ou nom)"}

const objectHelp = `config : {kind, width, height, offX, offY, name, fill, line, lineWidth, transparency, text, bold, italic, size, color, align, valign, flipH, flipV, data, image}. kind : "rect", "roundRect" (rectangle arrondi), "ellipse", "triangle", "line", "arrow" (flèche), "text" (zone de texte) ou "image". width, height : taille en pixels (par défaut selon le genre ; une image garde ses proportions, 480 pixels de large au plus) ; offX, offY : décalage dans la cellule d'ancrage. fill et line : couleurs du fond et du contour ("#rrggbb", "" : aucun) ; lineWidth : épaisseur du contour (pixels) ; transparency : transparence du fond (0 à 100). text : texte de la forme ou de la zone de texte, mis en forme par bold, italic, size (points), color, align ("left", "center", "right") et valign ("top", "middle", "bottom"). flipH, flipV : sens d'une ligne ou d'une flèche (par défaut du coin haut gauche au coin bas droit, la pointe au bout). Une image se donne par data (contenu PNG, JPEG, GIF ou WebP en base64, 10 Mo au plus) ou par image (empreinte d'une image déjà dans le classeur, voir objects.list).`

func init() {
	register(
		&Command{
			Name:    "objects.create",
			Label:   "Insérer un objet",
			Summary: "Pose une forme, une zone de texte ou une image sur une feuille, coin haut gauche en row, col, et renvoie son identifiant.",
			Details: objectHelp,
			Params:  []Param{pRow, pCol, {Name: "config", Type: Object, Desc: "genre et présentation de l'objet"}, pSheet},
			Write:   true,
			Example: `objects.create(2, 2, {kind = "roundRect", width = 180, height = 60, text = "Objectif atteint", bold = true})
objects.create(6, 2, {kind = "arrow", width = 120, height = 0, line = "#c00000", lineWidth = 3})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				var in struct {
					canvas.ObjectConfig
					Data string `json:"data"`
				}
				if err := c.Decode("config", &in); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				cfg := in.ObjectConfig
				if in.Data != "" {
					raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(in.Data))
					if err != nil {
						return nil, msg.Wrap(canvas.ErrObject, "object.imageUnreadable", nil)
					}
					return d.CreateImage(c, s.ID, row, col, raw, cfg)
				}
				return d.CreateObject(c, s.ID, row, col, cfg)
			},
		},
		&Command{
			Name:    "objects.delete",
			Label:   "Supprimer l'objet",
			Summary: "Supprime une image, une forme ou une zone de texte.",
			Params:  []Param{pObject},
			Write:   true,
			Run: func(c *Call) (any, error) {
				id, err := c.object("_lattis_objects", "name", "objet", "object")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteObject(c, id)
			},
		},
		&Command{
			Name:    "objects.list",
			Summary: "Liste les images, formes et zones de texte d'une feuille, de l'arrière-plan au premier plan.",
			Returns: "{ {id, kind, name, row, col, offX, offY, width, height, text, image}, ... }",
			Params:  []Param{pSheet},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				objs, err := d.Objects(c, s.ID)
				if err != nil {
					return nil, err
				}
				type view struct {
					ID     int64  `json:"id"`
					Kind   string `json:"kind"`
					Name   string `json:"name,omitempty"`
					Row    int    `json:"row"`
					Col    int    `json:"col"`
					OffX   int    `json:"offX"`
					OffY   int    `json:"offY"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
					Text   string `json:"text,omitempty"`
					Image  string `json:"image,omitempty"`
				}
				out := make([]view, 0, len(objs))
				for _, o := range objs {
					k := o.Config
					// Lignes et colonnes comptées depuis 1, comme les autres commandes.
					out = append(out, view{o.ID, k.Kind, k.Name, o.Row + 1, o.Col + 1, k.OffX, k.OffY, k.Width, k.Height, k.Text, k.Image})
				}
				return out, nil
			},
		},
	)
}
