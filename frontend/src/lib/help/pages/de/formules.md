# Formeln

Eine Formel beginnt mit `=`. Funktionen werden mit ihren deutschen Namen geschrieben (`SUMME`, `WENN`, `ZÄHLENWENN`…, ebenso `WAHR`, `FALSCH` und die Fehlerwerte `#WERT!`, `#NV`…); die englischen Namen (`SUM`, `IF`…) werden ebenfalls akzeptiert. Argumente werden durch `;` getrennt, und Dezimalstellen werden mit einem Komma geschrieben: `=RUNDEN([Preis]*1,2; 2)`.

Während der Eingabe schlägt Lattis Funktionen, Tabellen und Spalten vor, zeigt die Syntax der aktuellen Funktion (das einzugebende Argument fett) und färbt die Bezüge ein. `F1` öffnet hier die Seite der Funktion unter dem Cursor; Formeln › Funktion einfügen öffnet die vollständige Liste.

## Bezüge {#references}

| Schreibweise | Verweist auf |
| --- | --- |
| `[Spalte]` | In einer Tabelle: den Wert der Spalte in der Zeile der Formel |
| `[Spalte][2]` | Den Wert der Spalte in der 2. Zeile derselben Tabelle |
| `Tabelle[Spalte]` | Die ganze Spalte (in einer Aggregatfunktion oder außerhalb der Tabelle); allein in einer freien Zelle (`=Umsatz[Preis]`) laufen ihre Werte nach unten über |
| `Tabelle[Spalte][2]` | Einen Wert, über den Rang seiner Zeile (1 = erste Datenzeile) |
| `Tabelle[Spalte][1:3]` | Die Zeilen 1 bis 3 einer Spalte |
| `Cell[A1]` | Die freie Zelle A1 des Blatts |
| `Tabelle2!Cell[A1]` | Die freie Zelle A1 eines anderen Blatts (`'Mein Blatt'!Cell[A1]`, wenn der Name Leerzeichen enthält) |
| `Cell[C14:C16]` | Einen Bereich freier Zellen |

Die Ränge folgen der Anzeigereihenfolge: Das Sortieren einer Tabelle ändert die referenzierte Zeile, wie in einer Tabellenkalkulation. Um eine Zeile über ihren Schlüssel nachzuschlagen, verwenden Sie [REFERENZ](fonction:REFERENZ).

Im Formelmodus fügt ein Klick auf eine Zelle ihren Bezug ein; Ziehen fügt einen Bereich ein. Von einer freien Zelle aus fügt ein Klick auf eine Zelle einer Tabelle genau diesen Wert ein (`Bericht[TEST][2]`, über den Rang seiner Zeile); ein Klick auf die Überschrift oder Ziehen über die ganze Spalte fügt die gesamte Spalte ein (`Bericht[TEST]`).

## Zelladressen (C14, A1:B5) {#adresses-a-la-excel}

Eine aus Gewohnheit eingegebene Adresse (`=C14*2`, `=SUMME(C14:C16)`, `$C$14`, `Tabelle2!B3`) wird beim Bestätigen umgewandelt: in `Cell[C14]` für eine freie Zelle, in `Tabelle[Spalte][k]` in einer Tabelle, in einen Bereich für `C14:C16`.

[SVERWEIS](fonction:SVERWEIS), [WVERWEIS](fonction:WVERWEIS) und [XVERWEIS](fonction:XVERWEIS) funktionieren auch mit freien Zellen (`=SVERWEIS(A2; D2:F20; 2; FALSCH)`), mit genauer oder ungefährer Übereinstimmung: Sie werden zu [INDEX](fonction:INDEX) und [VERGLEICH](fonction:VERGLEICH) oder [XVERGLEICH](fonction:XVERGLEICH).

## Berechnete Spalten {#colonnes-calculees}

Eine in eine Tabellenspalte eingegebene Formel gilt für alle ihre Zeilen: Die Spalte wird **berechnet**. `[Spalte]` verweist dort auf den Wert in derselben Zeile. Aggregatfunktionen akzeptieren einen Ausdruck über die Spalten einer Tabelle, Zeile für Zeile berechnet: `=SUMME(Umsatz[Preis]*Umsatz[Menge])`.

### In Werte umwandeln {#convertir-en-valeurs}

