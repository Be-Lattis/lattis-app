# Erste Schritte

Lattis funktioniert wie eine Tabellenkalkulation: ein endloses Raster, oben ein Menüband, unten die Blattregister. Der Unterschied liegt darunter: Jede Tabelle, die Sie eingeben, wird zu einer echten Datenbanktabelle, mit Datentypen und per SQL abfragbar.

## Erkannte Tabellen {#tableaux-detectes}

Geben Sie irgendwo auf dem Blatt ein paar Zeilen ein: eine Überschriftenzeile, dann Werte. Lattis erkennt den Datenblock und macht daraus eine **Tabelle**: Die Überschrift wird hervorgehoben, jede Spalte erhält einen Typ, und die Tabelle erhält einen Namen, den Sie im Tabellennamenfeld links neben der Bearbeitungsleiste ändern können (neuen Namen eingeben, dann Eingabe). Die erste Tabelle eines Blatts übernimmt den Namen des Registers (ohne Leerzeichen: „Umsatz 2026“ ergibt `Umsatz2026`), die folgenden heißen Tabelle1, Tabelle2… Wird das Register umbenannt, wird auch die Tabelle umbenannt, die seinen Namen trägt (nicht umgekehrt); diese Tabelle zeigt kein Namensschild, da das Register ihn bereits anzeigt.

