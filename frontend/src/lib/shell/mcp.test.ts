import { describe, expect, it } from 'vitest';
import { mcpConfigs } from './mcp';

describe('configurations MCP', () => {
	it('relais stdio, Claude Code et HTTP', () => {
		const c = mcpConfigs({ url: 'http://127.0.0.1:47390/mcp', token: 'abc', command: 'C:\\Lattis\\lattis.exe', args: ['mcp'] });
		expect(JSON.parse(c[0].text).mcpServers.lattis).toEqual({ command: 'C:\\Lattis\\lattis.exe', args: ['mcp'] });
		expect(c[1].text).toBe('claude mcp add --transport http lattis http://127.0.0.1:47390/mcp --header "Authorization: Bearer abc"');
		expect(JSON.parse(c[2].text).mcpServers.lattis.headers.Authorization).toBe('Bearer abc');
	});
});
