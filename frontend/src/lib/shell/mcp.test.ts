import { describe, expect, it } from 'vitest';
import { dropConfirm, mcpConfigs, pushConfirm } from './mcp';

describe('configurations MCP', () => {
	it('relais stdio, Claude Code et HTTP', () => {
		const c = mcpConfigs({ url: 'http://127.0.0.1:47390/mcp', token: 'abc', command: 'C:\\Lattis\\lattis.exe', args: ['mcp'] });
		expect(JSON.parse(c[0].text).mcpServers.lattis).toEqual({ command: 'C:\\Lattis\\lattis.exe', args: ['mcp'] });
		expect(c[1].text).toBe('claude mcp add --transport http --scope user lattis http://127.0.0.1:47390/mcp --header "Authorization: Bearer abc"');
		expect(JSON.parse(c[2].text).mcpServers.lattis.headers.Authorization).toBe('Bearer abc');
	});
});

describe("demandes d'accord", () => {
	it("file d'attente dans l'ordre d'arrivée", () => {
		const r = (id: number) => ({ id, client: 'Claude Desktop', label: `Action ${id}`, tool: 'sheets_add' });
		let q = pushConfirm([], r(1));
		q = pushConfirm(q, r(2));
		q = pushConfirm(q, r(1));
		expect(q.map((x) => x.id)).toEqual([1, 2]);
		expect(dropConfirm(q, 1).map((x) => x.id)).toEqual([2]);
		expect(dropConfirm(q, 9)).toHaveLength(2);
	});
});
