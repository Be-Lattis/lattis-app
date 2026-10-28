# KI-Assistent

Der **Assistent ✦** hilft Ihnen, eine Analyse aufzubauen: Er liest die Struktur Ihrer Tabellen, schlägt einen Ansatz vor und kann berechnete Spalten, Schlüssel, PivotTables und Diagramme hinzufügen, sortieren oder formatieren. Er öffnet sich in einem Bereich rechts neben dem Raster, wie Copilot in Excel, und bleibt geöffnet, wenn Sie das Blatt wechseln.

## Den Assistenten aktivieren {#activer-l-assistant}

Der Assistent ist standardmäßig deaktiviert: Aktivieren Sie **KI-Assistent** unter Datei › Einstellungen › Funktionen. Die Schaltfläche **Assistent ✦** erscheint dann oben rechts im Menüband (`Alt+I`) und öffnet seinen Bereich; deaktiviert gibt es weder Schaltfläche noch Tastenkürzel noch Bereich. In einem Unternehmen kann die Richtlinie der Organisation ihn deaktivieren oder vorgeben.

## Verbinden {#se-connecter}

Die Schaltfläche **Verbindung** in der Kopfzeile des Bereichs bietet zwei Wege an. Das Panel öffnet sich von selbst, solange nichts verbunden ist.

### API-Schlüssel (empfohlen) {#cle-d-api-recommande}

1. Wählen Sie den Anbieter (Anthropic, Mistral, OpenAI, Gemini, Mammouth…) und das Modell, dann **Konsole öffnen**: die Seite des Anbieters, auf der Sie einen Schlüssel erstellen.
2. Kopieren Sie den Schlüssel, dann **Einfügen** (oder `Strg+V` im Feld).
3. **Prüfen und speichern**: Lattis testet den Schlüssel beim Anbieter, bevor er gespeichert wird. Ein abgelehnter Schlüssel oder ein Konto ohne Guthaben ergibt eine klare Meldung im Panel.

Der Schlüssel wird im Schlüsselbund des Systems aufbewahrt, nie in der Arbeitsmappe. **Schlüssel vergessen** entfernt ihn.

> Bei Anthropic funktioniert ein Schlüssel nur mit Guthaben: Laden Sie es auf der Seite **Billing** der Konsole auf (platform.claude.com). Die Abonnements **Claude Max** und **Team** bringen jeden Monat API-Guthaben (100 bis 500 US-Dollar je nach Tarif): Verknüpfen Sie die Organisation der Konsole unter claude.ai › Settings › Billing, um es zu erhalten. Die Abonnements Pro und Enterprise bringen keines.

### Desktop-App (Abonnement Claude Pro, Max, Team) {#application-de-bureau-abonnement-claude-pro-max-team}

Ein Claude-Abonnement kann nicht direkt in einer anderen Anwendung verwendet werden, aber **Claude Desktop** und **Claude Code** können Lattis über seinen MCP-Server steuern. Das Register „Desktop-App“ des Verbindungspanels (App: Claude) richtet sie mit einem Klick ein; starten Sie Claude Desktop danach neu.

Sie sprechen dann mit Claude in seiner App. Der Bereich des Assistenten zeigt **Von Claude**: was Claude in der Arbeitsmappe tut, während es geschieht, mit **Diese Änderungen rückgängig machen** für jede Folge von Aktionen. Die Option **Vor Änderungen fragen** lässt Sie jeden Schreibvorgang in Lattis bestätigen, auch das Speichern unter einem anderen Namen, Exporte und Importe. Die Einstellung **An den Assistenten gesendete Daten** gilt auch für Claude (siehe unten).

### Gateway der Organisation {#passerelle-de-l-organisation}

Stellt das Unternehmen ein eigenes Gateway bereit, meldet der Bereich „Über das Gateway Ihrer Organisation verbunden“: Es ist kein Schlüssel einzugeben und kein Anbieter zu wählen.

### Lokales Modell {#modele-local}

Der Anbieter **Lokales Modell** verwendet ein auf diesem Computer installiertes Modell (Ollama, LM Studio…): Keine Daten verlassen den Computer, und es gibt keinen Schlüssel. Seine Adresse wird im Verbindungspanel oder unter Datei › Einstellungen › KI-Assistent festgelegt. Das ist experimentell: Kleine Modelle rufen die Werkzeuge von Lattis schlechter auf.

## Unterhalten {#converser}

