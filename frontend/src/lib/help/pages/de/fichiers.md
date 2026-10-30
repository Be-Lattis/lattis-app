# Dateien und Wiederherstellung

Eine Lattis-Arbeitsmappe ist eine einzige `.lattis`-Datei: ein ZIP-Archiv mit einer komprimierten SQLite-Datenbank und einem kurzen Manifest. Sie können sie per E-Mail versenden und archivieren wie eine Excel-Arbeitsmappe; einmal mit 7-Zip oder WinRAR entpackt (der Windows-Explorer kann ihre Komprimierung Zstandard nicht lesen), kann jedes SQLite-Werkzeug ihre Tabellen lesen.

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

## Einer live geteilten Arbeitsmappe eine Tabelle hinzufügen {#ajouter-un-tableau-a-un-classeur-partage-en-direct}

In einer kollaborativen Arbeitsmappe (live auf einem Lattis-Server geteilt) wird eine nach dem Veröffentlichen vom Besitzer oder einem Designer angelegte Tabelle (eingegeben, eingefügt, über Einfügen, aus einem Bereich umgewandelt…) automatisch der geteilten Arbeitsmappe hinzugefügt, sobald sie entsteht; offline bei der Wiederverbindung. Eine Tabelle, die auf Ihrem Computer bleibt (von einer Abfrage geladen, vom Server abgelehnt oder von einem Bearbeiter angelegt), trägt über ihrem Rahmen das bernsteinfarbene Etikett **auf diesem Rechner**: Die anderen Mitglieder sehen sie nicht. Ein Klick auf das Etikett oder Rechtsklick auf die Tabelle › **Zur geteilten Arbeitsmappe hinzufügen…** legt sie in der Arbeitsmappe des Servers an, mit ihren Zeilen: Die anderen Mitglieder erhalten sie sofort (an derselben Stelle des Blatts oder darunter, wenn die Stelle bei ihnen belegt ist), mit einer Meldung in der Statusleiste, und sie wird danach wie die anderen geteilten Tabellen synchronisiert (Eingaben, Konflikte, Rechte). Dieselbe Aktion wird unter **Im Lattis Data Lake teilen…** angeboten („Diese Tabelle zur geteilten Arbeitsmappe hinzufügen“, zuerst) sowie unter **Zugriff verwalten** › Rollen und Rechte, unter der Liste der Tabellen.

- Den Besitzern und Designern der Arbeitsmappe vorbehalten, und nur online: Andernfalls meldet es das Dialogfeld.
- Die Spaltentypen werden wie bei den anderen geteilten Tabellen festgelegt; berechnete Spalten bleiben auf jedem Computer berechnet. Ungültige Eingaben müssen zuerst korrigiert werden.
- Eine Ablehnung durch den Server (zum Beispiel Name bereits von einer anderen Tabelle der Arbeitsmappe belegt) erscheint in der Statusleiste und ändert nichts.
- Das Hinzufügen lässt sich nicht mit `Strg+Z` rückgängig machen (das Bestätigungsfeld erinnert daran): Um die Tabelle aus der geteilten Arbeitsmappe zu entfernen, löscht **Tabelle löschen** sie für alle. Das Rückgängigmachen (`Strg+Z`) der Erstellung der Tabelle selbst löscht sie ebenfalls vom Server, für alle; `Strg+Y` legt sie wieder an und fügt sie erneut hinzu.

## Zugriff auf eine geteilte Arbeitsmappe verwalten {#gerer-l-acces-a-un-classeur-partage}

**Datei › Zugriff verwalten…** (oder das Kontomenü) legt fest, wer eine auf einem Lattis-Server veröffentlichte Arbeitsmappe sieht und ändert. Jedes Mitglied hat eine oder mehrere **Rollen**; seine Rechte sind die seiner Rollen zusammen.