Rechtsklick auf eine berechnete Spalte › **In Werte umwandeln** (oder Formeln › In Werte umwandeln): Die Formel wird entfernt, und die Spalte wird zu einer gewöhnlichen Spalte, die ihre aktuellen Werte behält, mit dem Typ dieser Werte (Ganzzahl, Dezimalzahl, Datum, Wahrheitswert, sonst Text; ein Fehler wie `#DIV/0!` bleibt gekennzeichnet). Sie behält ihren Namen: Formeln, die auf sie verweisen, lesen sie weiter, aber sie folgt nicht mehr den Spalten, die ihre Formel las. Mehrere ausgewählte berechnete Spalten werden zusammen umgewandelt; auf freien Zellen ersetzt der Befehl ihre Formeln durch ihre Werte (ein übergelaufenes Ergebnis behält seine Formel). Strg+Z holt die Formeln zurück. Abgelehnt unter geschützter Struktur und vorerst in einer geteilten Tabelle einer kollaborativen Arbeitsmappe.

## Überlaufende Ergebnisse {#resultats-qui-se-deversent}

Manche Funktionen geben mehrere Werte zurück ([FILTER](fonction:FILTER), [SORTIEREN](fonction:SORTIEREN), [EINDEUTIG](fonction:EINDEUTIG), [SEQUENZ](fonction:SEQUENZ), [TEXTTEILEN](fonction:TEXTTEILEN), [ABFRAGE](fonction:ABFRAGE)…): Das Ergebnis läuft in die benachbarten Zellen über, wie die dynamischen Arrays einer Tabellenkalkulation. Eine ganze Spalte allein in einer freien Zelle (`=Umsatz[Preis]`) läuft ebenso über, in der Zeilenreihenfolge der Tabelle. Ist der Platz belegt, zeigt die Formel `#ÜBERLAUF!`. In einer Aggregatfunktion liefern sie ihre Werte: `=ANZAHL2(TEXTTEILEN(Cell[A1]; ","))` zählt die Teile eines Texts; Funktionen, die eine Spalte durchlaufen (ZÄHLENWENN, KGRÖSSTE, VERGLEICH…), lesen sie nicht (`#WERT!`). EINDEUTIG, SORTIEREN, ÜBERNEHMEN und WEGLASSEN lesen eine Spalte (`=EINDEUTIG(Nestbeobachtung[Nest-ID])`), eine ganze Tabelle, einen Zellbereich (`Cell[A1:A20]`) oder ein anderes dynamisches Array; FILTER und SORTIERENNACH eine Tabelle oder eine Spalte.

`=TEXTTEILEN("Name=Müller;Ort=Paris"; "="; ";")` teilt einen Text in Zeilen (am `;`) und dann in Spalten (am `=`); kürzere Zeilen werden mit `#NV` oder mit dem letzten Argument aufgefüllt. Um überzulaufen, darf der Text nicht von einer Tabellenzeile abhängen: eine Zelle oder ein Wert wie `Umsatz[Notizen][2]`.

Eine **Matrixkonstante** wird in geschweifte Klammern geschrieben: `{1;2;3}` (eine Spalte), `{"a"."b"}` (eine Zeile), `{1.2;3.4}`. Sie läuft wie ein dynamisches Array über und dient insbesondere dazu, mehrere Trennzeichen anzugeben: `=TEXTTEILEN(Cell[A1]; {",";" "})`.

### Ein übergelaufenes Ergebnis in eine Tabelle umwandeln {#convertir-un-resultat-deverse-en-tableau}

Der Überlaufbereich ist blau umrahmt; seine QuickInfo nennt die Zelle der Formel. **Rechtsklick** auf diesen Bereich:

- **In verknüpfte Tabelle umwandeln**: Eine echte Tabelle ersetzt das Ergebnis. Sie bleibt mit der Formel verknüpft (in einer Abfrage der Arbeitsmappe gespeichert, Quelle „Formel“, sichtbar unter Daten › Abfragen) und **berechnet sich von selbst neu**, wenn sich ihre Quellen ändern: Eine Eingabe in `Nestbeobachtung` aktualisiert sofort die aus `=EINDEUTIG(Nestbeobachtung[Nest-ID])` gewonnene Tabelle, und ein einziges Strg+Z macht sowohl die Eingabe als auch die Neuberechnung rückgängig. Ihre berechneten Spalten sind schreibgeschützt (blau, die Farbe des Designs für externe Quellen); ein rechts neben ihrer Überschrift eingegebener Name fügt ihr eine **erfasste Spalte** hinzu (grün, die Farbe der Tabellen), deren Werte ihrer Zeile folgen, wenn sich das Ergebnis ändert (über die Spalte selbst bei einem `EINDEUTIG` einer Spalte, sonst über den angeforderten Schlüssel), wie neben einer Abfrage. F5 aktualisiert sie ebenfalls. Fügen Sie ihrer Abfrage Schritte hinzu (Daten › Abfragen), wird sie danach nur noch mit F5 aktualisiert.
- **In Tabelle umwandeln (Werte)**: Eine gewöhnliche Tabelle behält die aktuellen Werte, ohne Verknüpfung.

