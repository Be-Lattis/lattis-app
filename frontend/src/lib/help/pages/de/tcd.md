# PivotTables

Eine PivotTable fasst eine Tabelle zusammen: Summen nach Kunde, nach Monat, nach Region… Lattis berechnet sie in SQL (`GROUP BY`), was auch bei einer Million Zeilen schnell bleibt.

## Eine PivotTable erstellen {#creer-un-tableau-croise}

1. Klicken Sie in die Quelltabelle.
2. Einfügen › PivotTable. Die Statusleiste meldet „Klicken Sie auf die Zelle, in der die PivotTable platziert werden soll (Esc: abbrechen)“.
3. Klicken Sie auf die Zelle, in die die linke obere Ecke der PivotTable kommen soll. In der Zwischenzeit ist nichts blockiert: Sie können im Blatt scrollen, sich mit der Tastatur bewegen, das Blatt wechseln (die PivotTable wird dann auf dem anderen Blatt platziert) oder das Menüband verwenden. `Esc` (oder **Abbrechen** in der Statusleiste) bricht ab.
4. Die PivotTable wird platziert, und der Bereich **PivotTable-Felder** öffnet sich rechts.

Um sie allein auf einem neuen Blatt zu platzieren, wie es Excel standardmäßig tut, klicken Sie statt auf eine Zelle auf **Neues Blatt** in der Statusleiste.

## Eine PivotTable verschieben {#deplacer-un-tableau-croise}

- **Rechtsklick › Verschieben** auf der PivotTable, dann klicken Sie auf die Zelle, in die ihre linke obere Ecke kommen soll (Berichtsfilter inbegriffen), auf demselben oder einem anderen Blatt; `Esc` bricht ab.
- Oder ziehen Sie den Rand der Auswahl, wie bei einer Tabelle: Eine in der PivotTable getroffene Auswahl (eine einzelne Zelle genügt) nimmt die ganze PivotTable mit; mit `Strg` wird sie kopiert.
- Oder schneiden Sie die ganze PivotTable aus (`Strg+X`) und fügen Sie sie anderswo ein (`Strg+V`).

Die PivotTable wird genau an der gewählten Zelle platziert. Die Tabellen und anderen PivotTables, die sie überdeckt oder berührt, weichen nach rechts aus, mit einer leeren Spalte dazwischen, und schieben ihrerseits die an, auf die sie treffen; ein Klick in eine andere Tabelle schiebt diese also rechts neben die PivotTable. Diagramme und Schaltflächen, die über dem Raster schweben, bewegen sich nicht. Jedes Platzieren oder Verschieben wird in einem Schritt rückgängig gemacht (`Strg+Z`).

In die Zellen einer PivotTable kann nicht einzeln eingegeben werden: Doppelklicken Sie darauf, um ihre Felder zu ändern.

## Der Feldbereich {#le-volet-des-champs}

Die Liste oben zeigt die Spalten der Quelltabelle mit dem Symbol ihres Typs; die Suche filtert die Liste. Aktivieren Sie ein Feld, um es hinzuzufügen, oder ziehen Sie es in einen der vier Bereiche:

| Bereich | Rolle |
| --- | --- |
| Filter | Berichtsfilter, über der PivotTable |
| Spalten | Eine Spalte der PivotTable pro Wert des Felds |
| Zeilen | Eine Zeile der PivotTable pro Wert des Felds (mehrere Felder werden verschachtelt) |
| Werte | Die berechneten Zahlen |

Jeder Chip hat sein ▾-Menü: verschieben, entfernen, filtern und, für einen Wert, die Zusammenfassung und die Anzeige. Mit der Tastatur: `Tab` geht von einem Element zum nächsten, `Eingabe` öffnet das Menü eines Chips, `Entf` entfernt ihn, `Alt+↑` / `Alt+↓` verschieben ihn.

Eine berechnete Spalte kann noch nicht als Feld verwendet werden.

### Berechnetes Feld (nach einer Formel gruppieren) {#champ-calcule-regrouper-par-une-formule}

**Berechnetes Feld…** unter der Liste der Felder fügt ein Feld hinzu, dessen Wert für jede Zeile der Quelltabelle durch eine Formel über ihre Spalten berechnet wird, in eckigen Klammern geschrieben: `=JAHR([Datum])` gruppiert nach Jahr, `=TEXT([Datum];"mmmm")` nach Monat, `=LINKS([Code];2)` nach den ersten beiden Zeichen eines Codes, `=WENN([Betrag]>1000;"Groß";"Klein")` nach Größenklasse. Das Dialogfeld schlägt während der Eingabe Funktionen und Spalten vor und zeigt die ersten Werte, die die Formel ergibt (oder ihren Fehler). Das Feld kommt mit dem Symbol ƒ in die Liste und wird wie eine Spalte platziert: unter Zeilen oder Spalten gruppiert es die Zeilen nach seinem Ergebnis; unter Filter filtert es; unter Werte wird es zusammengefasst. Sein ▾-Menü in der Liste bearbeitet oder löscht es (es verlässt dann die Bereiche). Die PivotTable berechnet es bei jeder Aktualisierung neu; das Umbenennen einer Spalte der Quelle aktualisiert die Formel. `Strg+Z` macht es rückgängig. Skripte: `calc = {{name = "Jahr", text = "=JAHR([Datum])"}}` in der Konfiguration von `pivots.create` oder `pivots.set`.