- **Mitglieder**: Laden Sie eine Adresse mit ihren Rollen ein (das Dialogfeld nennt, was sie erlauben werden), entfernen Sie eine Rolle mit dem ✕ auf ihrem Chip, vergeben Sie eine mit **+ Rolle**. Unter jeder Person nennt eine Zeile, was sie tun kann („Anzeigen, hinzufügen, eigene Zeilen bearbeiten“). Über ein Team erhaltene Rollen sind gepunktet und werden unter **Teams** geändert. Der letzte Besitzer ist geschützt: Vergeben Sie diese Rolle zuerst an jemand anderen.
- **Rollen und Rechte**: Eine Tabelle zeigt für jede Rolle, ob sie **Anzeigen**, **Zeilen hinzufügen**, **Eigene Zeilen bearbeiten** (die von ihr hinzugefügten, Löschen inbegriffen), **Alle Zeilen bearbeiten** und **Struktur und Rechte** ändern kann (✓ ja, ◐ teilweise: manche Zeilen oder manche Tabellen, – nein). Wählen Sie eine Rolle, um ihren Zugriff auf die Arbeitsmappe, auf jedes Blatt und auf jede Tabelle festzulegen: *Kein Zugriff*, *Anzeigen*, *Anzeigen, hinzufügen, eigene Zeilen bearbeiten* oder *Alles anzeigen und bearbeiten*; „Wie Blatt“ oder „Wie Arbeitsmappe“ behält den Zugriff der Ebene darüber. Unter jeder Tabelle lesen sich Filter als Sätze („Sieht nur Zeilen, in denen Region = „Nord““, „Bearbeitet nur Zeilen, in denen Verkäufer = die angemeldete Person“); **Einschränken…** oder **Bearbeiten** legt sie fest. Der Besitzer hat alle Rechte; der Designer ebenso, außer über die Rechte.
- **Erweiterte Regeln** (Besitzer): Eine Regel entzieht ein Recht an einer Tabelle unabhängig vom Zugriff der Rolle und liest sich als ein Satz („Niemand kann Zeilen in „Umsatz“ löschen, außer Besitzern und Designern.“). Beginnen Sie mit einer Vorlage — **Löschen verbieten**, **Nur hinzufügen**, **Nur Zeilen mit Status … bearbeiten**, **Eigene Zeilen bearbeiten, ohne neue anzulegen** (für eine Rolle, die ihre Zeilen hinzufügt und bearbeitet: Sie kann dann nur noch bearbeiten) — oder wählen Sie die Tabelle, die Rollen, die verbotenen Aktionen und die mögliche Ausnahme.
- **Anzeigen als…**: In der Zeile eines Mitglieds oder einer Rolle oder in ihrem Register zeigt, was dieses Mitglied mit einer Tabelle tun kann und welche Zeilen es sieht, auf dem Server unter seiner Identität gelesen (Regeln inbegriffen).
- **Abweichungen**: außerhalb von Lattis, direkt in PostgreSQL geänderte Regeln; **Rechte neu kompilieren** stellt die von Lattis wieder her.

**Vom Server entfernen…** (Besitzer; unten in diesem Dialog oder unter Datei) löscht die Arbeitsmappe für alle vom Server: Daten und Berechtigungen, Pipelines und Formulare; ihre Freigaben im Data Lake werden getrennt. Kopien auf den Rechnern bleiben: Die anderen Mitglieder sehen, dass die Arbeitsmappe gelöscht wurde, und können ihre behalten (**Lokale Kopie speichern…**). Ihre bleibt als lokale Arbeitsmappe geöffnet. Eine Bestätigung fragt vorher; das lässt sich nicht rückgängig machen.

## Server nicht erreichbar: offline arbeiten oder trennen {#serveur-injoignable-travailler-hors-ligne-ou-se-detacher}

Antwortet der Server einer geteilten Arbeitsmappe nicht mehr (Server gestoppt, Netzwerk ausgefallen), erscheint unter dem Menüband ein orangefarbenes Banner, zum Beispiel „Der Server localhost:8080 antwortet nicht“. **Details** zeigt den technischen Fehler (auch als QuickInfo und im Protokoll).

- **Kollaborative Arbeitsmappe**: Sie bleibt bearbeitbar. Eingaben, hinzugefügte oder gelöschte Zeilen, hinzugefügte oder umbenannte Spalten warten auf Ihrem Computer (kleines graues Dreieck in der Ecke der Zellen, „Offline — 3 ausstehend“ in der Statusleiste), selbst wenn Sie Lattis schließen: Sie bleiben mit der Arbeitsmappe erhalten. Wenn der Server zurück ist, werden sie in der Reihenfolge gesendet, in der Sie sie vorgenommen haben. Eine zwischenzeitlich von einem Kollegen geänderte Zelle wird zu einem **Konflikt** (Bereich Konflikte: Ihre oder seine Version behalten); was der Server ablehnt (während des Ausfalls entzogene Rechte, unantastbar gewordene Spalte, von einer Kollegin gelöschte Zeile), wird auf Ihrem Computer rückgängig gemacht, mit einer Meldung.
- **Eingefrorene Arbeitsmappe**: Sie bleibt schreibgeschützt einsehbar, bis der Server zurück ist.

