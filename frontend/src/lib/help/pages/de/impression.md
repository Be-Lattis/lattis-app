# Seitenlayout und Drucken

Datei › Drucken / PDF (`Strg+P`) druckt das aktive Blatt. Für ein PDF wählen Sie im Druckfenster den Drucker „Microsoft Print to PDF“ (oder „Als PDF speichern“). Was gedruckt wird, legen Sie in der Gruppe Seite einrichten der Registerkarte **Ansicht** des Menübands fest (`Alt+W`). Die Einstellungen gehören zum Blatt: Sie werden mit der Arbeitsmappe gespeichert, mit `Strg+Z` rückgängig gemacht und folgen einer Kopie des Blatts.

## Gruppe Seite einrichten (Registerkarte Ansicht) {#groupe-mise-en-page-onglet-affichage}

| Befehl | Wirkung |
| --- | --- |
| Seitenränder | Normal, Breit oder Schmal; „Benutzerdefinierte Seitenränder…“ öffnet das Dialogfeld Seite einrichten |
| Ausrichtung | Hochformat oder Querformat |
| Format | Papierformat: A4, A3, A5, B5, US Letter, US Legal, Tabloid, Executive |
| Druckbereich | „Festlegen“: Nur die Auswahl wird gedruckt. „Aufheben“: Der ganze verwendete Bereich wird wieder gedruckt |
| Umbrüche | „Einfügen“: Eine neue Seite beginnt über der aktiven Zelle und links davon (in Zeile 1 nur links). „Entfernen“ entfernt diese Umbrüche. „Alle zurücksetzen“ entfernt alle manuellen Umbrüche |
| Drucktitel | Oben zu wiederholende Zeilen und links zu wiederholende Spalten auf jeder Seite |

Das Einfügen oder Löschen von Zeilen und Spalten verschiebt den Bereich, die Titel und die Umbrüche mit.

## Skalierung {#mise-a-l-echelle}

**Breite** und **Höhe** passen das Blatt an eine Anzahl von Seiten an: Lattis verkleinert es (bis auf 10 %), damit es passt. „1 Seite“ breit und „Automatisch“ hoch bringt alle Spalten auf die Breite einer Seite. Beim Anpassen werden manuelle Seitenumbrüche wie in Excel ignoriert. **Skalierung** druckt mit einem festen Prozentsatz und beendet das Anpassen.

## Blattoptionen {#options-de-la-feuille}

„Gitternetzlinien anzeigen“ und „Überschriften anzeigen“ ändern nur den Bildschirm. „Gitternetzlinien drucken“ zeichnet alle Zellen. „Überschriften drucken“ fügt die Spaltenbuchstaben und Zeilennummern hinzu. Ohne Gitternetzlinien werden nur die Zellen von Tabellen umrahmt.

## Das Dialogfeld Seite einrichten {#la-boite-mise-en-page}

Die kleine Schaltfläche unten rechts in jeder Gruppe öffnet das Dialogfeld **Seite einrichten**. Es hat vier Register:

- **Papierformat**: Ausrichtung, Skalierung oder Anpassen an Seiten, Papierformat, erste Seitenzahl.
- **Seitenränder**: oben, unten, links, rechts, Kopf- und Fußzeile (in cm), horizontale und vertikale Zentrierung auf der Seite.
- **Kopfzeile/Fußzeile**: eine Vorlage („Seite 1 von ?“, Blattname, Dateiname…) oder drei freie Bereiche (links, Mitte, rechts). Die Schaltflächen fügen Codes ein: `&[Page]`, `&[Pages]`, `&[Date]`, `&[Heure]` (Uhrzeit), `&[Fichier]` (Datei), `&[Chemin]` (Pfad), `&[Onglet]` (Blatt). Diese Codes sind in allen Sprachen gleich. Schreiben Sie `&&` für ein einzelnes kaufmännisches Und.
- **Blatt**: Druckbereich (`$A$1:$F$40`), oben zu wiederholende Zeilen (`$1:$1`) und links zu wiederholende Spalten (`$A:$A`). Die Schaltfläche **Auswahl** übernimmt die Auswahl des Blatts. Gitternetzlinien, Überschriften, Schwarzweiß und die Seitenreihenfolge (erst nach unten, dann nach rechts, oder umgekehrt) werden ebenfalls hier festgelegt.

**Drucken…** übernimmt die Einstellungen und druckt dann.

## Seitenumbrüche im Raster {#sauts-de-page-dans-la-grille}

Nach dem Drucken oder dem Ändern des Seitenlayouts zeigen gepunktete Linien, wo die Seiten geteilt werden. Manuelle Umbrüche werden mit kräftigeren Strichen angezeigt, und ein Druckbereich wird umrahmt. Ansicht › **Seitenumbrüche** blendet sie ein oder aus.

## Was gedruckt wird {#ce-qui-s-imprime}

Das Blatt, wie das Raster es anzeigt: Werte und Zahlenformate, Farben, Schriftarten, Rahmen, Ausrichtung, Zeilenumbruch, bedingte Formatierung (Farben, Balken, Symbole), verbundene Zellen, Pivot-Tabellen und Diagramme, mit den Spaltenbreiten und Zeilenhöhen. Ausgeblendete Zeilen und Spalten werden nicht gedruckt. Lattis teilt die Seiten selbst: Jede hat die Größe des gewählten Papiers, mit Rändern, Kopf- und Fußzeile. Ein großes Blatt (100.000 Zeilen und mehr) wird Seite für Seite vorbereitet, mit dem Fortschritt in der Statusleiste.

Grenzen:

- Tausende Seiten: Das Druckfenster braucht eine Weile, um sie anzuzeigen. Ein Druckbereich oder ein Filter verringert ihre Zahl.
- Lassen Sie im Druckfenster die Ränder auf „Standard“ und die Skalierung auf 100 %. Andernfalls werden die von Lattis vorbereiteten Seiten verschoben.
- Bietet das Fenster die Option „Kopf- und Fußzeilen“ an, lassen Sie sie deaktiviert: Sie fügt die des Browsers hinzu (Datum, Adresse).

## Excel und OpenDocument {#excel-et-opendocument}

Das Seitenlayout geht mit der als `.xlsx` oder `.ods` exportierten Arbeitsmappe mit: Papier, Ausrichtung, Ränder, Skalierung oder Anpassen, Druckbereich, Titel, Seitenumbrüche, Gitternetzlinien, Überschriften, Zentrierung, Kopf- und Fußzeile. Beim Import einer `.xlsx` übernimmt Lattis das Seitenlayout jedes Blatts.

## Skripte {#scripts}

`sheets.page_setup()` liest das Seitenlayout des Blatts, und `sheets.set_page_setup({...})` ändert es (siehe [die Lua-Referenz](lua:sheets.set_page_setup)).
