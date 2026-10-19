import { describe, expect, it } from 'vitest';
import {
	PALETTE,
	DARK_BG,
	buildDefinition,
	checks,
	contrast,
	copiedLabel,
	draftFromForm,
	listFor,
	newDraft,
	pagesOf,
	paramName,
	positionColumns,
	prefillURL,
	qrSVG,
	reasonAction,
	reducedMarkdown,
	unitOf,
	widgetFor,
	widgetsFor,
	type FormColumn,
	type FormInfo,
	type ServerForm
} from './forms';

const col = (name: string, type: string, more: Partial<FormColumn> = {}): FormColumn => ({
	id: 1,
	name,
	columnId: `c-${name}`,
	type,
	key: false,
	auto: false,
	avgLength: 10,
	format: '',
	list: null,
	...more
});

const info = (columns: FormColumn[], more: Partial<FormInfo> = {}): FormInfo => ({
	reason: '',
	message: '',
	table: 'Nids',
	server: 'https://lattis.example.test',
	serverName: 'Serveur',
	workbookId: 'wb',
	tableId: 't',
	public: false,
	columns,
	choice: null,
	restricted: [],
	forms: [],
	teams: [],
	...more
});

const day = new Date(2026, 9, 19);

describe('correspondance des types (§3.1)', () => {
	it('propose un champ par type de colonne', () => {
		expect(widgetFor(col('Nom', 'text'))).toBe('text');
		expect(widgetFor(col('Remarque', 'text', { avgLength: 120 }))).toBe('textarea');
		expect(widgetFor(col('Œufs', 'integer'))).toBe('number');
		expect(widgetFor(col('Prix', 'decimal'))).toBe('number');
		expect(widgetFor(col('Le', 'date'))).toBe('date');
		expect(widgetFor(col('À', 'datetime'))).toBe('datetime');
		expect(widgetsFor(col('Payé', 'boolean')).map((w) => w.value)).toEqual(['checkbox', 'yesno']);
	});
	it('liste : boutons radio jusqu’à 7 valeurs, liste avec recherche au-delà', () => {
		const list = (n: number) => ({ values: Array.from({ length: n }, (_, i) => `v${i}`), fixed: true, source: '', tableId: '', columnId: '', error: '' });
		expect(widgetFor(col('Statut', 'text', { list: list(7) }))).toBe('radio');
		expect(widgetFor(col('Commune', 'text', { list: list(8) }))).toBe('list');
		expect(widgetFor(col('Espèce', 'text', { list: { ...list(2), fixed: false, tableId: 't2', columnId: 'c2' } }))).toBe('list');
	});
	it('unité tirée du format de nombre', () => {
		expect(unitOf('# ##0,00 €')).toBe('€');
		expect(unitOf('%')).toBe('%');
		expect(unitOf('0,00')).toBe('');
	});
});

describe('listes', () => {
	const base = { values: ['Pie', 'Merle'], fixed: false, source: '', tableId: '', columnId: '', error: '' };
	it('copie une liste fixe', () => {
		expect(listFor(col('S', 'text', { list: { ...base, fixed: true } }), '2026-10-19')).toEqual({ values: ['Pie', 'Merle'] });
	});
	it('cite la colonne d’un tableau publié', () => {
		expect(listFor(col('E', 'text', { list: { ...base, source: 'Espèces[Nom]', tableId: 't2', columnId: 'c2' } }), '2026-10-19')).toEqual({ table_id: 't2', column: 'c2' });
	});
	it('copie une autre plage, avec sa date', () => {
		expect(listFor(col('E', 'text', { list: { ...base, source: 'Cell[A1:A10]' } }), '2026-10-19')).toEqual({ values: ['Pie', 'Merle'], copied_at: '2026-10-19', source: 'Cell[A1:A10]' });
		expect(copiedLabel('2026-10-19')).toBe('Liste copiée le 19/10');
	});
});

