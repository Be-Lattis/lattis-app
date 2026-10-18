import { describe, expect, it } from 'vitest';
import { lintLua } from './lualint';
import { reference, search } from './reference';

const messages = (src: string) => lintLua(src).map((d) => `${d.severity}: ${d.message}`);
const only = (src: string) => {
	const d = lintLua(src);
	expect(d).toHaveLength(1);
	return { ...d[0], text: src.slice(d[0].from, d[0].to) };
};

describe('syntaxe', () => {
	it('accepte les scripts valides', () => {
		for (const src of [
			'local t = {1, 2, 3,}\nprint(#t)',
			'local s = [==[ a ]] b ]==]\nprint(s)',
			'--[==[ long\ncommentaire ]==]\nprint(1)',
			'for i = 1, 3 do if i == 2 then goto continue end print(i) ::continue:: end',
			'local function f(...) return select("#", ...) end\nprint(f(1, 2))',
			'print(...)',
			'local t = {f = function(self) return self end}\nt:f()',
			'print"x" print{1}',
			'local x = -2 ^ 2 .. "a" .. 3',
			'repeat local x = 1 until x == 1',
			'local a = {}\nfunction a.b(x) return x end\nfunction a:c() return self end',
			'(print)("x")',
			'local x = 5 x = x + 1'
		])
			expect(messages(src), src).toEqual([]);
	});

	it('signale un end manquant avec la ligne du bloc ouvert', () => {
		const d = only('for i = 1, 3 do\n  if i > 1 then\n    print(i)\nend');
		expect(d.severity).toBe('error');
		expect(d.message).toBe('« end » attendu pour fermer « for » de la ligne 1, trouvé la fin du script');
	});

	it('signale un end en trop', () => {
		expect(only('print(1)\nend').message).toBe('« end » en trop : aucun bloc à fermer');
	});

	it('explique les erreurs courantes', () => {
		expect(only('if x != 1 then end').message).toContain('~=');
		expect(only('if x = 1 then end').message).toContain('utilisez ==');
		expect(only('local x = 1\nx += 1').message).toContain('x = x + valeur');
		expect(only('for i = 1, 3 do\n  continue\nend').message).toContain('continue');
		expect(only('local x = 7 // 2').message).toContain('math.floor');
		expect(only('if a && b then end').message).toContain('and');
		expect(only('print(“x”)').message).toContain('guillemet typographique');
	});

	it('refuse les accents dans les noms et propose la bonne écriture', () => {
		const d = only('local l = {}\nprint(l.Payé)');
		expect(d.text).toBe('Payé');
		expect(d.message).toContain('["Payé"]');
		expect(only('local payé = 1').message).toContain('sans accent');
	});

	it('signale un texte non terminé', () => {
		expect(only('print("abc)').message).toContain('texte non terminé');
		expect(only('local s = [[abc').message).toContain(']]');
	});

	it('suit les règles de gopher-lua', () => {
		expect(only('break').message).toContain('en dehors d’une boucle');
		expect(only('while true do break print(1) end').message).toContain('dernière instruction');
		expect(only('local function f() return ... end').message).toContain('...');
		expect(only('goto fin\nprint(1)').message).toContain('::fin::');
		expect(only('::a:: ::a::').message).toContain('déjà définie');
		expect(only('do goto a local x = 1 ::a:: print(x) end').message).toContain('portée de la variable locale « x »');
		expect(only('f() = 1').message).toContain('résultat d’un appel');
		expect(only('local goto = 1').message).toContain('mot réservé');
	});
});

