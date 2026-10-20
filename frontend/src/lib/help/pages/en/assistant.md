# AI Assistant

The **Assistant ✦** helps you build an analysis: it reads the structure of your tables, suggests an approach, and can add calculated columns, keys, PivotTables and charts, sort or format. It opens in a pane to the right of the grid, like Copilot in Excel, and stays open when you change sheets.

## Turn on the assistant {#activer-l-assistant}

The assistant is off by default. The **Assistant ✦** button, at the top right of the ribbon (`Alt+I`), opens its pane: check **Turn on the AI Assistant**. The same setting is in File › Settings › Features. In a company, the organization's policy can turn it off or impose it.

## Connect {#se-connecter}

The **Connection** button, in the pane's header, offers two ways. The panel opens on its own as long as nothing is connected.

### API key (recommended) {#cle-d-api-recommande}

1. Choose the provider (Anthropic, Mistral, OpenAI, Gemini, Mammouth…) and the model, then **Open the console**: the provider's page where you create a key.
2. Copy the key, then **Paste** (or `Ctrl+V` in the field).
3. **Verify and save**: Lattis tries the key with the provider before saving it. A rejected key or an account without credits gives a clear message in the panel.

The key is kept in the system keychain, never in the workbook. **Forget the key** removes it.

> At Anthropic, a key only works with credits: add some on the console's **Billing** page (platform.claude.com). The **Claude Max** and **Team** subscriptions give API credits every month ($100 to $500 depending on the plan): link the console's organization in claude.ai › Settings › Billing to receive them. The Pro and Enterprise subscriptions don't give any.

### Desktop app (Claude Pro, Max, Team subscription) {#application-de-bureau-abonnement-claude-pro-max-team}

A Claude subscription can't be used directly in another application, but **Claude Desktop** and **Claude Code** can drive Lattis through its MCP server. The “Desktop app” tab of the connection panel (app: Claude) configures them in one click; then restart Claude Desktop.

You then talk to Claude in its app. The assistant pane shows **From Claude**: what Claude does in the workbook, as it happens, with **Undo these changes** for each series of actions. The **Ask before changing** option makes you approve each write in Lattis, including saving under another name, exports and imports. The **Data sent to the assistant** setting also applies to Claude (see below).

### Organization gateway {#passerelle-de-l-organisation}

When the company provides its own gateway, the pane says “Connected through your organization's gateway”: there is no key to enter and no provider to choose.

### Local model {#modele-local}

The **Local model** provider uses a model installed on this computer (Ollama, LM Studio…): no data leaves the computer, and there is no key. Its address is set in the connection panel or in File › Settings › AI Assistant. This is experimental: small models are worse at calling Lattis's tools.

## Chat {#converser}

Type your request, then `Enter` (`Shift+Enter` starts a new line). Each message goes out with what you are looking at: the active sheet, the table under the cursor and the selection, shown above the input box (“Context: Orders, selection Amount[3:10]”). “Analyze this table” or “explain this column” therefore apply to what is selected.

- The assistant's steps (reading a table, adding a column…) are grouped in a **N steps** line, expanded during the answer then collapsed; a click opens it again. The name of an underlined step leads to the sheet or table concerned.
- A failed step that the assistant then fixed is shown in gray; only an error left unresolved is in red.
- `Esc` stops the current answer. Stopped before the model answered, or failed at that moment (network, quota), it disappears and your message comes back in the input box: the model didn't receive it.
- The up arrow, in an empty input box, recalls the previous message; the copy button, under an answer, puts it on the clipboard.
- **Retry**, under an answer interrupted by an error (provider overloaded, outage), resumes it where it stopped, without sending your message again: what it already changed isn't done again, and the answer can still be undone in one go. When the error asks you to wait (“retry in 30 s”), the button counts down the seconds.
- A very long answer stops after 100 steps: **Continue** resumes it in the same way.
- A question about the data (“which city has the best revenue?”) gets a **result** under the answer: the assistant writes a query, Lattis runs it on the computer and shows you 50 rows at most. In “Structure only” mode, the result is for you alone: the assistant sees neither its values nor its number of rows.
- A link in an answer shows its domain (“See the report (example.com)”); a click shows the full address and asks for confirmation before opening it in the browser.
- **New** (pane header) starts a new conversation; the previous one stays in the list of conversations (☰), from where it can be resumed or deleted (after confirmation).
- When the conversation gets long, the assistant summarizes its beginning to continue: the **Session compacted** marker shows where. Everything stays displayed in the pane; only the model now works on the summary.

