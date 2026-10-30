# Diagramme

Ein Diagramm ist mit einer Tabelle oder einer PivotTable verbunden und wird mit ihr aktualisiert.

## Ein Diagramm einfügen {#inserer-un-graphique}

1. Klicken Sie in die Tabelle (oder die PivotTable), die dargestellt werden soll.
2. Einfügen › wählen Sie einen Diagrammtyp.
3. Das Diagramm wird auf dem Blatt platziert, und der Bereich **Diagramm** öffnet sich rechts.

Sind mehrere Spalten der Tabelle ausgewählt, bestimmen sie das Diagramm: Die erste Textspalte liefert die Kategorien, die Zahlenspalten die Datenreihen.

Das Diagramm lässt sich mit der Maus verschieben und an seinen Ziehpunkten in der Größe ändern. Ein Klick darauf öffnet den Bereich wieder; Rechtsklick: ausschneiden, kopieren, als Bild kopieren (zum Einfügen in eine E-Mail oder ein Dokument), löschen. Mit der Tastatur, sobald das Diagramm ausgewählt ist: Die Pfeiltasten verschieben es, Umschalt+Pfeiltasten ändern seine Größe, Tab geht zum nächsten Diagramm, Entf löscht es.

## Dargestellte Daten {#donnees-tracees}

Ein Diagramm zeigt nur die vom Filter der Tabelle angezeigten Zeilen; das Kontrollkästchen **Vom Filter ausgeblendete Zeilen** im Bereich nimmt sie alle wieder auf.

Ein Diagramm zeichnet höchstens 2.000 Punkte: Darüber hinaus meldet es dies unter der Zeichnung („Erste 2.000 Punkte von 50.000“). Eine zu lange Linie wird reduziert statt abgeschnitten: Jedes Intervall behält seinen ersten und letzten Punkt, sein Minimum und sein Maximum. Kastengrafiken und Histogramme werden immer auf allen Zeilen berechnet.

Werte übernehmen auf den Achsen, in den Beschriftungen und QuickInfos das Format ihrer Spalte (€, %, Dezimalstellen, Datumswerte). Eine Datumsspalte auf X ergibt eine Zeitachse für Linien- und Flächendiagramme und kann auch in einem Punktdiagramm gelesen werden.

## Typen {#types}

Säule, Balken, Linie, Fläche, Kreis, Ring, Punkt (XY), Kombination (Säulen und Linien, Sekundärachse möglich), 3D-Säule und 3D-Kreis, Wasserfall, Treemap, Netz, Trichter, Kastengrafik, Histogramm, Tachometer und **Karte** (Punkte oder Dichte auf einer OpenStreetMap-Karte, aus Spalten für Breiten- und Längengrad).

## Der Bereich Diagramm {#le-volet-graphique}

- **Typ**: ändert die Form, ohne die gewählten Daten zu verlieren; „Gestapelt“ für Säulen und Flächen.
- **Daten**: die Quelle (eine Tabelle oder eine PivotTable), die Kategoriespalte (horizontale Achse) und eine oder mehrere Wertereihen, in der mit den Pfeilen ▲ ▼ gewählten Reihenfolge; bei Linien unterbrechen **leere Zellen** die Linie (Lücke), zählen als null oder werden verbunden.
- **Farben**: die jeder Datenreihe, oder jeder Kategorie bei Kreis-, Ring-, Trichter- und Treemap-Diagrammen, sowie Zunahme, Abnahme und Summe beim Wasserfall.
- **Titel**: Diagramm- und Achsentitel.
- **Elemente**: Legende, Datenbeschriftungen; Sekundärachse für eine Reihe eines Kombinationsdiagramms.

## Karten {#cartes}

Eine Karte liest entweder zwei Spalten (Breitengrad, Längengrad) oder eine einzige Spalte, die beides enthält. Die Grundkarten stammen von OpenStreetMap: Sie benötigen eine Verbindung, und der Zugriff auf den Kachelserver erfordert Ihre Zustimmung. Diese Zustimmung hängt nicht von Skripten ab (sie gilt auch in der Webversion); nach einer Ablehnung erteilt sie die Schaltfläche **Hintergrundkarte anzeigen** der Karte. Die öffentliche Demo hat keine Hintergrundkarte: Sie stellt keine Verbindung her, und die Karte sagt es.