describe('brouillon et définition (§10.2)', () => {
	const cols = [
		col('ID', 'integer', { key: true, auto: true }),
		col('Commune', 'text'),
		col('Espèce', 'text', { list: { values: ['Pie'], fixed: false, source: 'Espèces[Nom]', tableId: 't2', columnId: 'c2', error: '' } }),
		col('Le', 'date'),
		col('Année', 'integer'),
		col('Latitude', 'decimal'),
		col('Longitude', 'decimal')
	];
	it('clé numérotée posée par le serveur, absente du formulaire', () => {
		const d = newDraft(info(cols), day);
		const def = buildDefinition(d);
		expect(def.format).toBe(1);
		expect(def.fields.some((f) => f.column === 'c-ID')).toBe(false);
		expect(def.server_values).toContainEqual({ column: 'c-ID', value: { token: 'next_number' } });
		expect(def.fields.find((f) => f.column === 'c-Le')?.default).toEqual({ token: 'today' });
		expect(def.fields.find((f) => f.column === 'c-Espèce')?.list).toEqual({ table_id: 't2', column: 'c2' });
	});
	it('position : un champ pour Latitude et Longitude', () => {
		expect(positionColumns(cols)).toEqual({ latitude: 'c-Latitude', longitude: 'c-Longitude' });
		expect(positionColumns([col('Position GPS', 'text')])).toEqual({ column: 'c-Position GPS' });
		const def = buildDefinition(newDraft(info(cols), day));
		expect(def.fields.filter((f) => f.column === 'c-Latitude')).toEqual([]);
		expect(def.fields.find((f) => f.kind === 'position')).toMatchObject({ latitude: 'c-Latitude', longitude: 'c-Longitude' });
	});
	it('clé saisie obligatoire, textes vides retirés', () => {
		const d = newDraft(info([col('Code', 'text', { key: true })]), day);
		const f = buildDefinition(d).fields[0];
		expect(f).toMatchObject({ column: 'c-Code', label: 'Code', required: true, widget: 'text', param: 'code', max_length: 10000 });
		expect('help' in f).toBe(false);
	});
	it('onglet à choix : valeur fixe, année en cours ou champ', () => {
		const i = info(cols, { choice: { column: 'Année', value: '2025' } });
		let d = newDraft(i, day);
		expect(d.choice?.mode).toBe('fixed');
		let def = buildDefinition(d);
		expect(def.server_values).toContainEqual({ column: 'c-Année', value: { fixed: '2025' } });
		expect(def.fields.some((f) => f.column === 'c-Année')).toBe(false);
		d = newDraft(info(cols, { choice: { column: 'Année', value: '2026' } }), day);
		expect(d.choice?.mode).toBe('year');
		def = buildDefinition(d);
		expect(def.fields).toContainEqual({ column: 'c-Année', hidden: true, default: { token: 'year' } });
		d.choice!.mode = 'field';
		expect(buildDefinition(d).fields.find((f) => f.column === 'c-Année')?.widget).toBe('number');
	});
	it('reprend un formulaire du serveur', () => {
		const d = newDraft(info(cols), day);
		d.appearance.title = 'Relevé';
		d.fields[1].help = 'Commune du nid';
		const def = buildDefinition(d);
		const form: ServerForm = { id: 'f', code: 'ABC', url: 'https://x/f/ABC', table_id: 't', title: 'Relevé', state: 'draft', pause_reason: '', access: 'audience', responses: 0, last_response_at: '', manageable: true, version: 3, definition: def };
		const back = draftFromForm(info(cols), form, day);
		expect(back.appearance.title).toBe('Relevé');
		expect(back.access).toBe('audience');
		expect(buildDefinition(back)).toEqual(def);
	});
});

describe('sections', () => {
	it('une section par page', () => {
		const fields = [{ column: 'a' }, { kind: 'section' as const, title: 'Où ?' }, { column: 'b' }, { column: 'c', hidden: true }, { kind: 'section' as const, title: 'Quoi ?' }, { column: 'd' }];
		expect(pagesOf(fields, false)).toHaveLength(1);
		const pages = pagesOf(fields, true);
		expect(pages.map((p) => p.length)).toEqual([1, 2, 2]);
	});
});

describe('palette', () => {
	it('8 couleurs au contraste AA, en clair et en sombre', () => {
		expect(PALETTE).toHaveLength(8);
		for (const p of PALETTE) {
			expect(contrast(p.light, '#ffffff'), p.name).toBeGreaterThanOrEqual(4.5);
			expect(contrast(p.dark, DARK_BG), p.name).toBeGreaterThanOrEqual(4.5);
		}
		expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 0);
	});
});

