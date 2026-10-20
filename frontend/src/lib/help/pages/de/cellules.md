# Formatierung, Notizen und Anzeige

Die Befehle des Menübands folgen denen von Excel, an denselben Stellen. Sie gelten für die Auswahl (oder die aktive Zelle) und lassen sich mit `Strg+Z` rückgängig machen.

## Zellen formatieren {#mise-en-forme-des-cellules}

Start › Schriftart, Ausrichtung und Zahl formatieren die Auswahl; das kleine Startprogramm in der Ecke jeder Gruppe oder `Strg+1` öffnet das Dialogfeld **Zellen formatieren** (Zahl, Ausrichtung, Schrift, Rahmen, Ausfüllen). Ein Klick auf die bereits gewählte Ausrichtung (Links, Zentriert, Rechts) stellt wie in Excel die Standardausrichtung wieder her.

## Schrift- und Füllfarben {#couleurs-de-police-et-de-remplissage}

Start › Schriftfarbe und Füllfarbe (Hervorheben in einem Dokument): Die Schaltfläche wendet die zuletzt gewählte Farbe an, der ▾-Pfeil öffnet die Palette, wie in Excel.

- **Designfarben**: die zehn Farben des Designs der Arbeitsmappe (Hintergrund 1 und Text 1, Weiß und Schwarz; Hintergrund 2 und Text 2; Akzent 1 bis 6, darunter die Farbe der Tabellen und die der externen Quellen), und unter jeder fünf hellere oder dunklere Abstufungen. Eine hier gewählte Farbe **folgt dem Design**: Wird das Design geändert (Ansicht › Design), werden diese Zellen neu eingefärbt.
- **Standardfarben**: Dunkelrot, Rot, Orange, Gelb, Hellgrün, Grün, Hellblau, Blau, Dunkelblau und Lila, unabhängig vom Design immer gleich.
- **Zuletzt verwendete Farben**: die auf diesem Computer mit „Weitere Farben…“ gewählten.
- **Keine Füllung** oder **Automatisch** entfernt die Farbe; **Weitere Farben…** erlaubt eine beliebige Farbe.

Die QuickInfo jedes Farbfelds nennt seinen Namen („Grün, Akzent 1, heller 40 %“). Mit der Tastatur bewegen die Pfeiltasten durch die Farbfelder, und `Eingabe` wählt aus. Dieselbe Palette wird in Zellen formatieren (`Strg+1`) und in der bedingten Formatierung verwendet. Beim Export nach Excel oder OpenDocument werden Designfarben so geschrieben, wie sie angezeigt werden.

Start › Bearbeiten › Löschen entfernt die Formate, den Inhalt (`Entf`) oder die Notiz der Auswahl. Füllbereich kopiert die erste Zeile nach unten (`Strg+D`) oder die erste Spalte nach rechts (`Strg+R`).

Start › Als Tabelle formatieren (`Strg+L`) macht aus dem Block freier Zellen um die aktive Zelle eine Tabelle, deren erste Zeile die Überschriften liefert. Auf einer leeren Zelle legt Einfügen › Tabelle (oder `Strg+L`) eine leere Tabelle mit einer Spalte „Spalte 1“ an, die Überschrift auf der Zelle: Geben Sie darunter ein, um sie zu füllen, in der Überschrift, um die Spalte umzubenennen.

## Bedingte Formatierung {#mise-en-forme-conditionnelle}

Start › Bedingte Formatierung färbt Zellen abhängig von ihrem Wert: Regeln zum Hervorheben (größer als, zwischen, Text, der enthält, doppelte Werte…), Datenbalken, Farbskalen und Symbolsätze. Der **Regel-Manager** listet die Regeln des Blatts auf, um sie zu bearbeiten oder zu löschen. In einer Tabelle gilt eine Regel für die ganze Spalte, auch für künftige Zeilen.

## Datenüberprüfung {#validation-des-donnees}

Daten › Datenüberprüfung legt die Liste der Werte fest, die in der Auswahl oder in einer ganzen Tabellenspalte zulässig sind: Die Zelle zeigt einen Dropdownpfeil (`Alt+↓` öffnet ihn), und jede andere Eingabe wird abgelehnt.

Die Liste wird eingegeben (**Eingegebene Werte**, einer pro Zeile) oder wie in Excel aus einem **Bereich** übernommen:

- eine Tabellenspalte, zum Beispiel einer Nachschlagetabelle: `Kunden[Ort]` (oder einige ihrer Zeilen, `Kunden[Ort][1:5]`);
- Zellen: `Cell[A1:A10]` oder `Tabelle2!Cell[A1:A10]` auf einem anderen Blatt (`A1:A10`, nach Excel-Art eingegeben, wird akzeptiert);
- eine benannte Zelle.

