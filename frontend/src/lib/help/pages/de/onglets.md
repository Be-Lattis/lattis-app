# Dokumente und Webseiten

Neben Tabellenblättern kann eine Arbeitsmappe zwei weitere Arten von Registern enthalten: **Dokumente**, um einen Bericht zu schreiben, und **Webseiten**, um eine kleine Anwendung zu bauen. Beide werden per Rechtsklick auf das „+“ der Register unten oder auf ein Register hinzugefügt (Einfügen).

## Dokument {#document}

Eine endlose Seite mit formatiertem Text: Überschriften, Listen, Zitate, Code, Ausrichtung, Schriftarten, Farben, Links und Bilder, wie in einer Textverarbeitung über das Menüband formatiert.

Sie können auch lebendige Elemente einfügen, die von den Tabellen der Arbeitsmappe gespeist werden:

- **Diagramme** und **PivotTables** (derselbe Bereich wie auf einem Blatt);
- **Schaltflächen**, die eine Funktion eines Skripts ausführen;
- **Felder** (Kontrollkästchen, Liste, Eingabefeld…), deren Wert in eine Zelle geschrieben wird.

Rechtsklick auf ein eingefügtes Element: dasselbe Menü wie auf dem Blatt; anderswo ein Menü zum Einfügen. Der Text wird kurz nach dem letzten Tastenanschlag von selbst gespeichert, als ein Rückgängig-Schritt der Arbeitsmappe. Datei › Drucken / PDF gilt für das Dokument.

## Webseite {#page-web}

Eine Funktion, die unter Datei › Einstellungen › Funktionen aktiviert wird (siehe [Einstellungen](parametres)); eine Arbeitsmappe, die bereits Webseiten enthält, zeigt und verwendet sie auch ohne diese Einstellung.

Eine von Ihnen geschriebene Webseite (HTML, CSS, JavaScript), in einem isolierten Rahmen angezeigt: Sie hat weder Zugriff auf den Datenträger noch auf den Rest der Anwendung, und ihr Netzwerkzugriff läuft über Lattis. Sie kommuniziert mit der Arbeitsmappe über ein kleines Objekt `lattis`:

```
lattis.sql("SELECT * FROM Bestellungen WHERE Betrag > ?", [50])
lattis.query("Top-Städte")
lattis.on("change", () => refresh())
lattis.call("sheets.list")
lattis.call("cells.set", {row: 1, col: 1, value: 42})
```

`lattis.sql` führt eine lesende SQL-Abfrage aus (SELECT oder WITH) und `lattis.query` eine gespeicherte Abfrage (Favorit) des [Registers SQL](sql) über ihren Namen; beide geben ein Array von Objekten zurück, eine Eigenschaft pro Spalte. `lattis.queries()` listet die gespeicherten Abfragen.

Die Befehle von `lattis.call` sind die der [Lua-Skripte](lua). Eine Seite erreicht eine Website nur über `lattis.call("http.get", …)`, mit Ihrer Zustimmung. Eine Seite, die von selbst versucht, eine andere Adresse zu öffnen (`location.href = …`), wird sofort neu geladen; die Anfrage an diese Adresse kann trotzdem hinausgegangen sein: ein Grund mehr, nur Seiten von Personen zuzulassen, denen Sie vertrauen.

**Ohne Skripte** (Webversion, deren Server keine Skripte erlaubt, öffentliche Demo) behält eine Seite das Lesen: `lattis.sql`, `lattis.query`, `lattis.on` und die lesenden Befehle von `lattis.call` (`sheets.list`, `tables.rows`, `cells.get`…). Ein Befehl, der die Arbeitsmappe ändert, eine Website erreicht oder ein Skript startet, wird abgelehnt, und ein Hinweis über der Seite sagt es.

Eine mit einer Arbeitsmappe erhaltene (oder anderswo geänderte) Webseite wird nicht sofort ausgeführt: Das Register fragt **Zulassen** oder **Nicht ausführen**, mit **Code anzeigen**. Eine Seite kann die Arbeitsmappe lesen und ändern: Lassen Sie sie nur zu, wenn Sie der Person vertrauen, die Ihnen diese Arbeitsmappe gegeben hat. Die Zustimmung gilt für diese Version der Seite, auf diesem Computer; Seiten, die Sie hier erstellen oder ändern, werden automatisch zugelassen. Nach „Nicht ausführen“ stellt die Schaltfläche **Entscheiden…** des Registers die Frage erneut.

Der Seiteneditor befindet sich neben der Vorschau: `Strg+S` speichert, `Strg+Eingabe` aktualisiert die Vorschau, ohne zu speichern.

## Externe Webseiten {#pages-web-externes}

Eine Webseite kann auch eine Seite einer Website anzeigen („Website…“), wenn Webseiten aktiviert sind.