Geben Sie Ihre Anfrage ein, dann `Eingabe` (`Umschalt+Eingabe` beginnt eine neue Zeile). Jede Nachricht geht mit dem hinaus, was Sie gerade sehen: das aktive Blatt, die Tabelle unter dem Cursor und die Auswahl, angezeigt über dem Eingabefeld („Kontext: Bestellungen, Auswahl Betrag[3:10]“). „Analysiere diese Tabelle“ oder „erkläre diese Spalte“ beziehen sich also auf das Ausgewählte.

- Die Schritte des Assistenten (eine Tabelle lesen, eine Spalte hinzufügen…) werden in einer Zeile **N Schritte** zusammengefasst, während der Antwort aufgeklappt und danach zugeklappt; ein Klick öffnet sie wieder. Der Name eines unterstrichenen Schritts führt zum betroffenen Blatt oder zur betroffenen Tabelle.
- Ein fehlgeschlagener Schritt, den der Assistent anschließend behoben hat, wird grau angezeigt; nur ein ungelöst gebliebener Fehler ist rot.
- `Esc` stoppt die aktuelle Antwort. Wurde sie gestoppt, bevor das Modell geantwortet hat, oder ist sie in diesem Moment fehlgeschlagen (Netzwerk, Kontingent), verschwindet sie, und Ihre Nachricht kehrt ins Eingabefeld zurück: Das Modell hat sie nicht erhalten.
- Die Pfeiltaste nach oben ruft in einem leeren Eingabefeld die vorherige Nachricht auf; die Kopierschaltfläche unter einer Antwort legt sie in die Zwischenablage.
- **Wiederholen** unter einer durch einen Fehler unterbrochenen Antwort (Anbieter überlastet, Ausfall) setzt sie dort fort, wo sie aufgehört hat, ohne Ihre Nachricht erneut zu senden: Was sie bereits geändert hat, wird nicht noch einmal ausgeführt, und die Antwort lässt sich weiterhin auf einmal rückgängig machen. Verlangt der Fehler zu warten („in 30 s erneut versuchen“), zählt die Schaltfläche die Sekunden herunter.
- Eine sehr lange Antwort stoppt nach 100 Schritten: **Fortsetzen** setzt sie auf dieselbe Weise fort.
- Eine Frage zu den Daten („welche Stadt hat den besten Umsatz?“) erhält unter der Antwort ein **Ergebnis**: Der Assistent schreibt eine Abfrage, Lattis führt sie auf dem Computer aus und zeigt Ihnen höchstens 50 Zeilen. Im Modus „Nur Struktur“ ist das Ergebnis nur für Sie: Der Assistent sieht weder seine Werte noch seine Zeilenanzahl.
- Ein Link in einer Antwort zeigt seine Domain („Siehe den Bericht (example.com)“); ein Klick zeigt die vollständige Adresse und bittet um Bestätigung, bevor sie im Browser geöffnet wird.
- **Neu** (Kopfzeile des Bereichs) beginnt eine neue Unterhaltung; die vorherige bleibt in der Liste der Unterhaltungen (☰), aus der sie fortgesetzt oder (nach Bestätigung) gelöscht werden kann.
- Wird die Unterhaltung lang, fasst der Assistent ihren Anfang zusammen, um fortzufahren: Die Markierung **Sitzung komprimiert** zeigt, wo. Alles bleibt im Bereich angezeigt; nur das Modell arbeitet nun mit der Zusammenfassung.

## Schlecht erkannte Tabellen reparieren {#reparer-des-tableaux-mal-reconnus}

Der Vorschlag **Tabellen auf diesem Blatt reparieren** lässt den Assistenten das Layout des Blatts neu lesen: wo die Tabellen stehen, die Art der Zellen in ihrer ersten und letzten Zeile (Text, Zahl, Datum, Formel, leer), fette oder verbundene freie Zellen und Zeilen, die wie ein Titel oder eine Summe aussehen. Er liest ihre Werte nicht, auch nicht im Modus „Nur Struktur“. Danach korrigiert er mit den Befehlen, die die Tabelle, ihren Schlüssel und die auf sie verweisenden Formeln erhalten: für die Überschrift gehaltener Titel, in den Daten verbliebene Überschrift, aus der Tabelle verschobene Summenzeile, Überschriften auf zwei Zeilen, zu teilende Spalten.

Was der Assistent in der Arbeitsmappe liest (Namen, Zelltext), wird ihm als Daten präsentiert, nie als Anweisung: Einer Zelle, die ihn „auffordert“, eine Spalte zu löschen, darf er nicht gehorchen, und er sagt es. Er löscht eine Spalte, ein Diagramm, eine PivotTable oder Duplikate nur, wenn Sie ihn darum bitten.