Lattis versucht es von selbst erneut, immer seltener (von einigen Sekunden bis zu zwei Minuten); das Banner verschwindet, sobald der Server antwortet. Die Schaltflächen des Banners:

- **Wiederholen** (auch Datei › **Verbindung wiederholen**): versucht es sofort.
- **An einer lokalen Kopie arbeiten**: speichert die Arbeitsmappe unter einem Namen Ihrer Wahl als gewöhnliche Arbeitsmappe ohne Verbindung zum Server und öffnet sie dann. Noch nicht gesendete Änderungen sind in der Kopie (die Meldung sagt es); sie bleiben auch in der geteilten Arbeitsmappe ausstehend, die nicht angetastet wird.
- **Vom Server trennen…** (Besitzer der Arbeitsmappe; auch unter Datei, online oder offline): Diese Arbeitsmappe wird zu einer gewöhnlichen lokalen Arbeitsmappe, die nicht mehr synchronisiert wird. Die Arbeitsmappe des Servers wird nicht geändert (die anderen Mitglieder behalten sie, Sie können sie vom Server erneut öffnen). Eine vom Server geöffnete Arbeitsmappe ist nur eine Kopie im Cache von Lattis: Lattis fragt, wo sie gespeichert werden soll, und entfernt dann den Cache. Das Trennen lässt sich nicht rückgängig machen, und eine Bestätigung erinnert daran.

## Eine Tabelle im Lattis Data Lake teilen {#partager-un-tableau-au-data-lake-lattis}

Rechtsklick auf eine Tabelle › **Im Lattis Data Lake teilen…** macht die Tabelle aus anderen Arbeitsmappen auf dem Lattis-Server lesbar (Einfügen › Lattis Data Lake, Registerkarte SQL), gemäß einer Zielgruppe: Personen, Teams, mit einem Zeilenfilter für jede. Ist die Arbeitsmappe nicht veröffentlicht, bietet das Dialogfeld zwei Wege an:

- **Die Arbeitsmappe mit Rechteverwaltung veröffentlichen**: Die ganze Arbeitsmappe wird auf dem Server veröffentlicht (Datei › Auf einem Server veröffentlichen…), mit ihren Rollen und Filtern; das Dialogfeld kommt dann zurück, um die Tabelle zu teilen.
- **Nur diese Tabelle als Datenquelle veröffentlichen**: Die Arbeitsmappe bleibt auf Ihrem Computer und wird nicht bereitgestellt. Der Server behält eine Kopie der Tabelle, gespeist von einer Pipeline, deren Quelle Ihre Arbeitsmappe ist. Nur die **aktivierten Spalten** werden gesendet, mit ihren Werten (eine berechnete Spalte sendet ihre Ergebnisse, nicht ihre Formel); die anderen Tabellen, die Blätter, Formeln, Skripte und nicht aktivierten Spalten verlassen den Computer nicht.

Eine Quelle wird aktualisiert, wenn Sie es verlangen: Rechtsklick auf die Tabelle › **Data-Lake-Quelle aktualisieren** oder über dasselbe Dialogfeld; aktivieren Sie „Quelle bei jedem Speichern der Arbeitsmappe aktualisieren“, um sie bei jedem `Strg+S` zu senden. Jede Aktualisierung sendet die ganze Tabelle, geschrieben gemäß dem gewählten Modus: **Alles ersetzen** (der Datenbestand wird zur gesendeten Tabelle), **Anfügen** (Zeilen werden den bereits empfangenen hinzugefügt) oder **Nach Schlüssel aktualisieren** (eine Zeile mit demselben Schlüssel wird geändert, eine neue wird hinzugefügt). Die Übersicht erscheint in der Statusleiste: gelesene, hinzugefügte, geänderte, gelöschte Zeilen und die vom Server abgelehnten Werte, mit dem ersten (Zeile, Spalte, Grund). Eine gesendete Spalte, die Sie in der Arbeitsmappe umbenennen, umwandeln oder löschen, wird bei der nächsten Aktualisierung auch auf dem Server umbenannt, umgewandelt oder gelöscht. Die Verknüpfung mit der Quelle wird in der Arbeitsmappe gespeichert (speichern Sie sie); das Dialogfeld erlaubt auch, den Namen, die gesendeten Spalten, den Modus, den Schlüssel und die Zielgruppe zu ändern, die Quelle **auszusetzen** (nichts wird mehr gesendet, der Datenbestand behält seine letzten Zeilen) und **die Quelle zu entfernen** (Arbeitsmappen, die sie verwenden, behalten ihre letzte Kopie). Die Dialogfelder Auf einem Server veröffentlichen und Zugriff verwalten listen die Quellen der Arbeitsmappe auf, gekennzeichnet mit „Pipeline“. Unter Einfügen › Lattis Data Lake trägt ein allein veröffentlichter Datenbestand dieselbe Kennzeichnung und nennt die Arbeitsmappe, aus der er stammt; ohne gewählten Datenbestand zeigt die Liste „Ihre gesendeten Quellen“ alle Ihre Quellen auf dem Server, aus welcher Arbeitsmappe sie auch stammen.

