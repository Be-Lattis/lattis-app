# Tabellen

Eine Tabelle ist ein eingegebener, eingefügter oder importierter Datenblock: Lattis erkennt ihn, typisiert seine Spalten und macht daraus eine per SQL abfragbare Tabelle.

## Erkannte Tabellen {#tableaux-detectes}

Geben Sie irgendwo auf dem Blatt ein paar Zeilen ein: eine Überschriftenzeile, dann Werte. Lattis erkennt den Datenblock und macht daraus eine **Tabelle**: Die Überschrift wird hervorgehoben, jede Spalte erhält einen Typ, und die Tabelle erhält einen Namen, den Sie im Tabellennamenfeld links neben der Bearbeitungsleiste ändern können (neuen Namen eingeben, dann Eingabe). Die erste Tabelle eines Blatts übernimmt den Namen des Registers (ohne Leerzeichen: „Umsatz 2026“ ergibt `Umsatz2026`), die folgenden heißen Tabelle1, Tabelle2… Wird das Register umbenannt, wird auch die Tabelle umbenannt, die seinen Namen trägt (nicht umgekehrt); diese Tabelle zeigt kein Namensschild, da das Register ihn bereits anzeigt.

- Es braucht mindestens zwei Spalten über zwei Zeilen (drei gefüllte Zellen eines 2 × 2-Quadrats): Eine einspaltige Liste oder eine einzelne Zeile bleibt aus freien Zellen.
- Die erste eingegebene Zeile ist die Überschrift, auch wenn sie wie die folgenden aussieht („Name | Ort“, dann „Lea | Paris“), außer sie enthält einen Wert, der kein Text ist: eine Zahl, ein Datum, eine Uhrzeit, einen Wahrheitswert, einen Prozentsatz, einen Betrag („Lea | 31“, „Land | 2024“, „10 | 20“). Dann sind alle eingegebenen Zeilen Daten, und die Überschrift kommt direkt darüber, mit den Namen „Spalte 1“, „Spalte 2“… in grauer Kursivschrift: Geben Sie dort den Namen jeder Spalte ein, oder Rechtsklick auf die 1. Zeile › „Erste Zeile als Überschriften verwenden“ (auch unter Rechtsklick › Tabelle): Sie wird an ihrer Stelle zur Überschrift, die Zeile mit „Spalte n“ verschwindet, Formeln werden angepasst, und eine Spalte wird wieder zu Zahlen, wenn ihr einziger Text der Name war. Soll eine Zahl ein Spaltenname sein, geben Sie sie nach einem Apostroph ein ('2024). Ist darüber kein freier Platz (erste Zeile des Blatts, eine Zelle, ein Titel oder eine Tabelle direkt darüber), bleiben die Zellen frei: Start › Als Tabelle formatieren (`Strg+L`) macht daraus eine Tabelle, deren erste Zeile die Überschriften liefert.
- Eine direkt unter einer Tabelle eingegebene Zeile erweitert sie, ebenso eine direkt rechts daneben eingegebene Spalte.
- Ein aus einem anderen Programm eingefügter Block wird zu einer Tabelle, mit derselben Überschriftenregel wie bei der Eingabe: Seine 1. Zeile ist die Überschrift, wenn sie nur Text enthält; eine Zahl, ein Datum… („Land | 2024“, „10 | 20“) macht sie zu einer Datenzeile, unter „Spalte 1“, „Spalte 2“… (Rechtsklick › „Erste Zeile als Überschriften verwenden“, um sie zurückzuholen). In der ersten Zeile des Blatts eingefügt, ohne Platz darüber, nimmt die Überschrift „Spalte n“ den Platz der 1. eingefügten Zeile ein, und der Block rückt eine Zeile nach unten.
- Nach den Daten eingegebene Überschriften: Geben Sie die Namen in die Zeile direkt über der Tabelle ein. Sobald zwei Namen über einer Überschrift stehen, die wie Daten aussieht („Lea | 31“, „Spalte 1“), bietet die Statusleiste „Als Überschriften verwenden“ an; andernfalls Rechtsklick auf eine dieser Zellen › „Als Überschriften von „Tabelle1“ verwenden“ (oder Rechtsklick auf die Überschrift › Tabelle › „Zeile darüber als Überschriften verwenden“). Die Namen ersetzen die Spaltennamen (eine leer gelassene Zelle behält den bisherigen), die alte Überschrift wird zur 1. Datenzeile, als Zahlen, wenn es Zahlen waren, und Formeln werden angepasst; `Strg+Z` macht es rückgängig. Ein einzelnes darüber eingegebenes Wort bleibt ein Titel.
- Ziehen Sie eine Tabelle neben eine andere, um sie zusammenzuführen.
- `Entf` auf Zellen einer Tabelle: Eine Zeile, in der nichts Eingegebenes mehr steht, wird gelöscht, als wäre sie nie eingegeben worden (eine berechnete Spalte, die ausgeblendete Spalte eines Auswahlblatts oder die automatisch vergebene Nummer eines ganzzahligen Schlüssels halten sie nicht; die Statusleiste meldet „N leere Zeilen gelöscht“, `Strg+Z` holt sie zurück). Eine Zeile, die schon leer war und in der `Entf` nichts gelöscht hat, bleibt bestehen.
- Was nicht in einer Tabelle steht, bleibt eine **freie Zelle**: ein Titel, ein Parameter, eine Summe.

Alles lässt sich mit `Strg+Z` rückgängig machen, auch das Anlegen einer Tabelle.

Ein **Rechtsklick** in einer Tabelle bietet zuerst an, was nur dort gilt (Wechseln zu, Erste Zeile als Überschriften verwenden, Diese Zeile aus der Tabelle nehmen auf der letzten Zeile, Zeile darüber als Überschriften verwenden auf einer Überschrift…), dann Ausschneiden, Kopieren, Einfügen; Inhalte löschen, Zellen formatieren… (`Strg+1`), Bedingte Formatierung…; die Notiz. Zuletzt folgen die Untermenüs jedes Objekts, mit seinem Namen betitelt: **Sortieren und Filtern**, **Zeile „12“** (ihr Schlüssel, wenn die Tabelle einen hat: oberhalb oder unterhalb einfügen, Zeile löschen), **Spalte „Name“** (umbenennen, Typ und Schlüssel, Überprüfung, Register filtern, in Werte konvertieren, links oder rechts einfügen, Spalte löschen) und **Tabelle „Messungen“** (Überschriften, Freigabe, In Bereich konvertieren, Tabelle löschen). Zeilennummern, Spaltenbuchstaben, PivotTables (**PivotTable „Umsatz“**), Diagramme und Schaltflächen folgen derselben Reihenfolge.

### Kopfzeilengruppen (Zeile darüber) {#groupes-d-en-tetes-ligne-du-dessus}

Steht über den Überschriften eine Zeile mit Gruppen (WALDSTÜCK über Name, Status und Parzelle), Rechtsklick auf die Überschrift › Tabelle › „Überschriften auf 2 Zeilen (Gruppen darüber)“: Diese Zellen, verbunden oder nicht, werden zu den Gruppen der Tabelle und über den Überschriften angezeigt. Eine Spalte heißt dann „Gruppe.Name“: `Bestand[WALDSTÜCK.Name]` in einer Formel, `"WALDSTÜCK.Name"` in SQL. Geben Sie im Gruppenband etwas ein, um eine Gruppe zu benennen oder umzubenennen (Formeln werden angepasst), löschen Sie den Inhalt, um sie zu entfernen. „Kopfzeilengruppen entfernen“ setzt die vollständigen Namen wieder in eine einzige Zeile.

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

Der Text „null“, eingegeben, eingefügt oder importiert (CSV, .xlsx), lässt die Zelle leer; geben Sie 'null ein, um das Wort zu behalten. In einer Tabelle zeigt eine leere Zelle einen grauen Strich „–“ (nichts wird kopiert oder exportiert); eine leere freie Zelle bleibt weiß.

## Sortieren und filtern {#trier-et-filtrer}

Das ▾ einer Spaltenüberschrift (oder Rechtsklick, oder Daten › Sortieren und Filtern) sortiert die Tabelle nach dieser Spalte: von A bis Z, von Z bis A oder in einer **benutzerdefinierten Reihenfolge**. Das Sortieren ändert die Reihenfolge der Zeilen dauerhaft; leere Zellen kommen immer ans Ende. Dasselbe Menü filtert die Spalte nach Werten („(Leere)“ am Ende der Liste) oder nach Bedingung.

### Benutzerdefiniertes Sortieren {#tri-personnalise}

**Benutzerdefiniertes Sortieren…** öffnet die Liste der Werte der Spalte, die Sie in die gewünschte Reihenfolge bringen: ziehen Sie sie, verwenden Sie die Schaltflächen Nach oben, Nach unten, Ganz nach oben, Ganz nach unten oder die Tastatur `Alt+↑` / `Alt+↓` (`Alt+Pos1` / `Alt+Ende`). Wählen Sie **In dieser Reihenfolge** oder **Umgekehrt**, dann OK. Werte werden ohne Berücksichtigung der Groß-/Kleinschreibung verglichen; Werte, die nicht in der Reihenfolge stehen, folgen aufsteigend, und leere Zellen bleiben am Ende.

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
