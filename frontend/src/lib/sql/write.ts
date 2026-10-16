// Une requête qui écrit part au backend telle quelle (DocumentService.Execute) :
// il connaît les noms affichés et rend l'écriture annulable. Les autres sont
// traduites en noms internes et exécutées en lecture seule.

const writeKeyword = /^(insert|update|delete|replace)$/i;

/** isWrite indique si la requête commence par INSERT, UPDATE, DELETE ou REPLACE (après un éventuel WITH). */
export function isWrite(sql: string): boolean {
	const words = sql
		.replace(/--[^\n]*/g, ' ')
		.replace(/\/\*[\s\S]*?\*\//g, ' ')
		.replace(/'(?:[^']|'')*'|"(?:[^"]|"")*"/g, ' x ')
		.trim()
		.split(/[^A-Za-z_]+/)
		.filter(Boolean);
	if (words.length === 0) return false;
	if (writeKeyword.test(words[0])) return true;
	if (words[0].toLowerCase() !== 'with') return false;
	// WITH ... : la première instruction principale après les expressions de table.
	let depth = 0;
	const text = sql.replace(/'(?:[^']|'')*'|"(?:[^"]|"")*"/g, ' x ');
	for (const m of text.matchAll(/[()]|[A-Za-z_]+/g)) {
		if (m[0] === '(') depth++;
		else if (m[0] === ')') depth--;
		else if (depth === 0 && /^(select|values|insert|update|delete|replace)$/i.test(m[0])) return writeKeyword.test(m[0]);
	}
	return false;
}