Die Einstellung **Unterhaltung in der Arbeitsmappe behalten** (Datei › Einstellungen › KI-Assistent) speichert sie mit der Arbeitsmappe; andernfalls wird sie gelöscht, wenn Sie die Arbeitsmappe schließen oder eine andere öffnen. Wird sie behalten, reist sie mit der Datei, einschließlich der angezeigten Ergebnisse und der vom Assistenten gelesenen Zeilen, und wird nur gespeichert, wenn auch die Arbeitsmappe geändert wurde. Ändert sich die Arbeitsmappe außerhalb der Antworten (Import, `Strg+Z`, später fortgesetzte Unterhaltung), erhält der Assistent mit der nächsten Nachricht ihre aktuelle Struktur.

## Rückgängig machen {#annuler}

Alles, was der Assistent während einer Antwort ändert, bildet einen einzigen Schritt: Die Antwort endet mit ihrer Zusammenfassung („3 Änderungen“) und der Schaltfläche **Diese Antwort rückgängig machen**. `Strg+Z` im Raster bewirkt dasselbe. Die Schaltfläche ist nur für die letzte Antwort aktiv, solange seitdem nichts anderes geändert wurde; andernfalls verwenden Sie `Strg+Z`.

## Gesendete Daten {#donnees-transmises}

Die Einstellung **An den Assistenten gesendete Daten** (Datei › Einstellungen › KI-Assistent, unten im Bereich wiederholt) bestimmt, was der Assistent lesen darf:

| Einstellung | Was der Anbieter erhält |
| --- | --- |
| Nur Struktur (Standard) | Namen der Blätter, Tabellen und Spalten, Typen, Anzahl der Zeilen und der eindeutigen Werte, Formeln, Position der Tabellen und Zellen auf dem Blatt (ohne ihren Inhalt, mit ihrer Art und den Hinweisen „Titel“ oder „Summe“); nicht das Ergebnis der angezeigten Abfragen und nicht die Anzahl der Zellen einer Ersetzung; Abfrageschritte und Verbindungsparameter, ohne Kennwörter und ohne die Werte von HTTP-Headern und Filtern. Die Spalten einer PivotTable, die die Werte eines Felds tragen, werden zusammengefasst („3 Wertespalte(n) des Felds „Kunde““) |
| Struktur und Beispielzeilen | Dasselbe, plus einige Zeilen bei jedem Lesen: höchstens 20 Zeilen, 200 Zeichen pro Wert |
| Gesamte Arbeitsmappe | Alles, was der Assistent lesen möchte, ohne Grenze |

Der Anbieter erhält außerdem Ihre Nachrichten und den Kontext (Blatt, Tabelle, Auswahl). Mit einem lokalen Modell verlässt nichts den Computer.

Die Einstellung gilt über den MCP-Server auch für **Claude Desktop** und **Claude Code**: Ohne „Gesamte Arbeitsmappe“ werden ihnen die Werkzeuge, die Werte lesen (Tabellenzeilen, SQL, Zellen, Skripte, Exporte), nicht angeboten oder auf dieselbe Weise eingeschränkt. Starten Sie Claude nach einer Änderung neu, damit es die neue Liste der Werkzeuge sieht. Claude Code liest auch die Dateien auf Ihrem Datenträger: Eine gespeicherte Arbeitsmappe bleibt für es direkt lesbar.

## Kosten {#cout}

Mit einem API-Schlüssel wird jede Antwort vom Anbieter nach der Anzahl der gelesenen und geschriebenen Tokens abgerechnet. Mit einem Anthropic-Schlüssel zeigt der Fuß des Bereichs eine Schätzung der Kosten der Unterhaltung („≈ 0,12 $ für diese Unterhaltung“) auf Grundlage der öffentlichen Preise; maßgeblich ist die Rechnung des Anbieters. Für andere Anbieter oder über ein Gateway, deren Preise abweichen, wird nichts angezeigt. Eine lange Folge von Schritten kostet mehr: Ein leichteres Modell (Sonnet statt Opus) genügt meist.

## Grenzen {#limites}

- Der Assistent kann sich irren: Prüfen Sie, was er vorschlägt, und machen Sie eine Antwort rückgängig, die Ihnen nicht passt.
- Im Modus „Nur Struktur“ sieht er die Werte nicht: Er kann den Inhalt der Zellen nicht beurteilen. Im Modus „Struktur und Beispielzeilen“ sieht er nur einen Teil davon.
- Eine sehr lange Unterhaltung wird zusammengefasst („Sitzung komprimiert“): Der Assistent kann dann ein Detail der ersten Wortwechsel vergessen. Beginnen Sie für ein neues Thema lieber eine neue Unterhaltung.
- Mit Claude Desktop oder Claude Code bleibt die Unterhaltung in Claude; Lattis sieht nur die Aktionen.