describe('Markdown réduit', () => {
	it('gras, italique, liens https, listes ; le reste échappé', () => {
		expect(reducedMarkdown('Une ligne **par nid**, *vite*.')).toBe('<p>Une ligne <strong>par nid</strong>, <em>vite</em>.</p>');
		expect(reducedMarkdown('[Guide](https://ex.fr/g)')).toContain('<a href="https://ex.fr/g" target="_blank"');
		expect(reducedMarkdown('[x](javascript:alert(1))')).not.toContain('<a');
		expect(reducedMarkdown('- un\n- deux')).toBe('<ul><li>un</li><li>deux</li></ul>');
		expect(reducedMarkdown('<b>x</b>')).toBe('<p>&lt;b&gt;x&lt;/b&gt;</p>');
	});
	it('remerciement : gras et italique seulement', () => {
		expect(reducedMarkdown('**Merci** [ici](https://ex.fr)', false)).toBe('<p><strong>Merci</strong> [ici](https://ex.fr)</p>');
	});
});

describe('vérifications (§3.5)', () => {
	it('liste vide, défaut hors liste, liste visible, restriction', () => {
		const cols = [col('Statut', 'text', { list: { values: ['Vu'], fixed: true, source: '', tableId: '', columnId: '', error: '' } }), col('Espèce', 'text', { list: { values: [], fixed: false, source: 'E[N]', tableId: 't2', columnId: 'c2', error: '' } })];
		const d = newDraft(info(cols), day);
		d.fields[0].default = { fixed: 'Couvé' };
		d.fields[0].required = true;
		const { errors, warnings } = checks(d, { columns: cols, restricted: ['Ajout interdit'] });
		expect(errors).toEqual([]);
		expect(warnings).toContain('Statut : valeur par défaut hors de la liste.');
		expect(warnings).toContain('Espèce : les valeurs de la liste seront visibles par tous les répondants.');
		expect(warnings.some((w) => w.startsWith('Statut est obligatoire'))).toBe(true);
		d.fields[0].list = { values: [] };
		expect(checks(d, { columns: cols, restricted: [] }).warnings).toContain('Statut : liste vide.');
	});
	it('erreurs : sans titre, sans champ, nom court en double, lien dans le remerciement', () => {
		const d = newDraft(info([col('A', 'text'), col('B', 'text')]), day);
		d.appearance.title = '';
		d.appearance.thanks = 'Voir https://ex.fr';
		d.fields[1].param = d.fields[0].param;
		const { errors } = checks(d, { columns: [], restricted: [] });
		expect(errors).toContain('Le formulaire n’a pas de titre.');
		expect(errors).toContain('Remerciement : ni lien ni redirection.');
		expect(errors).toContain('Le nom court « a » sert à plusieurs champs.');
		for (const f of d.fields) f.include = false;
		expect(checks(d, { columns: [], restricted: [] }).errors).toContain('Aucun champ : cochez au moins une colonne.');
	});
});

describe('pré-remplissage par l’adresse (§3.3 bis)', () => {
	it('nom court et lien', () => {
		expect(paramName('Espèce observée')).toBe('espece-observee');
		expect(paramName('Œufs', ['oeufs'])).toBe('oeufs-2');
		expect(paramName('***')).toBe('champ');
		expect(prefillURL('https://lattis.example.test/f/ABCDEFGHIJ', [['site', 'Nord'], ['espece', ''], ['commune', 'Saint-Malo & co']])).toBe(
			'https://lattis.example.test/f/ABCDEFGHIJ?site=Nord&commune=Saint-Malo+%26+co'
		);
	});
	it('QR code en SVG', () => {
		const svg = qrSVG('https://lattis.example.test/f/ABCDEFGHIJ?site=Nord');
		expect(svg.startsWith('<svg')).toBe(true);
		expect(svg).toContain('path');
	});
});

describe('§2.2', () => {
	it('action proposée selon la raison', () => {
		expect(reasonAction('local')?.label).toBe('Publier sur un serveur…');
		expect(reasonAction('private')?.label).toBe('Ajouter au classeur partagé…');
		expect(reasonAction('offline')?.action).toBe('retry');
		expect(reasonAction('frozen')).toBeNull();
	});
});