**Auswählen** blendet das Dialogfeld aus, während Sie auf den Bereich zeigen (Klicken, Ziehen oder `Umschalt+Pfeiltasten`; ein Klick auf die Überschrift einer Tabellenspalte übernimmt die ganze Spalte), bei Bedarf auf einem anderen Blatt: Klicken Sie auf sein Register, der Bezug wird zu `Tabelle2!Cell[A1:A10]`. `Eingabe` übernimmt ihn, `Esc` kehrt zurück, ohne ihn zu ändern; in beiden Fällen kommen Sie zum ursprünglichen Blatt und zur ursprünglichen Auswahl zurück. Der Titel des Dialogfelds nennt, wofür die Überprüfung gilt (Tabellenspalte, Zellen), und das Dialogfeld zeigt eine Vorschau der Werte.

Die Liste eines Bereichs wird bei jedem Öffnen der Dropdownliste und bei jeder Eingabe neu gelesen: eindeutige Werte, in der Reihenfolge des Bereichs, ohne leere Zellen. Der Bezug folgt dem Umbenennen der Tabelle, der Spalte oder des Blatts sowie dem Einfügen oder Löschen von Zeilen und Spalten. Verschwindet die Quelle (Tabelle gelöscht, Zeilen des Bereichs gelöscht), melden die Dropdownliste und das Dialogfeld es, und Eingaben werden abgelehnt, bis die Überprüfung korrigiert ist.

### Bezugsspalten: von einer Tabelle zur anderen wechseln {#colonnes-de-reference-basculer-d-un-tableau-a-l-autre}

Ist der Bereich die **Schlüsselspalte** einer anderen Tabelle (die mit 🔑 markierte, zum Beispiel `Arten[Name]`), wird die überprüfte Spalte zu einem **Bezug** auf diese Tabelle, wie ein Fremdschlüssel: Die Statusleiste meldet es.

- **Wechseln zu**: `Strg+Klick` auf eine Zelle der Spalte, die kleine Schaltfläche ↗ neben ihrem Listenpfeil oder Rechtsklick › Wechseln zu „Arten“ › Elster springt zur referenzierten Zeile (auf ihrem Blatt), die kurz aufleuchtet. Gibt es den Wert in der Zieltabelle nicht, ist der Eintrag abgeblendet und meldet es.
- **Zurück**: `Alt+←` oder „← Zurück zu…“ in der Statusleiste bringt Sie zur Ausgangszelle zurück (mehrere Sprünge werden einzeln zurückgenommen).
- **Zeilen, die darauf verweisen**: Rechtsklick auf eine Zelle der Schlüsselspalte der Zieltabelle › Zeilen, die auf „Elster“ verweisen (3) filtert die verweisende Tabelle auf diesen Wert und springt zu ihrer ersten Zeile.

Sonst wird nichts gespeichert: Das Entfernen der Überprüfung (oder `Strg+Z`) oder des Schlüssels der Zieltabelle hebt den Bezug auf, und er folgt dem Umbenennen der Tabelle und ihrer Schlüsselspalte. Fehlende Werte werden nicht durch eine Datenbankeinschränkung blockiert: Es ist die Überprüfung, die sie bei der Eingabe ablehnt.

Überprüfungen werden weder nach Excel (.xlsx) exportiert noch beim Import gelesen.

## Notizen {#notes}

Überprüfen › Neue Notiz (`Umschalt+F2`) heftet der aktiven Zelle eine Notiz an: Eine rote Ecke kennzeichnet sie, und sie erscheint beim Darüberfahren. Auf einer Zelle, die bereits eine hat, wird der Befehl zu **Notiz bearbeiten**. Die Stecknadel der Notiz hält sie eingeblendet. Überprüfen › Notiz löschen entfernt sie.

## Fenster fixieren {#figer-les-volets}

Ansicht › Fenster fixieren hält Zeilen und Spalten beim Scrollen auf dem Bildschirm:

- **Fenster fixieren**: die Zeilen oberhalb und die Spalten links der aktiven Zelle;
- **Oberste Zeile fixieren** oder **Erste Spalte fixieren**;
- **Fixierung aufheben** lässt sie wieder scrollen.

Die Einstellung gehört zum Blatt und wird mit der Arbeitsmappe gespeichert.

## Farbdesign {#theme-de-couleur}

Ansicht › Design wählt die Farbe der Tabellen der Arbeitsmappe: die der gewöhnlichen Tabellen (standardmäßig grün) und die der Tabellen, die aus einer externen Quelle oder von einer Abfrage geladen werden (standardmäßig blau). Jedes Design im Menü zeigt seine zwei Farben; Überschrift, deren Text, Rahmen, Symbole und Namensschild werden daraus abgeleitet, mit einem Text, der auf der Überschrift immer lesbar ist.