describe('avertissements', () => {
	it('signale une fonction db inconnue et propose la bonne', () => {
		const d = only('local r = db.qury("SELECT 1")');
		expect(d.severity).toBe('warning');
		expect(d.text).toBe('qury');
		expect(d.message).toBe('db.qury n’existe pas : voulez-vous dire db.query ?');
	});

	it('signale ce que le bac à sable retire', () => {
		expect(only('local f = io.open("x")').message).toContain('« io » n’est pas disponible');
		expect(only('os.execute("ls")').message).toContain('os.execute n’est pas disponible');
		expect(only('local m = require("x")').message).toContain('« require » n’est pas disponible');
	});

	it('vérifie le nombre d’arguments', () => {
		const d = only('db.update("Commandes", {Client = "x"})');
		expect(d.text).toBe('db.update');
		expect(d.message).toBe('db.update attend 3 arguments, 2 donnés : db.update(tableau, _id, { colonne = valeur })');
		expect(only('db.query()').message).toContain('au moins 1');
		expect(only('print(db.tables("x"))').message).toContain('db.tables attend 0');
		// Le dernier argument est un appel : il peut fournir plusieurs valeurs.
		expect(messages('db.update(unpack({"T", 1, {}}))')).toEqual([]);
		expect(messages('db.query("SELECT ? + ?", 1, 2)')).toEqual([]);
	});

	it('signale db:query au lieu de db.query', () => {
		expect(only('local r = db:query("SELECT 1")').message).toContain('avec un point');
	});

	it('signale un nom jamais défini, avec une suggestion', () => {
		const d = only('prnt("x")');
		expect(d.text).toBe('prnt');
		expect(d.message).toBe('« prnt » n’est pas défini : voulez-vous dire « print » ?');
		expect(only('for i, v in ipair({}) do end').message).toContain('« ipairs »');
	});

	it('ne signale pas les locales, paramètres et globales affectées', () => {
		expect(messages('local function f(a, ...) local b = a return b, ... end\nprint(f(1))')).toEqual([]);
		expect(messages('for _, t in ipairs(db.tables()) do print(t.name) end')).toEqual([]);
		expect(messages('local t = setmetatable({}, {}) function t:m() return self end')).toEqual([]);
		expect(messages('local db = {}\ndb.qury = 1\nprint(db.qury)')).toEqual([]);
	});

	it('conseille local pour une variable globale', () => {
		const d = only('total = 0\ntotal = total + 1');
		expect(d.severity).toBe('info');
		expect(d.text).toBe('total');
		expect(d.message).toContain('local total');
		expect(only('function aide() end\naide()').message).toContain('local function aide');
	});

	it('signale le remplacement d’un nom prédéfini', () => {
		const d = only('print = nil');
		expect(d.severity).toBe('warning');
		expect(d.message).toContain('remplace « print »');
	});

	it('signale un ancien nom', () => {
		expect(only('local n = table.getn({})').message).toContain('#t');
	});

	it('n’avertit pas sur les exemples de la référence', () => {
		for (const e of reference) if (e.example) expect(messages(e.example), e.name).toEqual([]);
	});
});

describe('recherche dans la référence', () => {
	it('trouve par nom, sans tenir compte des accents', () => {
		expect(search('query')[0].name).toBe('db.query');
		expect(search('insert')[0].name).toBe('db.insert');
		expect(search('majuscules').map((e) => e.name)).toContain('string.upper');
		expect(search('DATE')[0].name).toBe('os.date');
		expect(search('zzz')).toEqual([]);
	});
});

describe('commandes de l’application (api.gen.ts)', () => {
	it('connaît sheets, ui, buttons…', () => {
		expect(messages('sheets.add("Synthèse")\nui.select(1, 1, 3, 2)\nlocal s = ui.selection()\nprint(s.sheet)\nbuttons.create{row = 1, col = 1, label = "Go", script = "X"}')).toEqual([]);
		expect(only('sheets.ajouter("x")').text).toBe('ajouter');
		expect(only('ui.select()').message).toMatch(/argument/);
	});

	it('connaît les évènements du classeur et la variable event', () => {
		expect(messages('function saisie(e) print(e.address, event.value) end\nevents.on("change", saisie)\nevents.enable(false)\nprint(#events.list())')).toEqual([]);
		expect(reference.find((e) => e.name === 'events.on')?.category).toBe('events');
		expect(only('events.on()').message).toMatch(/argument/);
	});
});
