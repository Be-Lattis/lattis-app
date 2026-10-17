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
			hint: 'Commande à lancer dans un terminal.',
			text: `claude mcp add --transport http lattis ${s.url} --header "Authorization: Bearer ${s.token}"`
		},
		{
			label: 'Clients HTTP (Streamable HTTP)',
			hint: 'Pour les clients qui se connectent à une adresse.',
			text: JSON.stringify(http, null, 2)
		}
	];
}