- **Anpassen…**: zwei Farben Ihrer Wahl, mit sofortiger Vorschau im Raster, dann die übrigen Farben der Palette (Akzent 3 bis 6, Text 2, Hintergrund 2), aus den ersten beiden abgeleitet, wenn Sie sie nicht ändern; **Zu meinen Designs hinzufügen** behält es in der Liste für Ihre anderen Arbeitsmappen (im selben Fenster wieder entfernbar).
- Jedes Design hat seine **Palettenfarben** (siehe Schrift- und Füllfarben): Mit einer Designfarbe gefärbte Zellen ändern sich mit ihm. Eine Arbeitsmappe ohne eigenes Design behält das Standarddesign, sobald darin eine Designfarbe verwendet wird, damit die Farbe für eine Kollegin gleich bleibt.
- Das Design wird mit der Arbeitsmappe gespeichert (`Strg+Z` macht es rückgängig): Ein Kollege öffnet sie mit denselben Farben, auch ohne dieses Design. Eine auf einem Lattis-Server geteilte Arbeitsmappe veröffentlicht es mit ihrem Layout.
- Eine Arbeitsmappe, die kein Design gewählt hat (neue Arbeitsmappe oder eine aus der Zeit vor den Designs), übernimmt das **Standarddesign** aus Datei › Einstellungen › Ansicht.

Ihre Organisation kann eigene Designs anbieten, nur diese anbieten oder das Standarddesign vorgeben. Designfarben stehen auch als CSS-Variablen zur Verfügung (`--lattis-table-head`, `--lattis-source-head`…).

## Schützen {#proteger}

Überprüfen › Blatt schützen lehnt jede Änderung an seinem Inhalt ab; Sie können sich weiterhin bewegen, auswählen, kopieren, suchen, filtern und drucken. Überprüfen › Struktur schützen sperrt alles außer der Eingabe in Tabellen: Sie geben ein, fügen ein (nur Werte), löschen Werte in deren Datenzellen, fügen durch Eingabe darunter Zeilen hinzu, fügen Zeilen ein oder löschen sie, sortieren und filtern; freie Zellen, Formatierung, Spalten und Überschriften, die Tabellen selbst, Objekte, Namen, Regeln, Zeilen und Spalten des Blatts, Seitenlayout und Register werden abgelehnt. Überprüfen › Arbeitsmappe schützen lehnt das Hinzufügen, Löschen, Umbenennen, Verschieben oder Ausblenden von Blättern ab; Sie können wählen, zusätzlich die Struktur aller Blätter (nur Eingabe in Tabellen) oder ihren Inhalt zu sperren. Abgelehnte Befehle sind abgeblendet, und ihre QuickInfo nennt, wie der Schutz aufgehoben wird (siehe [Erste Schritte](prise-en-main#proteger-une-feuille-ou-le-classeur)).

In einer auf einem Lattis-Server geteilten Arbeitsmappe blendet eine veröffentlichte Tabelle, die Sie aufgrund Ihrer Rechte nicht ändern dürfen (Rolle Leser, eingefrorene Arbeitsmappe, offline), ebenso die Befehle ab, die ihre Daten ändern würden; Formatierung, Sortieren und Filter bleiben erlaubt, da sie nur auf Ihrem Computer gelten.

In einer kollaborativen Arbeitsmappe ändern Besitzer und Designer die **Struktur** einer geteilten Tabelle wie die einer lokalen Tabelle: eine Spalte hinzufügen, umbenennen, löschen, ihren Typ ändern, den Schlüssel wählen, die Tabelle umbenennen oder löschen. Die Änderung wird zuerst auf dem Server vorgenommen und dann auf dem Computer angewendet, und die anderen erhalten sie sofort (ihre Tabelle wird neu gelesen; der Rückgängig-Verlauf, der sie betraf, wird geleert). Der Server kann ablehnen, und seine Meldung erscheint in der Statusleiste: Eine Spalte, auf die der Filter einer Rolle, eine Einschränkung oder die Zielgruppe eines Datenbestands verweist, lässt sich nicht umbenennen, löschen oder umwandeln; eine Tabelle, die den Data Lake oder eine Pipeline speist, lässt sich nicht löschen. `Strg+Z` macht die Änderung auch auf dem Server rückgängig (lehnt er ab, bleibt der Schritt rückgängig zu machen); eine gelöschte und dann wiederhergestellte Tabelle kommt nur auf Ihrem Computer zurück, als lokale Tabelle. Die anderen Mitglieder (Bearbeiter, Teilnehmer) ändern die Struktur nicht, Sie müssen online sein, und Server-Indizes werden mit den Befehlen `tables.add_index` und `tables.drop_index` festgelegt (Skripte, Assistent).
