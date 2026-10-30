# Erste Schritte

Lattis funktioniert wie eine Tabellenkalkulation: ein endloses Raster, oben ein Menüband, unten die Blattregister. Der Unterschied liegt darunter: Jede Tabelle, die Sie eingeben, wird zu einer echten Datenbanktabelle, mit Datentypen und per SQL abfragbar.

## Geführte Tour {#visite-guidee}

In drei Minuten zeigt die geführte Tour das Wichtigste in einer kleinen neuen Arbeitsmappe: eine eingetippte Tabelle, eine berechnete Spalte, zwei mit REFERENZ verknüpfte Tabellen, eine Pivot-Tabelle mit Diagramm, dann eine SQL-Abfrage. Führen Sie jeden Schritt selbst aus oder klicken Sie auf Weiter, damit Lattis ihn erledigt; Überspringen (oder `Esc`) beendet sie. Sie wird beim ersten Start angeboten und lässt sich über die Hilfe neu starten (? › Geführte Tour).

## Erkannte Tabellen {#tableaux-detectes}

Geben Sie irgendwo eine Überschriftenzeile und dann einige Zeilen mit Werten ein: Lattis macht daraus eine **Tabelle**, mit einem Typ pro Spalte und einem Namen (der des Registers für die erste, links neben der Bearbeitungsleiste änderbar). Eine direkt daneben eingegebene Zeile oder Spalte erweitert sie; ein eingefügter Block wird ebenfalls zur Tabelle. Enthält die erste Zeile Zahlen oder Datumsangaben, sind es Daten: Die Überschrift „Spalte 1“, „Spalte 2“… kommt darüber und kann umbenannt werden. `Strg+L` formatiert einen Bereich als Tabelle.

Was nicht in einer Tabelle steht, bleibt eine **freie Zelle**: ein Titel, ein Parameter, eine Summe. Alles lässt sich mit `Strg+Z` rückgängig machen, auch das Anlegen einer Tabelle. Erkennungsregeln, nachträglich eingegebene Überschriften, Kopfzeilengruppen auf zwei Zeilen, Kontextmenü: siehe [Tabellen](tableaux).

## Spaltentypen {#types-de-colonnes}

Jede Spalte hat einen Typ, der aus Ihrer Eingabe erraten wird: Text, Ganzzahl, Dezimalzahl, Datum (mit oder ohne Uhrzeit), Wahrheitswert oder Berechnet (eine Formel für die ganze Spalte). Der Typ wird im Menü der Spaltenüberschrift geändert; siehe [Spaltentypen](tableaux#types-de-colonnes).

## Sortieren und filtern {#trier-et-filtrer}

Das ▾ einer Überschrift sortiert die Tabelle (A bis Z, Z bis A, benutzerdefinierte Reihenfolge) und filtert die Spalte nach Werten oder nach Bedingung. Das Sortieren ändert die Reihenfolge der Zeilen dauerhaft; leere Zellen kommen ans Ende. Siehe [Sortieren und filtern](tableaux#trier-et-filtrer).

## Schlüssel {#cles}

Ein **Schlüssel** ist eine Spalte mit eindeutigen, überall ausgefüllten Werten (eine Kundennummer): Er identifiziert jede Zeile, wird am Rand angezeigt und dient den Nachschlagen mit [REFERENZ](fonction:REFERENZ). Eine erste Spalte namens ID oder mit 1, 2, 3… wird automatisch zum Schlüssel, und eine hinzugefügte Zeile erhält die nächste Nummer. Siehe [Schlüssel](tableaux#cles).

## Formeln {#formules}

Eine Formel beginnt mit `=` und verweist auf Spalten über ihren Namen. In eine Tabellenspalte eingegeben, gilt sie für die ganze Spalte:

```
=[Menge]*[Stückpreis]
```

Siehe [Formeln](formules) für Bezüge und die Liste der Funktionen. Während der Eingabe öffnet `F1` die Seite der Funktion unter dem Cursor.

## Blätter zusammenführen {#regrouper-des-onglets}

Ein Blatt pro Monat, Jahr oder Stadt, mit denselben Spalten? Daten › Blätter zusammenführen… vereint sie in einer einzigen Tabelle mit einer Kategoriespalte und passt die Formeln an, die auf sie verwiesen. Siehe [Blätter zusammenführen](regroupement).

## Auswahlblatt (ein Jahr, ein Monat, eine Stadt nach der anderen) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

Rechtsklick auf ein Register › **Auswählen nach** › eine Spalte (Jahr): Das Blatt zeigt jeweils einen Wert, gewählt in seiner ▾-Liste, wie ein Blatt pro Jahr, mit **Neues Jahr…** zum Hinzufügen. Siehe [Auswahlblatt](regroupement#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois).

## Speichern {#enregistrer}

Eine Arbeitsmappe ist eine einzige `.lattis`-Datei. `Strg+S` speichert; nach einem Absturz werden die Änderungen beim nächsten Start zur Wiederherstellung angeboten. `Strg+Z` funktioniert bis zum Schließen, auch nach `Strg+S`, aber der Verlauf wird nicht in der Datei gespeichert. Datei › Importieren und Exportieren lesen und schreiben CSV, `.xlsx` und `.ods`. Siehe [Dateien und Wiederherstellung](fichiers) und [Seitenlayout und Drucken](impression).

## Ein Blatt oder die Arbeitsmappe schützen {#proteger-une-feuille-ou-le-classeur}

Überprüfen › Blatt schützen, Struktur schützen (nur Eingabe in Tabellen) und Arbeitsmappe schützen verhindern **versehentliche Änderungen**, mit optionalem Kennwort (siehe [Schützen](cellules#proteger)). Das ist keine Sicherheit: Die Datei ist nicht verschlüsselt, und wer über die passenden Werkzeuge verfügt, kann ihre Daten lesen oder den Schutz ohne Kennwort entfernen. Verlassen Sie sich bei vertraulichen Daten nicht darauf.

## Wie geht es weiter? {#et-ensuite}

- [PivotTables](tcd) und [Diagramme](graphiques) zum Auswerten.
- [Die Registerkarte SQL](sql) zum Abfragen von Tabellen.
- [Lua-Skripte](lua) zum Automatisieren.
- [Daten importieren](connexions) aus Dateien, Datenbanken oder dem Web.