## Zusammenfassung und Anzeige der Werte {#synthese-et-affichage-des-valeurs}

Zusammenfassungen: Summe, Anzahl, Eindeutige Anzahl, Mittelwert, Min, Max. Eine Zahlenspalte wird standardmäßig summiert, die anderen gezählt. Die Anzeige kann auch ein Prozentsatz sein: des Gesamtergebnisses, der Spaltensumme oder der Zeilensumme. Ein Zahlenformat (Währung, Prozent, Dezimalstellen) wird pro Wert gewählt.

## Beschriftungen sortieren und filtern {#trier-et-filtrer-les-etiquettes}

Das ▾ einer Zeilenbeschriftung (oder das ▾-Menü eines Chips in den Bereichen Zeilen und Spalten) sortiert das Feld: von A bis Z, von Z bis A, in der Reihenfolge der Quelle oder mit **Benutzerdefiniertes Sortieren…**, das die Werte des Felds in die gewünschte Reihenfolge bringt oder nach einer Liste (Tage, Monate, Listen der Arbeitsmappe), wie bei einer [Tabellenspalte](prise-en-main). Das ▾ einer Beschriftung filtert auch ihre Werte durch Ankreuzen. Ohne gewählte Sortierung folgen die Beschriftungen der Reihenfolge der Quelltabelle. Die gewählte Reihenfolge wird in der PivotTable gespeichert: Ändern sich die Daten, nimmt ein auftauchender Wert der Liste seinen Platz ein, die anderen folgen aufsteigend. Wie in Excel bilden die leeren Zellen des Felds unabhängig von der Sortierung immer das letzte Element: die letzte Zeile (oder Spalte) vor der Summe und „(Leere)“ am Ende der Liste im ▾-Menü.

## Aktualisierung {#mise-a-jour}

Die PivotTable folgt den Änderungen im Bereich nach einer kurzen Verzögerung, in einem einzigen Rückgängig-Schritt. Sie folgt auch den Daten der Quelltabelle. Der Bereich öffnet sich mit einem Klick in die PivotTable wieder.

## Spalten rechts hinzufügen {#ajouter-des-colonnes-a-droite}

Geben Sie einen Namen in die Zelle direkt rechts neben der Überschrift der PivotTable ein: Sie erhält eine **erfasste Spalte** innerhalb ihres Rahmens, mit grüner Überschrift (der Rest der PivotTable behält das Blau der berechneten Daten). Ihre Zellen werden wie gewöhnliche Zellen bearbeitet:

- **Ein Wert** wird Zeile für Zeile eingegeben (eine Notiz, ein Ziel, ein Kommentar). Er folgt seiner Zeilenbeschriftung: Ändert sich die PivotTable (Daten der Quelltabelle, Sortierung, Filter, Felder), bleibt er neben derselben Beschriftung. Eine Beschriftung, die verschwindet, legt ihre Werte **beiseite**; sie kommen mit ihr zurück. Der Feldbereich nennt, wie viele Zeilen beiseitegelegt sind, und **Beiseitegelegte Werte vergessen** löscht sie (Strg+Z zum Rückgängigmachen). Werden alle Felder unter Zeilen entfernt, werden alle Werte beiseitegelegt; sie kommen zurück, wenn das Feld zurückkommt. Ebenso, wenn das Feld unter Zeilen gewechselt wird: Die Werte werden pro Feld und pro Beschriftung gespeichert.
- **Eine Formel** (beginnend mit =) gilt wie eine berechnete Spalte für die ganze Spalte: `=[Anzahl]*2` liest den Wert der Zeile in der Spalte „Anzahl“ der PivotTable (oder in einer anderen erfassten Spalte), `=[Anzahl]/SUMME(TCD1[Anzahl])` vergleicht mit der ganzen Spalte (nur Detailzeilen), und eine Adresse wie `=B3` oder `=A20*[Anzahl]` liest eine Zelle des Blatts. Eine Formel wird auch in Summenzeilen berechnet. Ein in eine Formelspalte eingegebener Wert wird abgelehnt; das Löschen einer ihrer Zellen löscht die Formel (zuvor eingegebene Werte kommen zurück).
- **Summen**- und Teilsummenzeilen nehmen keinen Wert an.
- Umbenennen: Überschrift neu eingeben. Spalte entfernen: ihre Überschrift löschen (Entf auf der ganzen Spalte oder Überschrift gelöscht), ihre Werte gehen mit (Strg+Z zum Rückgängigmachen).

Erfasste Spalten bleiben am rechten Rand der PivotTable, wenn sie breiter wird (ein Wert oder Feld mehr unter Spalten), und schieben an, was neben ihnen steht, wie die PivotTable selbst. Sie bewegen sich mit ihr; eine Kopie der PivotTable kopiert auch ihre Werte. Eine Formel anderswo liest sie über ihre Adresse (`=E5`), und die SQL-Ansicht der PivotTable enthält sie.

## Eine PivotTable anderswo verwenden {#utiliser-un-tableau-croise-ailleurs}

Eine PivotTable hat einen Namen (oben im Bereich angezeigt): Sie kann in der [Registerkarte SQL](sql) wie eine Tabelle abgefragt werden und dient als Quelle für [Diagramme](graphiques). Die Funktion [GRUPPIERENNACH](fonction:GRUPPIERENNACH) liefert eine einfache Gruppierung direkt in einer Formel.
