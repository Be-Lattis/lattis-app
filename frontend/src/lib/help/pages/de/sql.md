# SQL-Abfragen

Jede Tabelle der Arbeitsmappe ist eine SQLite-Tabelle. Die Registerkarte **SQL** (rechts neben den Registerkarten des Menübands) fragt sie mit den Namen ab, die Sie auf dem Bildschirm sehen.

## Eine Abfrage schreiben {#ecrire-une-requete}

```
SELECT Kunde, SUM("Betrag netto") AS Summe
FROM Bestellungen
WHERE Bezahlt = 0
GROUP BY Kunde
ORDER BY Summe DESC
```

- Ein Name mit einem Leerzeichen, einem Akzent oder einem Großbuchstaben steht in doppelten Anführungszeichen: `"Betrag netto"`.
- `Strg+Eingabe` führt die Auswahl aus, sonst die Anweisung unter dem Cursor (Anweisungen werden durch `;` oder eine Leerzeile getrennt).
- `Strg+Leertaste` schlägt Tabellen- und Spaltennamen vor; `Alt+↑` / `Alt+↓` blättern durch den Verlauf.
- Das Ergebnis zeigt höchstens 1.000 Zeilen.
- Die Schaltfläche **Referenz** (oder `F1` auf einem Schlüsselwort im Editor) zeigt gängige Syntax, nützliche Funktionen und die Regeln von Lattis; jeder Eintrag hat ein Beispiel zum Einfügen, mit den Namen Ihrer Tabellen.

[PivotTables](tcd) werden wie Tabellen abgefragt. `Cell_A1` verweist auf eine freie Zelle des angezeigten Blatts.

## Der linke Bereich {#le-volet-de-gauche}

Drei Listen: die **Objekte** der Arbeitsmappe (Tabellen, PivotTables und ihre Spalten; ein Klick springt zur Tabelle auf ihrem Blatt), die **Favoriten** (in der Arbeitsmappe gespeicherte Abfragen) und der **Verlauf**.

Wenn Sie an einem Lattis-Server angemeldet sind, zeigen die Objekte in einer eigenen Gruppe auch die **Datenbestände des Lattis Data Lake**, die Sie lesen dürfen: Ansichten `donnees.name` (Doppelklick: Beispielabfrage). Eine Abfrage, die auf sie verweist, läuft auf dem Server, schreibgeschützt und auf die Zeilen Ihrer Zielgruppe beschränkt; sie wird im SQL von PostgreSQL geschrieben und kann nicht auf die Tabellen der Arbeitsmappe verweisen (um beide zu kombinieren, fügen Sie den Datenbestand in die Arbeitsmappe ein: Einfügen › Lattis Data Lake).

## Das Ergebnis auf ein Blatt senden {#envoyer-le-resultat-sur-une-feuille}

Das Ergebnis einer Abfrage wird in eine neue Tabelle kopiert oder von ihr **referenziert**: Die Tabelle bleibt dann mit der Abfrage verknüpft und wird mit Rechtsklick › Tabelle › Abfrage aktualisieren (oder `F5`) neu gelesen. Sie behält die Überschriften des Ergebnisses; technische Spalten (`_pos`) kommen nie hinein.

### Neben ein verknüpftes Ergebnis schreiben {#ecrire-a-cote-d-un-resultat-lie}

Die von der Abfrage geladenen Spalten (blaue Überschrift oder die Farbe externer Quellen des unter Ansicht › Design gewählten Designs) können nicht bearbeitet werden. Um eigene Informationen hinzuzufügen, geben Sie direkt rechts neben der Überschrift einen Namen ein: Die neue **erfasste Spalte** (grüne Überschrift) wird von Hand oder durch eine Formel gefüllt, und die Formeln der Arbeitsmappe sehen sie wie die anderen (`Abfrage1[Kommentar]`).

Bei jeder Aktualisierung folgen ihre Werte ihren Zeilen über einen **Schlüssel**: die Spalte `id` des Ergebnisses, wenn es eine gibt (wie bei `SELECT *`), sonst eine Spalte ohne Duplikate oder leere Zellen, nach der Lattis beim ersten eingegebenen Wert fragt. Passt keine einzelne Spalte, schlägt Lattis Paare vor: Bei `SELECT *, 2021 AS Jahr FROM Erfassung2021 UNION SELECT *, 2022 FROM Erfassung2022` kommen dieselben `id`-Werte von einem Jahr zum nächsten wieder, und es ist „id + Jahr“, das die Zeilen identifiziert. Der Zeilenrang ist möglich, aber nicht empfehlenswert: Ändert die Abfrage die Reihenfolge ihrer Zeilen, folgen die Werte nicht mehr.

- Eine Zeile, die aus dem Ergebnis verschwindet: Ihre Werte werden **beiseitegelegt** und kommen zurück, wenn ihr Schlüssel zurückkommt. Rechtsklick › Tabelle › Beiseitegelegte Werte vergessen löscht sie.
- Ein doppelter Schlüssel im Ergebnis: Die Werte für diesen Schlüssel werden beiseitegelegt, mit einer Warnung.
- Das Umbenennen oder Löschen einer erfassten Spalte erfolgt wie in einer gewöhnlichen Tabelle; der Schlüssel wird mit Rechtsklick › Tabelle › Schlüssel der erfassten Spalten… geändert.
- Alles wird mit `Strg+Z` rückgängig gemacht, Aktualisierung inbegriffen.

Dasselbe gilt für eine von einer Abfrage aus dem Editor geladene Tabelle ([Daten importieren](connexions)).

## Die Daten ändern {#modifier-les-donnees}

Abfragen mit `INSERT`, `UPDATE` und `DELETE` ändern Tabellen wie eine Eingabe; sie werden mit `Strg+Z` rückgängig gemacht. Während eine Abfrage läuft, unterbricht die Schaltfläche **Stopp** sie; ein Lesevorgang stoppt nach 30 s von selbst, ein Schreibvorgang nach 2 min (er blockiert Eingaben, solange er dauert). Ein gestoppter Schreibvorgang ändert nichts.

## Eine Abfrage in einer Formel {#une-requete-dans-une-formule}

Die Funktion [ABFRAGE](fonction:ABFRAGE) führt ein schreibgeschütztes `SELECT` aus und lässt sein Ergebnis auf das Blatt überlaufen:

```
=ABFRAGE("SELECT Kunde, SUM(Betrag) FROM Umsatz GROUP BY Kunde")
```

## In Skripten {#dans-les-scripts}

`db.query`, `db.rows` und `db.execute` verwenden dasselbe SQL: siehe [Lua-Skripte](lua).