In beiden Fällen sind **die Überschriften** die Texte direkt über der Formel, wenn über jeder Spalte einer steht (ein Titel „Nester“ über `=EINDEUTIG(…)` wird zur Überschrift der Tabelle namens Nester); andernfalls die Namen der Spalten der Formel („Nest-ID“, „Wert“ bei SEQUENZ), in der leeren Zeile über der Formel (die Werte bewegen sich nicht) oder, falls keine vorhanden ist, in der Zeile der Formel (die Werte rücken eine Zeile nach unten). Ein einziger Rückgängig-Schritt holt die Formel zurück.

## Als Text geschriebene Zahlen {#nombres-ecrits-en-texte}

In einer Textspalte lesen Funktionen, die Zahlen erwarten ([SUMME](fonction:SUMME), [MITTELWERT](fonction:MITTELWERT), [MIN](fonction:MIN), [MAX](fonction:MAX), [SUMMEWENN](fonction:SUMMEWENN), [SUMMENPRODUKT](fonction:SUMMENPRODUKT), Statistik…), jeden Text als die Zahl, die er darstellt: „12“, „1.234,5“, „12 €“, „15 %“ (0,15), „(12)“ (−12) sowie fremde Schreibweisen „1,234.56“ oder „1'234.5“. **Ein Text, der keine Zahl ist, zählt als 0**: Der Mittelwert von „12“ und „abc“ ist 6. Leere Zellen werden weiterhin ignoriert. [ANZAHL](fonction:ANZAHL) zählt nur die Texte, die Zahlen sind: `=ANZAHL2(Umsatz[Punkte]) - ANZAHL(Umsatz[Punkte])` ergibt die Anzahl der Texte, die keine sind.

Eine Tabellenkalkulation ignoriert diese Texte. Ein Bereich freier Zellen (`Cell[A1:A10]`) folgt dieser Regel: Seine Texte sind Beschriftungen und werden ignoriert. In einer direkten Berechnung (`=[Punkte]*2`) ergibt ein Text, der keine Zahl ist, `#WERT!`.

## Zwischennamen {#noms-intermediaires}

[LET](fonction:LET) benennt Werte, die einmal berechnet werden und in den folgenden Werten sowie in der abschließenden Berechnung verwendbar sind: `=LET(netto; [Preis]*[Menge]; steuer; netto*0,2; netto+steuer)`. Ein Name darf weder eine Zelladresse (`A1`, `Cell[A1]`) noch ein Funktionsname sein.

## Fehlerwerte {#valeurs-d-erreur}

| Fehler | Übliche Ursache |
| --- | --- |
| `#WERT!` | Argument vom falschen Typ, oder Spalten zweier Tabellen vermischt |
| `#DIV/0!` | Division durch null |
| `#NV` | Wert nicht gefunden (XVERWEIS, VERGLEICH, REFERENZ, GEMEINDE…) |
| `#NAME?` | Unbekannte Funktion, Tabelle oder Spalte |
| `#BEZUG!` | Gelöschte Zelle oder gelöschter Bereich |
| `#CIRC!` | Zirkelbezug: Die Formel hängt von sich selbst ab |
| `#ZAHL!` | Zahl außerhalb des gültigen Bereichs (Quadratwurzel einer negativen Zahl…) |

## Statistik und Finanzen {#statistiques-et-finances}

Statistikfunktionen lesen eine ganze Spalte, einen Bereich oder einen Ausdruck über die Spalten einer Tabelle; Wahrheitswerte und leere Zellen werden ignoriert, Texte werden wie oben beschrieben gelesen: [STABW.S](fonction:STABW.S), [VARIANZ](fonction:VARIANZ), [MODALWERT](fonction:MODALWERT), [QUANTIL](fonction:QUANTIL), [QUARTILE](fonction:QUARTILE), [KORREL](fonction:KORREL), [STEIGUNG](fonction:STEIGUNG)…: `=QUARTILE(Umsatz[Betrag]; 3)`.

Finanzfunktionen folgen den üblichen Konventionen von Tabellenkalkulationen: Ein ausgezahlter Betrag ist negativ, und der Zinssatz ist der Satz pro Periode. Die Monatsrate eines Darlehens von 200.000 € über 20 Jahre zu 4 % pro Jahr schreibt sich `=RMZ(4%/12; 20*12; 200000)` (−1.211,96). Siehe auch [BW](fonction:BW), [ZW](fonction:ZW), [ZZR](fonction:ZZR), [ZINS](fonction:ZINS), [ZINSZ](fonction:ZINSZ) und [KAPZ](fonction:KAPZ) für einen Tilgungsplan. [NBW](fonction:NBW) und [IKV](fonction:IKV) lesen die Zahlungsströme einer Spalte **in der Anzeigereihenfolge der Tabelle**: `=IKV(Zahlungen[Betrag])`.

