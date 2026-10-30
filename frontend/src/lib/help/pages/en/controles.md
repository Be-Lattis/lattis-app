# Buttons and controls

Like a spreadsheet's form controls, controls are placed freely on a sheet (or in a [document](onglets)): Insert › Button, or Insert › Field.

## Control types {#types-de-controles}

| Control | What it does |
| --- | --- |
| Button | A click runs the linked action |
| Check box | Writes TRUE or FALSE to the linked cell |
| Drop-down list | Writes the choice to the linked cell |
| Input box | Writes the text to the linked cell when leaving it, or with `Enter` |
| Text box | Like the input box, on several lines (`Ctrl+Enter` confirms) |
| File picker | Writes the path of the chosen file to the linked cell |

## The control pane {#le-volet-du-controle}

On insertion (or with right-click › Edit), the pane on the right sets:

- the control's **name**, for scripts (automatic if empty);
- its **label**;
- the **linked cell**: clicking the sheet fills it in while the field has the focus;
- the **action**: a function of a script from the library (Script tab).

A control can be created without an action and linked later. Everything an action changes is undone with a single `Ctrl+Z`.

## Working with a control {#manipuler-un-controle}

Drag to move it; `Ctrl+click` selects it to resize it; right-click: run, edit, cut, copy, delete.

## In a script {#dans-un-script}

Scripts read and listen to controls with `controls.list`, `controls.get` and `controls.on`: see [Lua scripts](lua).
