# Daten importieren und Verbindungen

Daten › Daten abrufen importiert Daten in eine Tabelle, nach Art von Power Query: Jede Transformation wird zu einem Schritt, der erneut ausgeführt werden kann, und die Tabelle wird bei Bedarf aktualisiert.

## Quellen {#sources}

| Quelle | Details |
| --- | --- |
| Text- oder CSV-Datei | Trennzeichen Ihrer Wahl |
| .xlsx-Datei | Ein Blatt Ihrer Wahl |
| JSON-Datei, Parquet-Datei | |
| SQLite-Datenbank (Datei) | Eine Tabelle oder eine Abfrage |
| PostgreSQL, MySQL / MariaDB, SQL Server | Über eine Verbindung (siehe unten) |
| Web (JSON-API) | Eine Adresse, die JSON zurückgibt |
| Tabelle der Arbeitsmappe, Andere Abfrage, SQL-Abfrage der Arbeitsmappe | Um zu transformieren, was bereits vorhanden ist |

## Der Abfrage-Editor {#l-editeur-de-requete}

Der Editor zeigt eine Vorschau der Daten und die Liste der **angewendeten Schritte**. Die Befehle seines Menübands fügen einen Schritt hinzu:

- **Spalten**: auswählen, entfernen, umbenennen, Typ ändern;
- **Zeilen**: filtern, sortieren (bei Bedarf mit einer benutzerdefinierten Reihenfolge: die aufgeführten Werte zuerst, in dieser Reihenfolge), Duplikate entfernen, die ersten Zeilen behalten;
- **Transformieren**: teilen, Werte ersetzen, formatieren (TRIM, GROSSBUCHSTABEN…), benutzerdefinierte Spalte, Indexspalte, gruppieren nach, pivotieren, entpivotieren;
- **Kombinieren**: die Zeilen einer anderen Abfrage oder einer Tabelle zusammenführen (Join) oder anfügen.

Ein Klick auf eine Spalte der Vorschau wählt sie aus; `Strg+Z` entfernt den letzten Schritt. Das Ergebnis wird in eine Tabelle geladen oder bleibt eine reine Verbindung.

## Abfragen und Verbindungen {#requetes-et-connexions}

Der Bereich **Abfragen und Verbindungen** (Daten › Abfragen und Verbindungen) listet die Abfragen der Arbeitsmappe auf: bearbeiten, aktualisieren, löschen. `F5` in einer von einer Abfrage geladenen Tabelle führt sie erneut aus; `Strg+F5` oder Daten › Alle aktualisieren führt alle Abfragen erneut aus und berechnet dann die Formeln neu. Ein rechts neben ihrer Überschrift eingegebener Name fügt eine erfasste Spalte hinzu, die beim Aktualisieren erhalten bleibt (siehe [Neben ein verknüpftes Ergebnis schreiben](sql)).

## Erhaltene Arbeitsmappe: Zustimmung zu den Quellen {#classeur-recu-accord-des-sources}

Eine Abfrage ist in der Arbeitsmappe gespeichert. Damit eine erhaltene Arbeitsmappe beim Aktualisieren keinen unbekannten Server erreichen und keine Datei von Ihrem Computer kopieren kann, bittet Lattis um Ihre Zustimmung, wenn eine Abfrage zum ersten Mal eine Datei lesen oder einen Host erreichen will (Web-API, Datenbankserver). Die Zustimmung gilt für die Sitzung oder mit „Merken“ für diese Arbeitsmappe auf diesem Computer. Die Quellen einer im Editor geschriebenen Abfrage und die Dateien, die Sie dort wählen, werden automatisch zugelassen. Die zugelassenen Hosts lassen sich unter Registerkarte Skript › Netzwerkzugriff einsehen.

## Datenbankverbindungen {#connexions-aux-bases}

Eine **Verbindung** beschreibt einen Server (PostgreSQL, MySQL / MariaDB, SQL Server oder eine Web-API): Host, Port, Datenbank, Benutzer. Sie wird in der Arbeitsmappe gespeichert und dient mehreren Abfragen. Ihr Menü **⋯** im Bereich benennt sie um: Die Abfragen, die sie verwenden, übernehmen ihren neuen Namen (`Strg+Z` macht alles rückgängig).

Kennwörter werden nie in der Arbeitsmappe gespeichert: Sie bleiben im Schlüsselbund des Systems, und Lattis fragt bei der ersten Aktualisierung danach. Im Bereich können Sie sie auch vergessen lassen.

In einem Unternehmen können Verbindungen von der Konfiguration des Computers bereitgestellt werden; sie erscheinen, ohne dass etwas eingegeben werden muss.
