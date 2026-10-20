# Lua-Skripte

Die Registerkarte **Skript** automatisiert die Arbeitsmappe in Lua (Version 5.1): Tabellen lesen und schreiben, Blätter steuern, dem Benutzer Fragen stellen, auf Steuerelemente reagieren.

## Die Registerkarte Skript {#l-onglet-script}

- **Links die Bibliothek**: die in der Arbeitsmappe gespeicherten Skripte (sie reisen mit der `.lattis`-Datei). Rechtsklick: umbenennen, duplizieren, löschen.
- **In der Mitte der Editor**, der das Skript während der Eingabe prüft: Syntaxfehler in Rot, unbekannte Namen und falsche Anzahl von Argumenten in Gelb.
- **Rechts die Referenz**: die unten aufgeführten Funktionen, mit einer Schaltfläche, um ihr Beispiel einzufügen.
- **Unten die Ausgabe**: was `print` anzeigt, dann die Zusammenfassung der Ausführung.

`Strg+Eingabe` führt das Skript aus, `Strg+S` speichert es in der Arbeitsmappe (denken Sie dann daran, die Arbeitsmappe zu speichern).

## Lesen und schreiben {#lire-et-ecrire}

Abfragen werden in SQL mit den angezeigten Namen der Tabellen und Spalten geschrieben; ein Name mit einem Leerzeichen oder einem Akzent steht in doppelten Anführungszeichen:

```
for r in db.rows('SELECT Kunde, sum("Betrag netto") AS summe FROM Bestellungen GROUP BY Kunde') do
  print(r.Kunde, r.summe)
end
```

`db.insert`, `db.update` und `db.delete` schreiben wie eine Eingabe im Raster. Alles, was ein Skript schreibt, bildet **einen einzigen Rückgängig-Schritt**: `Strg+Z` nimmt es auf einmal zurück.

## Sicherheit und Grenzen {#securite-et-limites}

Ein Skript läuft in einer Sandbox: kein Zugriff auf Dateien, Programme oder Umgebungsvariablen. HTTP-Anfragen (`http.get`…) erreichen nur die Websites, die Sie zugelassen haben. Ein Skript wird nach 30 Sekunden, ab 512 MB Arbeitsspeicher oder mit der Schaltfläche Stopp angehalten. `db.query` gibt höchstens 100.000 Zeilen zurück (`db.rows` hat keine Grenze).

Ein mit einer Arbeitsmappe erhaltenes Skript (Schaltfläche, Steuerelement, Ereignis) bittet vor seiner ersten Ausführung um Ihre Zustimmung: **Zulassen** oder **Ablehnen**, mit **Code anzeigen**. Ein Skript kann die ganze Arbeitsmappe lesen und ändern: Lassen Sie es nur zu, wenn Sie der Person vertrauen, die Ihnen diese Arbeitsmappe gegeben hat. Auf diesem Computer geschriebene Skripte werden automatisch zugelassen.

Beim Öffnen löscht Lattis aus der Arbeitsmappe die SQL-Trigger (`TRIGGER`), die es nicht selbst angelegt hat (Lattis speichert nie welche in der Datei), und meldet es in der Statusleiste.

## Schaltflächen und Steuerelemente {#boutons-et-controles}

Eine Funktion eines Skripts wird mit einer Schaltfläche oder einem Steuerelement auf dem Blatt verknüpft: siehe [Schaltflächen und Steuerelemente](controles).

## Ereignisse der Arbeitsmappe {#evenements-du-classeur}

Ein Skript kann auf das reagieren, was Sie in der Arbeitsmappe tun: `events.on(Ereignis, Funktion, Skript)` abonniert eine Funktion eines gespeicherten Skripts, und das Abonnement reist mit der Datei.

```
function eingabe(e)
  ui.notify(e.address .. ": " .. tostring(e.old) .. " → " .. tostring(e.value))
end
events.on("change", "eingabe", "Mein Skript")
```

| Ereignis | Wann |
|---|---|
| `open` | die Arbeitsmappe wurde gerade geöffnet |
| `before_save` / `save` | vor / nach dem Speichern (`false` zurückgeben bricht das Speichern ab) |
| `before_close` | bevor die Arbeitsmappe oder das Fenster geschlossen wird (`false` zurückgeben bricht es ab) |
| `sheet` | ein anderes Register wird angezeigt |
| `selection` | die Auswahl hat sich geändert (sobald sie nicht mehr bewegt wird) |
| `change` | Zellen wurden geändert: Eingabe, Einfügen, Löschen, Verschieben, Tabellenzeile |
| `sheet_add` / `sheet_delete` | ein Blatt wird hinzugefügt / gelöscht |
| `recalc` | Formeln werden neu berechnet (F9) |

Die Funktion erhält eine Tabelle: `sheet`, `row`, `col`, `address` („B3“), `value` und `old` für eine einzelne eingegebene Zelle… Nur Ihre Aktionen lösen diese Ereignisse aus, nie ein Skript: Ein Listener, der auf das Blatt schreibt, löst sich nicht erneut aus. Listener laufen, ohne das Raster zu blockieren, was sie ändern, wird auf einmal rückgängig gemacht (`Strg+Z`), und ein Fehler wird nur einmal gemeldet. `events.enable(false)` schaltet sie für die Sitzung ab.

## Funktionsreferenz {#reference-des-fonctions}
