// Configurations à copier dans les clients MCP (Fichier › Serveur MCP).

export interface McpInfo {
	url: string;
	token: string;
	command: string;
	args: string[] | null;
}

export interface McpConfig {
	label: string;
	hint: string;
	text: string;
}

export function mcpConfigs(s: McpInfo): McpConfig[] {
	const desktop = { mcpServers: { lattis: { command: s.command, args: s.args ?? ['mcp'] } } };
	const http = { mcpServers: { lattis: { type: 'http', url: s.url, headers: { Authorization: `Bearer ${s.token}` } } } };
	return [
		{
			label: 'Claude Desktop (et clients stdio)',
			hint: 'À ajouter dans claude_desktop_config.json : le client lance Lattis en relais vers l’application ouverte.',
			text: JSON.stringify(desktop, null, 2)
		},
		{
			label: 'Claude Code',
			hint: 'Commande à lancer dans un terminal (portée user : Lattis est proposé dans tous vos projets).',
			text: `claude mcp add --transport http --scope user lattis ${s.url} --header "Authorization: Bearer ${s.token}"`
		},
		{
			label: 'Clients HTTP (Streamable HTTP)',
			hint: 'Pour les clients qui se connectent à une adresse.',
			text: JSON.stringify(http, null, 2)
		}
	];
}

/** Demande d'accord d'un client MCP avant une écriture (évènement mcp:confirm). */
export interface McpConfirmRequest {
	id: number;
	/** Nom affiché du client (« Claude Desktop »). */
	client: string;
	label: string;
	tool: string;
}

/** Ajoute une demande à la file (une demande déjà là n'est pas doublée). */
export function pushConfirm(queue: readonly McpConfirmRequest[], r: McpConfirmRequest): McpConfirmRequest[] {
	return queue.some((q) => q.id === r.id) ? [...queue] : [...queue, r];
}

/** Retire une demande close (réponse donnée, délai dépassé : mcp:confirm-done). */
export function dropConfirm(queue: readonly McpConfirmRequest[], id: number): McpConfirmRequest[] {
	return queue.filter((q) => q.id !== id);
}