Auch verfügbar: Wahrscheinlichkeitsverteilungen ([NORM.VERT](fonction:NORM.VERT), [T.VERT](fonction:T.VERT)…), Tests ([T.TEST](fonction:T.TEST)), Regression ([PROGNOSE.LINEAR](fonction:PROGNOSE.LINEAR), [RGP](fonction:RGP)), Abschreibungen ([LIA](fonction:LIA), [AMORDEGRK](fonction:AMORDEGRK)) und Wertpapiere ([KURS](fonction:KURS)). [DBSUMME](fonction:DBSUMME) und die anderen Datenbankfunktionen lesen ihre Kriterien aus einem Zellbereich: `=DBSUMME(Verkäufe; "Betrag"; Cell[E1:F3])`. [BEREICH.VERSCHIEBEN](fonction:BEREICH.VERSCHIEBEN) und [INDIREKT](fonction:INDIREKT) berechnen einen Bezug: `=SUMME(BEREICH.VERSCHIEBEN(Cell[B2]; 0; 0; Cell[A1]))`.

## Gemeinde eines GPS-Punkts {#commune-d-un-point-gps}

[GEMEINDE](fonction:GEMEINDE) liefert die französische Gemeinde (Commune), die einen Punkt enthält: `=GEMEINDE([Breitengrad]; [Längengrad])`, oder ihren INSEE-Code (`"code"`), ihre Postleitzahlen (`"cp"`), ihr Département (`"departement"`) oder ihre Region (`"region"`). Die Berechnung erfolgt offline, auf den in Lattis eingebetteten Gemeindegrenzen (französisches Festland und Überseegebiete), und bleibt auch auf einer Spalte mit 100.000 Zeilen schnell. Ein Punkt auf dem Meer oder außerhalb Frankreichs ergibt `#NV`. Da die Grenzen vereinfacht sind, kann ein Punkt wenige Dutzend Meter von einer Grenze entfernt in der Nachbargemeinde landen.

*Enthält Daten von IGN und INSEE (Gemeindegrenzen ADMIN EXPRESS, Ausgabe 2026, verbreitet von Etalab) und Postleitzahlen von La Poste über geo.api.gouv.fr, unter der Licence Ouverte / Etalab 2.0.*

## Verknüpfungen zwischen Formeln {#liens-entre-formules}

**Formeln › Verknüpfungen anzeigen** zeichnet die Verknüpfungen der ausgewählten Zellen: blaue Pfeile von dem, was ihre Formel liest, rote Pfeile zu den Formeln, die sie lesen. Auf eine ganze Spalte (`Kunden[Ort]`) wird über ihre Überschrift gezeigt; auf eine bestimmte Zeile (`Kunden[Ort][1]`, `[Saldo][-1]`, die von `REFERENZ` gefundene Zeile) über ihre Zelle, und ein Zeilenbereich (`Umsatz[Betrag][2:5]`) wird umrahmt. Ist diese Zeile ausgeblendet (Filter, Auswahlblatt, ausgeblendete Zeile) oder nicht auffindbar, zeigt der Pfeil auf die Spalte, und die Statusleiste nennt den Grund. Verknüpfungen zu einem anderen Blatt werden dort gezählt. Ein Klick in das Raster (oder `Esc`) entfernt die Pfeile.

## Was-wäre-wenn-Analyse {#analyse-de-scenarios}

**Daten › Was-wäre-wenn-Analyse**: **Zielwertsuche** findet den Wert einer Zelle, der einer Formel das gewünschte Ergebnis gibt; der **Szenario-Manager** speichert benannte Wertesätze, zeigt sie an und fasst sie auf einem neuen Blatt zusammen; die **Datentabelle** berechnet eine Formel für eine Liste von Werten (eine oder zwei Variablen, Ergebnisse als feste Werte: zum Neuberechnen erneut ausführen); der **Solver** maximiert, minimiert oder erreicht einen Wert unter Nebenbedingungen (exakt bei einem linearen Problem, sonst angenähert). Vor **Übernehmen** ändert sich nichts; `Strg+Z` macht rückgängig. Unter Datei › Einstellungen › Funktionen einschalten.

## Neuberechnung {#recalcul}

Formeln berechnen sich von selbst neu, wenn sich ihre Daten ändern. `F9` berechnet alles neu; `HEUTE`, `JETZT` und `ZUFALLSZAHL` ändern sich nur beim Öffnen und mit `F9`.

## Alle Funktionen {#toutes-les-fonctions}
