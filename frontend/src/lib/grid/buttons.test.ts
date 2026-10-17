import { describe, expect, it } from 'vitest';
import { scriptFunctions } from './buttons';

describe('boutons d’action', () => {
	it('liste les fonctions globales sans argument d’un script', () => {
		const src = `-- function commentee()
local function aide() end
function relancer()
  print("x")
end
function M.methode() end
function   archiver ( ) end
function avec(a, b) end
--[[ function bloc() end ]]
function relancer() end`;
		expect(scriptFunctions(src)).toEqual(['relancer', 'archiver']);
	});
});
