# Schaltflächen und Steuerelemente

Wie die Formularsteuerelemente einer Tabellenkalkulation werden Steuerelemente frei auf einem Blatt (oder in einem [Dokument](onglets)) platziert: Einfügen › Schaltfläche oder Einfügen › Feld.

## Arten von Steuerelementen {#types-de-controles}

| Steuerelement | Funktion |
| --- | --- |
| Schaltfläche | Ein Klick führt die verknüpfte Aktion aus |
| Kontrollkästchen | Schreibt WAHR oder FALSCH in die verknüpfte Zelle |
| Dropdownliste | Schreibt die Auswahl in die verknüpfte Zelle |
| Eingabefeld | Schreibt den Text beim Verlassen oder mit `Eingabe` in die verknüpfte Zelle |
| Textfeld | Wie das Eingabefeld, mehrzeilig (`Strg+Eingabe` bestätigt) |
| Dateiauswahl | Schreibt den Pfad der gewählten Datei in die verknüpfte Zelle |

## Der Bereich des Steuerelements {#le-volet-du-controle}

Beim Einfügen (oder mit Rechtsklick › Bearbeiten) legt der Bereich rechts Folgendes fest:

- den **Namen** des Steuerelements, für Skripte (automatisch, wenn leer);
- seine **Beschriftung**;
- die **verknüpfte Zelle**: Ein Klick auf das Blatt füllt sie aus, solange das Feld den Fokus hat;
- die **Aktion**: eine Funktion eines Skripts aus der Bibliothek (Registerkarte Skript).

Ein Steuerelement kann ohne Aktion erstellt und später verknüpft werden. Alles, was eine Aktion ändert, lässt sich mit einem einzigen `Strg+Z` rückgängig machen.

## Mit einem Steuerelement arbeiten {#manipuler-un-controle}

Ziehen, um es zu verschieben; `Strg+Klick` wählt es aus, um seine Größe zu ändern; Rechtsklick: ausführen, bearbeiten, ausschneiden, kopieren, löschen.

## In einem Skript {#dans-un-script}

Skripte lesen Steuerelemente und reagieren auf sie mit `controls.list`, `controls.get` und `controls.on`: siehe [Lua-Skripte](lua).
