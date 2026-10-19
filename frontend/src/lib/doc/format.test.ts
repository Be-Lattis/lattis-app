import { describe, expect, it } from 'vitest';
import { blockFormats, docFormat, parseDelta, quillFormats, sizePt } from './format';

describe('document : mise en forme', () => {
	it('lit la mise en forme Quill pour le ruban', () => {
		const f = docFormat({ bold: true, font: 'Arial', size: '14pt', color: '#c00000', align: 'center', header: 2 });
		expect(f.style).toEqual({ b: true, ff: 'Arial', fs: 14, fg: '#c00000', al: 'center' });
		expect(f.block).toBe('h2');
		expect(docFormat({ list: 'ordered' }).block).toBe('ordered');
		expect(docFormat({ align: 'justify' }).align).toBe('justify');
		// Sélection qui mêle deux polices : pas de police affichée.
		expect(docFormat({ font: ['Arial', 'Georgia'] }).style.ff).toBeUndefined();
	});

	it('traduit les correctifs du ruban en formats Quill', () => {
		expect(quillFormats({ b: 'toggle' }, { b: true })).toEqual({ bold: false });
		expect(quillFormats({ i: 'toggle' }, {})).toEqual({ italic: true });
		expect(quillFormats({ fs: 18, ff: null }, {})).toEqual({ size: '18pt', font: false });
		expect(quillFormats({ fs: 11 }, {})).toEqual({ size: false });
		expect(docFormat({}).style.fs).toBe(11);
		expect(quillFormats({ al: 'left', fg: null, bg: '#fff2cc' }, {})).toEqual({ align: false, color: false, background: '#fff2cc' });
		expect(quillFormats({ nf: '%', border: 'all' }, {})).toEqual({});
		expect(sizePt('10.5pt')).toBe(10.5);
		expect(sizePt('large')).toBeUndefined();
	});

	it('bascule les blocs du groupe Paragraphe', () => {
		expect(blockFormats('h1', 'normal')).toMatchObject({ header: 1, list: false });
		expect(blockFormats('bullet', 'bullet')).toMatchObject({ header: false, list: false, blockquote: false });
		expect(blockFormats('code', 'normal')['code-block']).toBe(true);
	});

	it('relit le contenu enregistré', () => {
		expect(parseDelta('{"ops":[{"insert":"a\\n"}]}').ops).toEqual([{ insert: 'a\n' }]);
		expect(parseDelta('Texte').ops).toEqual([{ insert: 'Texte\n' }]);
	});
});