Sie benötigen die Rolle „Datenherausgeber“ auf dem Server und einen Server, der von einer Arbeitsmappe gesendete Quellen akzeptiert; andernfalls meldet es das Dialogfeld.

## Eine Tabelle als Formular teilen {#partager-un-tableau-comme-formulaire}

Setzen Sie die aktive Zelle in eine Tabelle, dann **Einfügen › Formular**: Eine Webseite lässt Personen diese Tabelle ausfüllen, ohne die Arbeitsmappe zu öffnen, vom Telefon oder vom Computer aus. Jede Übermittlung fügt eine Zeile hinzu, die die Mitglieder live ankommen sehen; die Zeile wird im Namen des Formulars geschrieben („Formular: <Titel>“), die antwortende Person hat keine Rechte an der Arbeitsmappe.

Das Formular wird vom Lattis-Server bereitgestellt: Die Tabelle muss in einer kollaborativen Arbeitsmappe veröffentlicht sein, von einem Besitzer oder Designer der Arbeitsmappe, online. Andernfalls nennt das Dialogfeld, was fehlt, und bietet die zuerst auszuführende Aktion an: **Auf einem Server veröffentlichen…** (lokale Arbeitsmappe) oder **Zur geteilten Arbeitsmappe hinzufügen…** (Tabelle, die nur auf Ihrem Computer ist), dann öffnet sich der Designer. Eine eingefrorene Arbeitsmappe, eine von einer Abfrage geladene Tabelle, ein verknüpfter Data-Lake-Datenbestand oder eine gesendete Quelle kann kein Formular erhalten. Ein Mitglied, das weder Besitzer noch Designer ist, kann den Link der für es geöffneten Formulare kopieren.

Der Designer hat drei Register, mit der Seitenvorschau rechts (Telefon- oder Computerbreite); **Vorschau im Browser öffnen** zeigt die echte Seite des Servers, ohne die Antworten zu speichern.

- **Felder**: ein Feld pro Spalte (Kontrollkästchen „im Formular“, Griff zum Verschieben, Beschriftung, Hilfe, Platzhaltertext, Pflichtfeld, Standardwert fest oder heute, jetzt, aktuelles Jahr, Adresse der antwortenden Person). Der Feldtyp folgt der Spalte: Text oder langer Text, Zahl (mit Grenzen und Einheit), Datum, Datum und Uhrzeit, Kontrollkästchen oder Ja / Nein, Optionsfelder oder eine durchsuchbare Liste für eine Spalte mit Überprüfungsliste. Eine aus einer Spalte einer veröffentlichten Tabelle übernommene Liste wird vom Server neu gelesen; jede andere Liste wird kopiert („Liste kopiert am…“, **Aktualisieren**). Der nummerierte Schlüssel wird vom Server vergeben; ein eingegebener Schlüssel ist Pflicht. Ein Feld **Position** füllt Breitengrad und Längengrad (oder eine Spalte „lat, lon“), wenn die antwortende Person auf „Mein Standort“ tippt. **Vom Server gesetzte Werte** füllen eine Spalte ohne sichtbares Feld: Empfangen am, Antwortende Person, Formular (sein Titel), nächste Nummer oder fester Wert; **Spalte „Empfangen am“ hinzufügen** legt die Spalte in der Tabelle an. Bei einem Auswahlblatt ist die Kategorie ein fester Wert, das aktuelle Jahr oder ein Feld. Zwischenüberschriften teilen das Formular in Abschnitte.
- **Darstellung**: Titel, Einleitung (fett, kursiv, Links, Listen), Logo und Banner, Farbe aus einer gut lesbaren Palette, ein Abschnitt pro Seite, Beschriftung der Schaltfläche, Dankesnachricht, Nachricht bei geschlossenem Formular, Datenschutzhinweis (mit einer Vorlage), Sprache der Seite (Französisch oder Englisch). Kein Hochladen von Fotos oder Dateien.
- **Zugriff**: Mitglieder der Arbeitsmappe (Leser inbegriffen), ausgewählte Personen und Teams oder alle, die ein Konto auf dem Server haben; alle melden sich mit ihrem Lattis-Konto an.