Zum Zoomen wählen Sie die Karte aus (ein Klick): Das Mausrad zoomt um den Mauszeiger, Ziehen verschiebt die Karte, ein Doppelklick vergrößert (Umschalt+Doppelklick verkleinert), und die Tasten **+** und **−** zoomen ebenfalls. Solange die Karte nicht ausgewählt ist, scrollt das Mausrad das Blatt oder das Dokument; Alt+Ziehen verschiebt das ausgewählte Diagramm. Die Schaltflächen **+**, **−** und **Zentrieren** erscheinen beim Darüberfahren oben rechts. Die gewählte Ansicht wird mit der Arbeitsmappe gespeichert (Strg+Z macht sie rückgängig); **Zentrieren**, hier oder im Diagrammbereich, kehrt zur an die Punkte angepassten Ansicht zurück.

**Färben nach** (Diagrammbereich, Anzeige als Punkte): Wählen Sie eine Spalte, und jeder eindeutige Wert erhält seine eigene Farbe, über das ganze Spektrum verteilt; eine Legende unten links benennt sie (Kontrollkästchen **Farblegende**, um sie auszublenden). Die Farben folgen der sortierten Reihenfolge der Werte, nicht der der Zeilen: Sortieren oder Filtern der Tabelle bringt sie nicht durcheinander. Ab 20 Werten behalten die 19 häufigsten eine Farbe, und die anderen sind grau („Sonstige“); leere Zellen sind hellgrau. Die Dichte bleibt eine einzige Ebene: Die Option wird dort nicht angeboten.

Beim Darüberfahren über einen Punkt werden sein Name, seine Farbe und sein Wert angezeigt. Ist die Karte ausgewählt, wählt ein Klick auf einen Punkt (ohne Ziehen) seine Zeile in der Tabelle aus, auf ihrem Blatt, sogar aus einem Dokument heraus; überlappen sich mehrere Punkte, ist es der dem Mauszeiger nächste. Der erste Klick auf eine nicht ausgewählte Karte wählt sie nur aus.

## Ein Diagramm kopieren {#copier-un-graphique}

Ein kopiertes Diagramm kann auf einem anderen Blatt oder in einem [Dokument](onglets) eingefügt werden, wo es mit seinen Daten verbunden bleibt.

## Bilder, Formen und Textfelder {#images-formes-et-zones-de-texte}

Einfügen › **Bild** (oder Strg+V eines kopierten Bildes), **Formen** oder **Textfeld**: Das Objekt wird an der aktiven Zelle platziert und folgt ihren Zeilen und Spalten. Ziehen Sie es, um es zu verschieben, ziehen Sie an seinen Ziehpunkten, um die Größe zu ändern (ein Bild behält seine Proportionen; Umschalt hebt das auf), doppelklicken Sie auf eine Form, um Text einzugeben. Die Leiste über dem Objekt stellt Farben, Linie, Transparenz und Text ein; Entf löscht, Strg+D dupliziert. Ein sehr großes Bild wird beim Einfügen verkleinert; höchstens 10 MB (1 MB in einer freigegebenen Arbeitsmappe).

## Trendlinien und Fehlerindikatoren {#tendance-et-erreurs}

Diagrammbereich › **Analyse** (Säulen, Linien, Flächen, Punkte, Blasen): Reihe wählen, dann eine **Trendlinie** (linear, exponentiell, logarithmisch, polynomisch, Potenz, gleitender Durchschnitt), mit Prognose vor oder nach den Daten, Formel und R². **Fehlerindikatoren**: fester Wert, Prozentsatz, Standardabweichung, Standardfehler oder Beträge aus einer Spalte. Die Typen **Blasen** (X, Y, Größe) und **Kurs** (Eröffnung, Hoch, Tief, Schluss, Volumen) stehen unter Weitere Diagramme.

## Sparklines {#sparklines}

Einfügen › **Sparklines**: ein kleines Diagramm in einer Zelle (Linie, Säule, Gewinn/Verlust). Die Spalten einer Tabellenzeile wählen (Jan bis Dez): jede Tabellenzeile erhält ihre eigene, neue Zeilen eingeschlossen. Außerhalb einer Tabelle Datenbereich und Position angeben. Dasselbe Menü auf einer Zelle mit Sparkline stellt hervorgehobene Punkte, Farben und Achse ein oder löscht sie. Das Erstellen wird unter Datei › Einstellungen › Funktionen eingeschaltet; vorhandene Sparklines werden immer angezeigt.
