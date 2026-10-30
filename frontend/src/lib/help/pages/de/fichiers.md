# Dateien und Wiederherstellung

Eine Lattis-Arbeitsmappe ist eine einzige `.lattis`-Datei: ein ZIP-Archiv mit einer komprimierten SQLite-Datenbank und einem kurzen Manifest. Sie können sie per E-Mail versenden und archivieren wie eine .xlsx-Datei; einmal mit 7-Zip oder WinRAR entpackt (der Windows-Explorer kann ihre Komprimierung Zstandard nicht lesen), kann jedes SQLite-Werkzeug ihre Tabellen lesen.

## Speichern {#enregistrer}

`Strg+S` speichert, `F12` oder `Strg+Umschalt+S` speichert unter einem anderen Namen. Das Speichern ist sicher: Die Datei wird zuerst neben die alte geschrieben, zurückgelesen und dann an ihre Stelle gesetzt; ein abruptes Herunterfahren während des Speicherns lässt die alte Datei unversehrt.

- Eine unveränderte Arbeitsmappe wird nicht neu geschrieben.
- Bei einer großen Arbeitsmappe zeigt die Statusleiste den Fortschritt.
- Ist der Datenträger zu voll, meldet Lattis es, bevor etwas geschrieben wird („… benötigt, … frei“).
- Der Rückgängig-Verlauf sowie gelöschte Spalten und Zeilen kommen nie in die Datei.

Wird die Arbeitsmappe oder das Fenster mit ungespeicherten Änderungen geschlossen, fragt Lattis zuerst, ob sie gespeichert werden sollen.

## Eine .xlsx-Datei öffnen {#ouvrir-un-fichier-xlsx}

Datei › Öffnen (oder „Öffnen mit“, oder Ziehen und Ablegen auf das Fenster) öffnet auch eine `.xlsx`-Datei: Die Titelleiste zeigt „name.xlsx“. Beim ersten `Strg+S` fragt Lattis, ob das `.xlsx`-Format behalten oder in eine `.lattis`-Arbeitsmappe umgewandelt werden soll, und nennt, was `.xlsx` nicht behält (Skripte, Pivot-Tabellen, Formeln ohne Entsprechung…). „Nicht mehr fragen“ merkt sich die Wahl (Datei › Einstellungen › Dateien). Formeln, Formate, Stile, Verbindungen, Notizen, Links, Gültigkeitsregeln, bedingte Formatierung, Diagramme und Bilder werden in die Datei zurückgeschrieben; ein normaler Bereich bleibt ein Bereich. Die Wiederherstellung nach einem Zwischenfall gilt auch für eine `.xlsx`-Datei.

## Versionsverlauf {#historique-des-versions}

Jedes Speichern bewahrt eine Version der Arbeitsmappe neben der Datei auf (versteckter Ordner `.lattis-versions`): eine pro 10 Minuten, höchstens 20 und insgesamt das Zehnfache der Dateigröße. Datei › Versionsverlauf listet sie auf (Datum, Verfasser, Größe); **Version speichern** bewahrt eine unter einem Namen auf, die bleibt. **Öffnen** zeigt eine Version schreibgeschützt; **Wiederherstellen** ersetzt die Arbeitsmappe durch sie, die vorherige Version bleibt in der Liste. Eine auf einem Lattis Server geteilte Arbeitsmappe hat keinen Verlauf.

## Wiederherstellung nach einem Zwischenfall {#recuperation-apres-un-incident}

Während Sie arbeiten, hält Lattis eine Arbeitskopie der Arbeitsmappe auf Ihrem Computer. Beendet sich Lattis, ohne gespeichert zu haben (Absturz, erzwungenes Herunterfahren, Stromausfall), bietet der nächste Start an, die Änderungen wiederherzustellen: Wiederherstellen öffnet die Arbeitsmappe in dem Zustand, in dem sie war, mit ihrem Rückgängig-Verlauf; Verwerfen kehrt zur gespeicherten Datei zurück.

Ihre Eingaben und Befehle werden sofort auf den Datenträger geschrieben: Sie überstehen sogar einen Stromausfall.

## Im Netzwerk freigegebene Arbeitsmappe {#classeur-partage-sur-le-reseau}

Eine aus einem freigegebenen Ordner geöffnete Arbeitsmappe wird anderen Computern durch eine kleine versteckte Datei `~$name.lattis` gemeldet. Ein anderer Computer, der sie öffnet, wird gewarnt und kann sie schreibgeschützt öffnen. Wurde die Datei seit dem Öffnen anderswo geändert, bietet `Strg+S` an, eine Kopie zu speichern, zu überschreiben oder abzubrechen.

Um live auf einem Lattis-Server zusammenzuarbeiten, siehe [Live-Freigabe und Data Lake](partage).

## Importieren {#importer}

Datei › Importieren liest eine CSV-Datei (oder Text mit Trennzeichen), Arbeitsmappen `.xlsx`, `.xlsm` und OpenDocument `.ods`: Jedes Tabellenblatt wird zu einem Register, jeder Datenblock zu einer Tabelle, mit Formatierung, Notizen, Auswahllisten und bedingter Formatierung. Diagramme und PivotTables werden übernommen, wenn ihre Daten eine Tabelle der Arbeitsmappe sind. Was nicht übernommen wird (Makros, andere Objekte), wird am Ende des Imports gemeldet.

Für Daten, die aktualisiert werden sollen (JSON- oder Parquet-Datei, SQLite-Datenbank, PostgreSQL, MySQL, SQL Server, Webseite), verwenden Sie stattdessen eine Abfrage: siehe [Daten importieren und Verbindungen](connexions).

## Exportieren {#exporter}

- **CSV**: die aktive Tabelle oder das ganze Blatt. In einer CSV für eine Tabellenkalkulation wird einem Text, der mit `=`, `+`, `-` oder `@` beginnt, ein Apostroph vorangestellt, damit die Tabellenkalkulation ihn nicht als Formel ausführt.
- **`.xlsx` und OpenDocument (`.ods`)**: die ganze Arbeitsmappe. Tabellen bleiben Tabellen, Formeln werden übersetzt, wenn das Format eine Entsprechung hat, sonst als ihr Wert exportiert. PivotTables gehen als Werte hinaus; Diagramme, bedingte Formatierung, Auswahllisten und Notizen gehen in `.xlsx` hinaus; Skripte werden nicht exportiert. Ein Bericht nennt, was nicht unverändert hinausging.
- **SQL (`.sql`)**: ein Skript mit `CREATE TABLE` und dann `INSERT` für die Tabellen und PivotTables (als Werte), für die ganze Arbeitsmappe, das aktive Blatt oder die aktive Tabelle. Der Dialekt Standard passt zu den meisten Datenbanken; PostgreSQL, MySQL, SQL Server und SQLite passen Namen, Typen und Wahrheitswerte an ihre Datenbank an. Die Schlüsselspalte wird zum Primärschlüssel; eine berechnete Spalte geht als Werte hinaus, ihre Formel als Kommentar; freie Zellen werden nicht exportiert.
- **PDF**: Datei › Drucken, mit dem Drucker „Microsoft Print to PDF“ (siehe [Seitenlayout und Drucken](impression)).

## Beispielarbeitsmappe {#classeur-d-exemple}

Die Startseite bietet an, eine Beispielarbeitsmappe zu öffnen: Umsätze, Kunden, PivotTables, Diagramme, eine Karte, SQL-Abfragen und Skripte, mit einem Blatt „Entdecken“, das sie vorstellt. Sie wird nie von selbst gespeichert: Datei › Speichern unter, um sie zu behalten.