- Es braucht mindestens zwei Spalten über zwei Zeilen (drei gefüllte Zellen eines 2 × 2-Quadrats): Eine einspaltige Liste oder eine einzelne Zeile bleibt aus freien Zellen.
- Die erste eingegebene Zeile ist die Überschrift, auch wenn sie wie die folgenden aussieht („Name | Ort“, dann „Lea | Paris“), außer sie enthält einen Wert, der kein Text ist: eine Zahl, ein Datum, eine Uhrzeit, einen Wahrheitswert, einen Prozentsatz, einen Betrag („Lea | 31“, „Land | 2024“, „10 | 20“). Dann sind alle eingegebenen Zeilen Daten, und die Überschrift kommt direkt darüber, mit den Namen „Spalte 1“, „Spalte 2“… in grauer Kursivschrift: Geben Sie dort den Namen jeder Spalte ein, oder Rechtsklick auf die 1. Zeile › „Erste Zeile als Überschriften verwenden“ (auch unter Rechtsklick › Tabelle): Sie wird an ihrer Stelle zur Überschrift, die Zeile mit „Spalte n“ verschwindet, Formeln werden angepasst, und eine Spalte wird wieder zu Zahlen, wenn ihr einziger Text der Name war. Soll eine Zahl ein Spaltenname sein, geben Sie sie nach einem Apostroph ein ('2024). Ist darüber kein freier Platz (erste Zeile des Blatts, eine Zelle, ein Titel oder eine Tabelle direkt darüber), bleiben die Zellen frei: Start › Als Tabelle formatieren (`Strg+L`) macht daraus eine Tabelle, deren erste Zeile die Überschriften liefert.
- Eine direkt unter einer Tabelle eingegebene Zeile erweitert sie, ebenso eine direkt rechts daneben eingegebene Spalte.
- Ein aus Excel oder einem anderen Programm eingefügter Block wird zu einer Tabelle, mit derselben Überschriftenregel wie bei der Eingabe: Seine 1. Zeile ist die Überschrift, wenn sie nur Text enthält; eine Zahl, ein Datum… („Land | 2024“, „10 | 20“) macht sie zu einer Datenzeile, unter „Spalte 1“, „Spalte 2“… (Rechtsklick › „Erste Zeile als Überschriften verwenden“, um sie zurückzuholen). In der ersten Zeile des Blatts eingefügt, ohne Platz darüber, nimmt die Überschrift „Spalte n“ den Platz der 1. eingefügten Zeile ein, und der Block rückt eine Zeile nach unten.
- Nach den Daten eingegebene Überschriften: Geben Sie die Namen in die Zeile direkt über der Tabelle ein. Sobald zwei Namen über einer Überschrift stehen, die wie Daten aussieht („Lea | 31“, „Spalte 1“), bietet die Statusleiste „Als Überschriften verwenden“ an; andernfalls Rechtsklick auf eine dieser Zellen › „Als Überschriften von „Tabelle1“ verwenden“ (oder Rechtsklick auf die Überschrift › Tabelle › „Zeile darüber als Überschriften verwenden“). Die Namen ersetzen die Spaltennamen (eine leer gelassene Zelle behält den bisherigen), die alte Überschrift wird zur 1. Datenzeile, als Zahlen, wenn es Zahlen waren, und Formeln werden angepasst; `Strg+Z` macht es rückgängig. Ein einzelnes darüber eingegebenes Wort bleibt ein Titel.
- Ziehen Sie eine Tabelle neben eine andere, um sie zusammenzuführen.
- `Entf` auf Zellen einer Tabelle: Eine Zeile, in der nichts Eingegebenes mehr steht, wird gelöscht, als wäre sie nie eingegeben worden (eine berechnete Spalte, die ausgeblendete Spalte eines Auswahlblatts oder die automatisch vergebene Nummer eines ganzzahligen Schlüssels halten sie nicht; die Statusleiste meldet „N leere Zeilen gelöscht“, `Strg+Z` holt sie zurück). Eine Zeile, die schon leer war und in der `Entf` nichts gelöscht hat, bleibt bestehen.
- Was nicht in einer Tabelle steht, bleibt eine **freie Zelle**, wie in Excel: ein Titel, ein Parameter, eine Summe.

Alles lässt sich mit `Strg+Z` rückgängig machen, auch das Anlegen einer Tabelle.

Ein **Rechtsklick** in einer Tabelle bietet zuerst an, was nur dort gilt (Wechseln zu, Erste Zeile als Überschriften verwenden, Diese Zeile aus der Tabelle nehmen auf der letzten Zeile, Zeile darüber als Überschriften verwenden auf einer Überschrift…), dann Ausschneiden, Kopieren, Einfügen; Inhalte löschen, Zellen formatieren… (`Strg+1`), Bedingte Formatierung…; die Notiz. Zuletzt folgen die Untermenüs jedes Objekts, mit seinem Namen betitelt: **Sortieren und Filtern**, **Zeile „12“** (ihr Schlüssel, wenn die Tabelle einen hat: oberhalb oder unterhalb einfügen, Zeile löschen), **Spalte „Name“** (umbenennen, Typ und Schlüssel, Überprüfung, Register filtern, in Werte konvertieren, links oder rechts einfügen, Spalte löschen) und **Tabelle „Messungen“** (Überschriften, Freigabe, In Bereich konvertieren, Tabelle löschen). Zeilennummern, Spaltenbuchstaben, PivotTables (**PivotTable „Umsatz“**), Diagramme und Schaltflächen folgen derselben Reihenfolge.

### Kopfzeilengruppen (Zeile darüber) {#groupes-d-en-tetes-ligne-du-dessus}

Steht über den Überschriften eine Zeile mit Gruppen (WALDSTÜCK über Name, Status und Parzelle), Rechtsklick auf die Überschrift › Tabelle › „Überschriften auf 2 Zeilen (Gruppen darüber)“: Diese Zellen, verbunden oder nicht, werden zu den Gruppen der Tabelle und wie in Excel über den Überschriften angezeigt. Eine Spalte heißt dann „Gruppe.Name“: `Bestand[WALDSTÜCK.Name]` in einer Formel, `"WALDSTÜCK.Name"` in SQL. Geben Sie im Gruppenband etwas ein, um eine Gruppe zu benennen oder umzubenennen (Formeln werden angepasst), löschen Sie den Inhalt, um sie zu entfernen. „Kopfzeilengruppen entfernen“ setzt die vollständigen Namen wieder in eine einzige Zeile.

Eine Spalte ohne Gruppe bleibt **einstufig**: Ihre Zelle im Band und ihre Überschrift bilden eine einzige, über beide Zeilen verbundene Zelle, und ihr Name hat keinen Punkt. Lattis passt sich selbst an: Eine Gruppe über einer einzigen Spalte, deren Überschrift keinen echten Namen hat (Topografie über „Spalte 17“), wird zum Namen der Spalte, und eine Gruppe, die die Überschrift wiederholt (ENTDECKER über Entdecker), verschwindet. Zum Anpassen von Hand mit Start › Verbinden und zentrieren:

- **Bandzelle und Überschrift einer Spalte** › Verbinden: Die Spalte wird einstufig; sie behält ihre Unterüberschrift oder übernimmt den Namen der Gruppe, wenn ihre Überschrift automatisch war;
- **eine einstufige Spalte** › Zellverbund aufheben (dieselbe Schaltfläche): Die Bandzelle trennt sich von der Überschrift; geben Sie dort die Gruppe ein (wenn Sie nichts eingeben, bleibt sie einstufig). „Gruppe.Name“ in die Überschrift einzugeben bewirkt dasselbe;
- **mehrere benachbarte Bandzellen** › Verbinden: eine einzige Gruppe über diesen Spalten, die erste, die sie trugen („Gruppe“, wenn keine vorhanden war: Geben Sie ihren Namen im Band ein);
- **eine Gruppe über mehreren Spalten** › Zellverbund aufheben: Die Gruppe bleibt auf ihrer ersten Spalte, die anderen werden einstufig.

Jedes Mal werden die Spalten umbenannt (Formeln und SQL werden angepasst), in einem einzigen Rückgängig-Schritt, und die Statusleiste meldet, was sich geändert hat. Das Band mit Daten, mit Zellen außerhalb der Tabelle oder mit einer anderen Tabelle zu verbinden, wird abgelehnt.

## Spaltentypen {#types-de-colonnes}

Jede Spalte hat einen Typ, der aus Ihrer Eingabe abgeleitet wird:

| Typ | Beispiele |
| --- | --- |
| Text | Paris, Ref-001 |
| Ganzzahl | 12, -3 |
| Dezimalzahl | 12,5; 1.234,50 € |
| Datum | 15.01.2026; mit Uhrzeit: 15.01.2026 14:30 oder 14:30 15.01.2026 |
| Wahr/Falsch | WAHR, FALSCH |
| Berechnet | eine auf die ganze Spalte angewendete Formel |

Ein Wert, der nicht mehr zum Typ passt, erweitert die Spalte, wenn möglich (eine Dezimalzahl in einer Spalte mit Ganzzahlen macht daraus Dezimalzahlen, und die Statusleiste meldet es); andernfalls wird die Zelle als Fehler markiert. Der Typ wird im Menü der Spaltenüberschrift geändert.

Der Text „null“, eingegeben, eingefügt oder importiert (CSV, Excel), lässt die Zelle leer; geben Sie 'null ein, um das Wort zu behalten. In einer Tabelle zeigt eine leere Zelle einen grauen Strich „–“ (nichts wird kopiert oder exportiert); eine leere freie Zelle bleibt weiß.

## Sortieren und filtern {#trier-et-filtrer}

Das ▾ einer Spaltenüberschrift (oder Rechtsklick, oder Daten › Sortieren und Filtern) sortiert die Tabelle nach dieser Spalte: von A bis Z, von Z bis A oder in einer **benutzerdefinierten Reihenfolge**. Das Sortieren ändert die Reihenfolge der Zeilen dauerhaft; leere Zellen kommen immer ans Ende. Dasselbe Menü filtert die Spalte nach Werten („(Leere)“ am Ende der Liste) oder nach Bedingung.

### Benutzerdefiniertes Sortieren {#tri-personnalise}

**Benutzerdefiniertes Sortieren…** öffnet die Liste der Werte der Spalte, die Sie in die gewünschte Reihenfolge bringen, wie die benutzerdefinierten Listen von Excel: ziehen Sie sie, verwenden Sie die Schaltflächen Nach oben, Nach unten, Ganz nach oben, Ganz nach unten oder die Tastatur `Alt+↑` / `Alt+↓` (`Alt+Pos1` / `Alt+Ende`). Wählen Sie **In dieser Reihenfolge** oder **Umgekehrt**, dann OK. Werte werden ohne Berücksichtigung der Groß-/Kleinschreibung verglichen; Werte, die nicht in der Reihenfolge stehen, folgen aufsteigend, und leere Zellen bleiben am Ende.

- **Integrierte Listen**: Tage (Montag… Sonntag), Kurze Tage, Monate (Januar… Dezember), Kurze Monate. Passen die Werte der Spalte zu einer Liste, wird sie gleich angeboten.
- **Listen der Arbeitsmappe**: Aktivieren Sie „Diese Reihenfolge als benutzerdefinierte Liste speichern“ und benennen Sie sie (Größen, Prioritäten…); sie wird dann für jede andere Spalte oder jedes PivotTable-Feld angeboten und reist mit der Datei.
- Die Reihenfolge wird **für die Spalte gespeichert**: Sie wird beim nächsten benutzerdefinierten Sortieren wiederverwendet, und eine von einer Abfrage geladene Tabelle wird bei jeder Aktualisierung erneut in dieser Reihenfolge sortiert (solange keine gewöhnliche Sortierung sie ersetzt hat). Werte der Reihenfolge, die in den Daten fehlen, bleiben darin, für den Fall, dass sie auftauchen.
- `Strg+Z` macht das Sortieren in einem Schritt rückgängig. In Skripten: `tables.sort_custom`, `tables.sort_lists`, `tables.set_sort_lists`, `columns.sort_order`.

## Schlüssel {#cles}

Ein **Schlüssel** ist eine Spalte, deren Werte eindeutig und alle ausgefüllt sind: eine Kundennummer, eine Artikelnummer. Er identifiziert jede Zeile, wird am Rand anstelle der Zeilennummern angezeigt und dient den Nachschlagen mit [REFERENZ](fonction:REFERENZ).

Wenn Sie eine Tabelle anlegen (Eingabe, Einfügen, Als Tabelle formatieren), wird ihre erste Spalte automatisch zum Schlüssel, wenn sie nur Ganzzahlen enthält, eindeutig und alle ausgefüllt, und ihre Überschrift **ID** lautet oder ihre Werte der Reihe nach **1, 2, 3…** sind. Die Statusleiste meldet es („Schlüssel auf ID gesetzt“), und der 🔑 erscheint in der Überschrift; `Strg+Z` macht es zusammen mit dem Anlegen rückgängig, Rechtsklick › Schlüssel entfernen entfernt ihn allein. Eine importierte Datei behält ihre Tabellen, wie sie sind.

In anderen Fällen, wenn sich die erste Spalte einer Tabelle dafür eignet, schlägt Lattis sie dezent vor, und ein Klick genügt. „Nicht mehr vorschlagen“ entfernt den Vorschlag für diese Tabelle, die dann ihre Zeilennummern behält.

**Automatische Nummerierung.** Eine Zeile, die einer Tabelle mit ganzzahligem Schlüssel hinzugefügt wird, erhält die nächste Nummer (die größte plus 1), ob sie unter der Tabelle eingegeben, eingefügt, in der Mitte eingefügt oder von einem Skript hinzugefügt wird; die anderen Zeilen werden nicht neu nummeriert. Eine erste Spalte, die einer regelmäßigen Reihe folgt, ohne Schlüssel zu sein (10, 20, 30…), wird für eine am Ende hinzugefügte Zeile ebenfalls ausgefüllt. Ein Wert, den Sie in die Spalte eingeben, ersetzt die vorgeschlagene Nummer. Die QuickInfo der Überschrift weist auf eine nummerierte Spalte hin.

Wenn Sie in einer Tabelle ohne Schlüssel ein Nachschlagen eingeben ([REFERENZ](fonction:REFERENZ), SVERWEIS, XVERWEIS, EXISTIERT…), fragt Lattis, was es verwenden soll:

- **die Zeilennummer** der Tabelle: 1 für die erste Zeile, in der Reihenfolge der Tabelle, die sich ändert, wenn Sie sortieren oder Zeilen einfügen. Ein SVERWEIS wird zu `INDEX(…; VERGLEICH(…; 0))`: die erste gefundene Zeile, Duplikate erlaubt. Die Wahl gilt für die ganze Tabelle: Die Frage kommt nicht wieder;
- **die nachgeschlagene Spalte**, die zum Schlüssel der Tabelle wird: Ihre Werte müssen eindeutig und alle ausgefüllt sein, und ein Duplikat wird dort dann abgelehnt.

`Strg+Z` macht die Formel und diese Wahl auf einmal rückgängig. Eine Tabelle, die bereits einen Schlüssel hat, fragt nichts, und eine eingefügte oder importierte Formel behält die Zeilennummern, ohne zu fragen.

## Blätter zusammenführen {#regrouper-des-onglets}

Viele Arbeitsmappen haben **ein Blatt pro Monat, pro Jahr oder pro Stadt**, mit denselben Spalten. Lattis erkennt sie: Ihre Register sind mit derselben farbigen Linie unterstrichen, und ein Chip „⧉ 12 ähnliche Blätter“ folgt der Reihe (beim Darüberfahren werden die betroffenen Register hervorgehoben). Nach dem Import einer Excel-Arbeitsmappe, beim Öffnen oder wenn Sie ein Blatt der Reihe zum dritten Mal kopieren (sogar bevor Sie die Kopie umbenennen), bietet die Statusleiste es einmal an: **Zusammenführen…**, **Später** (der Chip bleibt) oder **Nicht mehr vorschlagen** (in der Datei gespeichert: Eine Kollegin, die sie erhält, wird nicht erneut gefragt; auch per Rechtsklick auf den Chip).

Blätter gehören zusammen, wenn mindestens zwei Drittel ihrer Spalten von einem Blatt zum nächsten wiederzufinden sind: Eine im Laufe des Jahres hinzugefügte Spalte bleibt dort leer, wo sie fehlte, und eine umbenannte Spalte („Betrag €“) wird ihrem Gegenstück („Betrag“) zugeordnet, was der Bereich immer anzeigt. Registernamen einer Reihe werden erkannt: Monate, Jahre, Quartale, Wochen, Datumswerte, Zahlen- oder Buchstabenfolgen („Los 1“, „Los 2“; „Anhang A“, „Anhang B“), Gemeinden.

**Daten › Blätter zusammenführen…** (oder Rechtsklick auf eine Auswahl von Registern) öffnet den Bereich:

1. **Blätter**: die zusammenzuführenden Tabellen, zum Ankreuzen, in der Reihenfolge der Register;
2. **Kategoriespalte**: ihr Name, aus den Registernamen vorausgefüllt (Monat, Jahr, Quartal, Ort, der gemeinsame Teil wie „Los“, sonst Kategorie) und immer änderbar, ihr Typ und der Wert, der von jedem Blatt übernommen wird, bei Bedarf zu korrigieren. Sie kommt an die erste Stelle der Tabelle, mit einer Dropdownliste ihrer Werte;
3. **Spalten**: die Zuordnung (✓ vorhanden, ∅ fehlt, „Betrag € → Betrag“ zugeordnet) und der endgültige Typ; eine Spalte, die zu Text wird, wird gekennzeichnet;
4. **Danach**: der Name der Tabelle und des neuen Blatts und was aus den alten Blättern wird: **gelöscht** (Standard) oder **ausgeblendet behalten**, ihre Tabellen umbenannt in `Januar_alt`;
5. **Vorschau**: die ersten Zeilen und die Gesamtzahl der Zeilen, dann die Übersicht über die Formeln, die auf die alten Blätter verwiesen: „14 Formeln übertragen, 2 von Hand zu korrigieren“, mit der Liste (Blatt, Zelle, Formel vorher → nachher).

**Zusammenführen** erledigt alles in einem einzigen Schritt: `Strg+Z` gibt die ursprünglichen Blätter zurück. Formatierungen und fehlerhafte Eingaben folgen ihren Zeilen. Der **Schlüssel** bleibt erhalten, wenn sich kein Wert von einem Blatt zum anderen wiederholt; andernfalls hat die zusammengeführte Tabelle keinen Schlüssel (die Werte bleiben), und Lattis meldet es mit einem Beispiel: „Kein Schlüssel: F-001 steht im Januar und im Februar“.

Was auf die alten Tabellen verwies, wird auf die zusammengeführte Tabelle **übertragen**, beschränkt auf den Monat (das Jahr, den Ort) des alten Blatts, im selben Rückgängig-Schritt und mit denselben Werten. Ein Blatt „Übersicht“, das `SUMME(Januar[Betrag])`, `SUMME(Februar[Betrag])`… berechnete, wird so übertragen:

| Vorher | Nachher |
|---|---|
| `SUMME(Januar[Betrag])` | `SUMMEWENNS(Umsatz[Betrag]; Umsatz[Monat]; "=Januar")` |
| `MITTELWERT`, `MAX`, `MIN` einer Spalte (oder einer Berechnung über die Spalten, `SUMME(Januar[Preis]*Januar[Menge])`) | `MITTELWERTWENNS`, `MAXWENNS`, `MINWENNS`, `SUMMEWENNS` |
| `ANZAHL2(Januar[Kunde])` | `ZÄHLENWENNS(Umsatz[Kunde]; "<>"; Umsatz[Monat]; "=Januar")` |
| `SUMMEWENN`, `ZÄHLENWENN`, `SUMMEWENNS`… | dieselben, als `…WENNS`, mit dem Monatskriterium ergänzt |
| `ZEILEN(Januar)` | `ZÄHLENWENNS(Umsatz[Monat]; "=Januar")` |
| `ANZAHL`, `MEDIAN`, mehrere Spalten in `MAX` oder `MITTELWERT`, `EINDEUTIG(Januar[Kunde])`… | die gefilterte Spalte: `FILTER(Umsatz[X]; Umsatz[Monat]="Januar")` |
| eine PivotTable auf Januar | dieselbe auf Umsatz, mit einem Berichtsfilter Monat = Januar |
| ein Diagramm auf Januar | dasselbe auf Umsatz, mit einem Filter Monat = Januar (in seinem Bereich angezeigt, entfernbar) |

Berechnete Spalten anderer Tabellen und Regeln der bedingten Formatierung folgen denselben Regeln. **Von Hand zu korrigieren** bleiben, mit dem Grund: eine bestimmte Zeile (`Januar[Betrag][3]`), `REFERENZ`, `SVERWEIS` und `EXISTIERT` (sie schlagen einen Schlüssel in einer benannten Tabelle nach), `TEILERGEBNIS`, die ganze Tabelle (`FILTER(Januar; …)`), eine aus einer alten Tabelle übernommene Prüfliste, eine Abfrage; SQL-Text und Lua-Skripte werden nicht erkannt. In diesem Fall werden die alten Blätter standardmäßig ausgeblendet behalten, damit diese Formeln weiter funktionieren; andernfalls werden sie gelöscht. Ohne Kategoriespalte unterscheidet nichts mehr die Blätter: Alles bleibt zu korrigieren. Danach ist eine Übersicht nach Monat besser eine PivotTable Monat × Betrag, die neuen Zeilen von selbst folgt.

Das Zusammenführen wird abgelehnt auf einem geschützten Blatt, auf einer gefilterten Tabelle (zuerst den Filter entfernen), auf einer von einer Abfrage geladenen oder auf einem Server geteilten Tabelle.

### Blätter mit mehreren Tabellen {#onglets-a-plusieurs-tableaux}

Ein Blatt „2025“ enthält oft mehrere Tabellen: Umsätze und Ausgaben, die sich jedes Jahr ändern, aber auch eine Preisliste, die unverändert von einem Blatt zum nächsten kopiert wird. Lattis führt dann **ganze Blätter** zusammen: Es ordnet die Tabellen von einem Blatt zum nächsten einander zu, Familie für Familie (dieselben Spalten, wie oben), und der Bereich zeigt unter **Weitere Tabellen auf den Blättern**, was aus jeder wird:

- **zusammenführen** (Tabellen, die sich ändern: Umsätze, monatliche Notizen): eine zusammengeführte Tabelle mit der Kategoriespalte, die dem Auswahlblatt **folgt**;
- **eine einzige Kopie behalten** (angeboten, wenn die Daten überall gleich sind: die Preisliste): die des letzten Blatts, ohne Kategoriespalte, für alle Jahre gleich. Weicht eine Kopie ab („Preise ändern sich 2024“), kennzeichnet der Bereich dies und schlägt stattdessen vor, sie zusammenzuführen, damit nichts verloren geht;
- **auf ihrem Blatt lassen** (angeboten für eine Tabelle, die nur auf einem Blatt vorkommt): Das Blatt bleibt dann erhalten, sichtbar, mit einem orangefarbenen Register, um es wiederzufinden, und der Bereich meldet es.

Das zusammengeführte Blatt übernimmt das **Layout des neuesten Blatts**: Jede zusammengeführte Tabelle nimmt den Platz ihrer eigenen ein, was darunter steht, rückt um so viel nach unten, wie sie gewachsen ist, und Tabellen, die auf diesem Blatt fehlen, folgen darunter. Die **freien Zellen** der Blätter werden übertragen (Kontrollkästchen im Bereich zum Deaktivieren): Ein Titel, eine Beschriftung oder eine Summe, die überall identisch ist, wird nur einmal behalten; `=SUMME(Umsatz2025[Betrag])` unter der Tabelle wird zu `=SUMME(Umsatz[Betrag])`, das die Summe des angezeigten Jahres ergibt; ein Wert, der sich von Blatt zu Blatt ändert („Ziel: 5.000“ 2024, „6.000“ 2025, ein Titel „Januar 2025“), wird zu einer **Zelle pro Jahr**: Sie zeigt den Wert des gewählten Jahres, und was Sie dort eingeben, gilt nur für dieses Jahr (leer bei „(Alle Jahre)“).

Die Kategorie kann auch aus einer **Zelle** statt aus dem Registernamen stammen (ein Titel „Januar 2025“ in A1 jedes Blatts, wenn die Blätter Tabelle1, Tabelle2… heißen): Der Bereich schlägt die zuverlässigste Quelle vor, auszuwählen unter **Übernommen aus**. Ähnliche Tabellen, die **auf demselben Blatt untereinander** unter Titeln stehen („Paris“, „Lyon“, „Marseille“), werden ebenso zusammengeführt, wobei die Kategorie aus den Titeln stammt. Hat jedes Blatt eine Spalte **Datum**, deren Werte alle in seinen Monat fallen, ist die Spalte Monat überflüssig: Das Kontrollkästchen „Keine Spalte hinzufügen“ ist standardmäßig aktiviert (nach Datum zu filtern genügt); deaktivieren Sie es, um die Spalte trotzdem hinzuzufügen.

**Dem Jahr folgen**: Auf einem Blatt mit Auswahl trägt eine Tabelle, die folgt, in der Ecke ihres Rahmens ein kleines **Abzeichen** mit dem angezeigten Jahr; eine Tabelle ohne Abzeichen ist für alle Jahre gleich. Rechtsklick auf die Tabelle › Tabelle › **Dem Jahr folgen** (oder Klick auf das Abzeichen) aktiviert oder deaktiviert es: Aktivieren fügt die Spalte Jahr als letzte hinzu, gefüllt mit dem angezeigten Jahr (ausgeblendet, solange ein Jahr gewählt ist); Deaktivieren entfernt sie, wenn sie nur ein Jahr enthält, behält sie andernfalls (nichts geht verloren), und die Tabelle zeigt alle ihre Zeilen. Eine Tabelle, die auf einem auf ein Jahr eingestellten Blatt formatiert wird (Einfügen › Tabelle), folgt ihm automatisch; die Statusleiste meldet es. `Strg+Z` macht es rückgängig. Skripte: `tables.follow_choice("Preise", false)`.

## Formeln {#formules}

Eine Formel beginnt mit `=` und verweist auf Spalten über ihren Namen. In eine Tabellenspalte eingegeben, gilt sie für die ganze Spalte:

```
=[Menge]*[Stückpreis]
```

Siehe [Formeln](formules) für Bezüge und die Liste der Funktionen. Während der Eingabe öffnet `F1` die Seite der Funktion unter dem Cursor.

## Auswahlblatt (ein Jahr, ein Monat, eine Stadt nach der anderen) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

Statt eines Blatts pro Jahr behalten Sie eine einzige Tabelle mit einer Spalte Jahr und machen ihr Blatt zu einem **Auswahlblatt**: Rechtsklick auf das Register › **Auswählen nach** › die Spalte (Lattis schlägt die mit wenigen verschiedenen Werten vor). Das Register wird zu „Umsatz · Jahr ▾“; sein kleiner Pfeil (oder `Alt+↓` auf dem Register) klappt die Liste der Werte auf:

- **ein Wert** (2025): Alle Tabellen des Blatts, die eine Spalte mit diesem Namen haben, zeigen nur deren Zeilen, und die Spalte Jahr, überall gleich, wird ausgeblendet. Tabellen ohne diese Spalte (eine Preisliste) ändern sich nicht. Die PivotTables des Blatts zählen nur dieses Jahr, wie bei einem vorgegebenen Berichtsfilter; `TEILERGEBNIS`, Diagramme und die Suche folgen ihm, wie sie einem Filter folgen;
- **(Alle Jahre)**: Nichts ist mehr ausgeblendet, die Spalte erscheint wieder;
- **Neues Jahr…** schlägt das nächste vor (2026 nach 2025, Dezember nach November, Los 4 nach Los 3), fügt es der Dropdownliste der Spalte hinzu und zeigt es an: Das Blatt sieht leer aus, bereit zum Ausfüllen, wie ein neues Blatt. `Strg+Z` macht es rückgängig.

**Über die Spalte**: Auf einer bereits ausgefüllten Tabelle bewirkt Rechtsklick auf die Überschrift der Spalte Jahr (oder ihr ▾-Menü) › **Blatt nach dieser Spalte filtern** dasselbe, ohne über das Register oder das Zusammenführen von Blättern zu gehen. Das Blatt beginnt mit allen Werten: Wählen Sie das Jahr mit seinem ▾-Pfeil. Bei einer berechneten Spalte, einer leeren oder einer mit mehr als 100 verschiedenen Werten ist der Eintrag abgeblendet, mit einer QuickInfo, die den Grund nennt; auf der bereits als Auswahl verwendeten Spalte wird er zu **Blatt nicht mehr filtern**. `Strg+Z` macht es rückgängig.

**Über eine Formel**: Rechtsklick auf das Register › **Auswählen nach** › **Einer Formel…**, wenn der auszuwählende Wert keine Spalte ist, sondern aus den Spalten jeder Zeile berechnet wird: `=JAHR([Datum])` ergibt ein Blatt pro Jahr ohne Spalte Jahr, `=TEXT([Datum];"mmmm")` pro Monat, `=WENN([Betrag]>1000;"Groß";"Klein")` pro Größenklasse. Das Dialogfeld schlägt während der Eingabe Funktionen und Spalten vor, errät den Namen der Kategorie (Jahr, Monat…, änderbar) und zeigt die ersten Werte der Formel sowie die Tabellen des Blatts, die ihr folgen werden: diejenigen, die alle Spalten haben, auf die sie verweist (die anderen ändern sich nicht; Rechtsklick › Tabelle › **Folgen** nimmt sie heraus oder wieder auf). Die Liste des Registers sortiert die Werte in natürlicher Reihenfolge (2024 vor 2025, Januar vor Februar). Keine Spalte zum Ausfüllen, also kein „Neues Jahr…“: Eine unter der Tabelle eingegebene Zeile erhält nichts automatisch, bleibt aber während der Eingabe angezeigt, bis sich der Wert des Registers das nächste Mal ändert; dann kommt sie zu dem Wert, den ihre Formel ergibt. `=BLATT.AUSWAHL()` gibt den gewählten Wert zurück (eine Zahl bei `=JAHR(…)`); die PivotTables des Blatts, die auf einer Tabelle mit den Spalten der Formel aufbauen, zählen nur den gewählten Wert. Das Umbenennen einer referenzierten Spalte aktualisiert die Formel. Skripte: `sheets.choose_by_formula("Umsatz", "=JAHR([Datum])", "Jahr")`.

**Über eine PivotTable**: Rechtsklick auf ein Feld in Zeilen, Spalten oder Filtern im PivotTable-Bereich (oder sein ▾ im Raster) › **Blatt nach diesem Feld filtern**. Das Auswahlblatt wird auf dem Blatt der PivotTable eingerichtet, nach der Spalte ihrer Quelltabelle: Die PivotTable zählt nur den gewählten Wert, wie die anderen PivotTables des Blatts, die auf einer Tabelle mit dieser Spalte aufbauen, und die Tabellen des Blatts, die sie haben. Die Quelltabelle wird, wenn sie auf einem anderen Blatt steht, nicht gefiltert; ihre Werte füllen die Liste des Registers. Das funktioniert auch für eine PivotTable, die auf einer von einer Abfrage geladenen Tabelle aufbaut.

Eine unter der Tabelle oder im grauen Band der Zeilen anderer Jahre eingegebene Zeile erhält automatisch das angezeigte Jahr; ebenso eine eingefügte Zeile. Die Auswahl ist kein Filter: „Filter löschen“ lässt sie unberührt, und Ihre ▾-Filter kommen hinzu. `Strg+Alt+Bild↓` und `Strg+Alt+Bild↑` wechseln zum nächsten oder vorherigen Jahr; die Statusleiste erinnert daran: „Jahr: 2025 (Auswahl des Blatts)“.

Freie Formeln **auf dem Blatt** sehen dessen Tabellen gefiltert: `=SUMME(Umsatz[Betrag])` unter der Tabelle ergibt die Summe des angezeigten Jahres, wie im alten Blatt „2025“. Die auf anderen Blättern, die Registerkarte SQL und Skripte sehen alle Zeilen. `=BLATT.AUSWAHL()` gibt den gewählten Wert zurück, für eine Übersicht, die dem Blatt folgt: `=SUMMEWENNS(Umsatz[Betrag]; Umsatz[Jahr]; BLATT.AUSWAHL())`.

Das Jahr zu wechseln ist Navigation, wie das Wechseln des Blatts: Es ist kein Rückgängig-Schritt, aber der zuletzt gewählte Wert wird in der Datei gespeichert und kommt beim Öffnen zurück (hat er keine Zeilen mehr, zeigt das Blatt den letzten Wert, außer direkt nach „Neues Jahr…“). Das Umbenennen der Spalte benennt die Kategorie um: Das Register folgt. Rechtsklick › Auswählen nach › „Nicht mehr auswählen nach…“ entfernt die Auswahl (rückgängig zu machen). In einer geteilten Arbeitsmappe bleibt die Auswahl auf Ihrem Computer: Sie wird nicht an die anderen gesendet.

Skripte: `sheets.choice()`, `sheets.set_choice("Umsatz", 2025)`, `sheets.choose_by("Umsatz", "Jahr")` (für das Blatt einer anderswo platzierten PivotTable: `sheets.choose_by{table = "Umsatz", column = "Jahr", sheet = "Übersicht"}`), `sheets.new_choice("Umsatz", 2026)` (siehe [Lua](lua)).

## Speichern {#enregistrer}

Eine Arbeitsmappe ist eine einzige `.lattis`-Datei (eine SQLite-Datenbank und ihre Beschreibung). `Strg+S` speichert. Wird die Arbeitsmappe oder das Lattis-Fenster mit ungespeicherten Änderungen geschlossen, fragt Lattis zuerst, ob sie gespeichert werden sollen; beendet sich Lattis, ohne zu speichern (Absturz, erzwungenes Herunterfahren), werden die Änderungen beim nächsten Start zur Wiederherstellung angeboten (siehe [Dateien und Wiederherstellung](fichiers)).

Der Rückgängig-Verlauf wird nie in der Datei gespeichert: `Strg+Z` funktioniert, bis die Arbeitsmappe geschlossen wird, auch nach `Strg+S`, aber eine wieder geöffnete (oder jemandem gesendete) Datei hat keinen Verlauf mehr, ebenso wenig die gelöschten Spalten oder Zeilen.

Eine aus einem freigegebenen Ordner (Netzwerk, OneDrive…) geöffnete Arbeitsmappe wird anderen Computern durch eine kleine versteckte Datei `~$name.lattis` daneben gemeldet: Ein anderer Computer, der sie öffnet, wird gewarnt und kann sie schreibgeschützt öffnen. Wurde die Datei seit dem Öffnen anderswo geändert, bietet `Strg+S` an, eine Kopie zu speichern, zu überschreiben oder abzubrechen.

Datei › Importieren liest CSV- und Excel-Dateien; Datei › Exportieren schreibt CSV, `.xlsx` oder OpenDocument (`.ods`) (in einer CSV für Excel wird einem Text, der mit `=`, `+`, `-` oder `@` beginnt, ein Apostroph vorangestellt, damit Excel ihn nicht als Formel ausführt); Datei › Drucken / PDF druckt oder speichert als PDF, gemäß Ansicht › Seitenlayout (siehe [Seitenlayout und Drucken](impression)).

## Ein Blatt oder die Arbeitsmappe schützen {#proteger-une-feuille-ou-le-classeur}

Überprüfen › Blatt schützen lehnt jede Änderung am Inhalt des Blatts ab; Struktur schützen erlaubt nur die Eingabe in Tabellen (Werte, hinzugefügte oder gelöschte Zeilen, Sortieren, Filtern); Arbeitsmappe schützen lehnt das Hinzufügen, Löschen oder Umbenennen von Blättern ab und kann zusätzlich die Struktur oder den Inhalt aller Blätter sperren. Ein optionales Kennwort verhindert, dass der Schutz versehentlich aufgehoben wird. Wie in Excel ist das ein Schutz vor **versehentlichen Änderungen**, keine Sicherheit: Die Datei ist nicht verschlüsselt, und wer über die passenden Werkzeuge verfügt, kann ihre Daten lesen oder den Schutz ohne Kennwort entfernen. Verlassen Sie sich bei vertraulichen Daten nicht darauf.

## Wie geht es weiter? {#et-ensuite}

- [PivotTables](tcd) und [Diagramme](graphiques) zum Auswerten.
- [Die Registerkarte SQL](sql) zum Abfragen von Tabellen.
- [Lua-Skripte](lua) zum Automatisieren.
- [Daten importieren](connexions) aus Dateien, Datenbanken oder dem Web.