Unten erscheinen Prüfungen in Orange (leere Liste, Standardwert nicht in der Liste, für Antwortende sichtbare Listenwerte…). **Entwurf speichern** behält das Formular, ohne es zu öffnen; **Formular öffnen** stellt es online und liefert seinen Link, seinen QR-Code (als PNG oder SVG zu speichern) und **Vorausgefüllter Link…**: ein Link pro Wert (zum Beispiel ein QR-Code pro Standort), der beim Öffnen Felder ausfüllt, mit der Option „durch den Link gesperrt“, damit sie nicht geändert werden können. Hat die Tabelle bereits Formulare, listet das Dialogfeld sie zuerst auf (Link kopieren, Bearbeiten…), mit **Neues Formular…**.

## Importieren {#importer}

Datei › Importieren liest eine CSV-Datei (oder Text mit Trennzeichen), Arbeitsmappen `.xlsx`, `.xlsm` und OpenDocument `.ods`: Jedes Tabellenblatt wird zu einem Register, jeder Datenblock zu einer Tabelle, mit Formatierung, Notizen, Auswahllisten und bedingter Formatierung. Diagramme und PivotTables werden übernommen, wenn ihre Daten eine Tabelle der Arbeitsmappe sind. Was nicht übernommen wird (Makros, andere Objekte), wird am Ende des Imports gemeldet.

Für Daten, die aktualisiert werden sollen (JSON- oder Parquet-Datei, SQLite-Datenbank, PostgreSQL, MySQL, SQL Server, Webseite), verwenden Sie stattdessen eine Abfrage: siehe [Daten importieren und Verbindungen](connexions).

## Exportieren {#exporter}

- **CSV**: die aktive Tabelle oder das ganze Blatt. In einer CSV für Excel wird einem Text, der mit `=`, `+`, `-` oder `@` beginnt, ein Apostroph vorangestellt, damit Excel ihn nicht als Formel ausführt.
- **Excel (`.xlsx`) und OpenDocument (`.ods`)**: die ganze Arbeitsmappe. Tabellen werden zu Excel-Tabellen, Formeln werden übersetzt, wenn das Format eine Entsprechung hat, sonst als ihr Wert exportiert. PivotTables gehen als Werte hinaus; Diagramme, bedingte Formatierung, Auswahllisten und Notizen gehen in `.xlsx` hinaus; Skripte werden nicht exportiert. Ein Bericht nennt, was nicht unverändert hinausging.
- **SQL (`.sql`)**: ein Skript mit `CREATE TABLE` und dann `INSERT` für die Tabellen und PivotTables (als Werte), für die ganze Arbeitsmappe, das aktive Blatt oder die aktive Tabelle. Der Dialekt Standard passt zu den meisten Datenbanken; PostgreSQL, MySQL, SQL Server und SQLite passen Namen, Typen und Wahrheitswerte an ihre Datenbank an. Die Schlüsselspalte wird zum Primärschlüssel; eine berechnete Spalte geht als Werte hinaus, ihre Formel als Kommentar; freie Zellen werden nicht exportiert.
- **PDF**: Datei › Drucken, mit dem Drucker „Microsoft Print to PDF“ (siehe [Seitenlayout und Drucken](impression)).

## Beispielarbeitsmappe {#classeur-d-exemple}

Die Startseite bietet an, eine Beispielarbeitsmappe zu öffnen: Umsätze, Kunden, PivotTables, Diagramme, eine Karte, SQL-Abfragen und Skripte, mit einem Blatt „Entdecken“, das sie vorstellt. Sie wird nie von selbst gespeichert: Datei › Speichern unter, um sie zu behalten.