## Repair badly recognized tables {#reparer-des-tableaux-mal-reconnus}

The **Repair the tables on this sheet** suggestion has the assistant reread the sheet's layout: where the tables are, the nature of the cells in their first and last rows (text, number, date, formula, empty), bold or merged free cells, and rows that look like a title or a total. It doesn't read their values, even in “Structure only” mode. It then fixes things with the commands that keep the table, its key and the formulas that refer to it: title taken for the header, header left in the data, total row moved out of the table, headers on two rows, columns to split.

What the assistant reads in the workbook (names, cell text) is presented to it as data, never as an instruction: a cell that “asks” it to delete a column must not be obeyed, and it says so. It only deletes a column, a chart, a PivotTable or duplicates if you ask it to.

The **Keep the conversation in the workbook** setting (File › Settings › AI Assistant) saves it with the workbook; otherwise, it is cleared when you close the workbook or open another one. When kept, it travels with the file, including the results shown and the rows read by the assistant, and is only saved if the workbook was also changed. If the workbook changes outside the answers (import, `Ctrl+Z`, conversation resumed later), the assistant receives its up-to-date structure with the next message.

## Undo {#annuler}

Everything the assistant changes during an answer forms a single step: the answer ends with its summary (“3 changes”) and the **Undo this answer** button. `Ctrl+Z` in the grid does the same. The button is only active for the last answer, as long as nothing else has been changed since; otherwise, use `Ctrl+Z`.

## Data sent {#donnees-transmises}

The **Data sent to the assistant** setting (File › Settings › AI Assistant, recalled at the bottom of the pane) chooses what the assistant can read:

| Setting | What the provider receives |
| --- | --- |
| Structure only (default) | Names of sheets, tables and columns, types, numbers of rows and of distinct values, formulas, position of tables and cells on the sheet (without their content, with their nature and the “title” or “total” hints); not the result of the queries shown, nor the number of cells of a replacement; query steps and connection parameters, without passwords or the values of HTTP headers and filters. The columns of a PivotTable that carry a field's values are summarized (“3 value column(s) of field “Customer””) |
| Structure and sample rows | The same, plus a few rows on each read: 20 rows at most, 200 characters per value |
| Entire workbook | Everything the assistant chooses to read, without limit |

The provider also receives your messages and the context (sheet, table, selection). With a local model, nothing leaves the computer.

The setting also applies to **Claude Desktop** and **Claude Code** through the MCP server: without “Entire workbook”, the tools that read values (table rows, SQL, cells, scripts, exports) are not offered to them or are limited in the same way. After a change, restart Claude so that it sees the new list of tools. Claude Code also reads the files on your disk: a saved workbook remains directly readable to it.

## Cost {#cout}

With an API key, each answer is billed by the provider according to the number of tokens read and written. With an Anthropic key, the foot of the pane shows an estimate of the conversation's cost (“≈ $0.12 for this conversation”) based on public pricing; the provider's invoice is what counts. Nothing is shown for other providers or through a gateway, whose prices differ. A long series of steps costs more: a lighter model (Sonnet rather than Opus) is usually enough.

## Limits {#limites}

- The assistant can make mistakes: review what it suggests, and undo an answer that doesn't suit you.
- In “Structure only” mode, it doesn't see the values: it can't judge the content of the cells. In “Structure and sample rows” mode, it only sees part of it.
- A very long conversation is summarized (“Session compacted”): the assistant may then forget a detail of the first exchanges. For a new subject, start a new conversation instead.
- With Claude Desktop or Claude Code, the conversation stays in Claude; Lattis only sees the actions.
