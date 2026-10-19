import { describe, expect, it } from 'vitest';
import { cellLines, copyFillBlock, copyFillPlan, fillBlock, insertAt, isReloadKey, shortcutOf, timeText, todayText } from './shortcuts';

const key = (key: string, mods: Partial<{ ctrl: boolean; shift: boolean; alt: boolean; code: string }> = {}) => ({
	key,
	code: mods.code ?? '',
	ctrlKey: !!mods.ctrl,
	metaKey: false,
	shiftKey: !!mods.shift,
	altKey: !!mods.alt
});

describe('raccourcis', () => {
	it('Excel, AZERTY et QWERTY', () => {
		expect(shortcutOf(key('d', { ctrl: true }))).toBe('fillDown');
		expect(shortcutOf(key('R', { ctrl: true }))).toBe('fillRight');
		expect(shortcutOf(key(';', { ctrl: true, code: 'Comma' }))).toBe('today');
		expect(shortcutOf(key('/', { ctrl: true, shift: true, code: 'Period' }))).toBe('now'); // AZERTY
		expect(shortcutOf(key(':', { ctrl: true, shift: true, code: 'Semicolon' }))).toBe('now'); // QWERTY
		expect(shortcutOf(key('=', { alt: true, code: 'Equal' }))).toBe('autoSum');
		expect(shortcutOf(key('&', { ctrl: true, code: 'Digit1' }))).toBe('formatCells');
		expect(shortcutOf(key(' ', { shift: true, code: 'Space' }))).toBe('selectRow');
		expect(shortcutOf(key('L', { ctrl: true, shift: true }))).toBe('toggleFilter');
		expect(shortcutOf(key('PageDown', { ctrl: true }))).toBe('nextSheet');
		expect(shortcutOf(key('PageUp', { ctrl: true }))).toBe('prevSheet');
		expect(shortcutOf(key('F4'))).toBe('repeat');
		expect(shortcutOf(key('F11', { shift: true }))).toBe('insertSheet');
		expect(shortcutOf(key('End'))).toBe('endMode');
		expect(shortcutOf(key('F5'))).toBe('refresh');
		expect(shortcutOf(key('F5', { ctrl: true }))).toBe('refreshAll');
		expect(shortcutOf(key('F5', { shift: true }))).toBe('refreshAll');
		expect(shortcutOf(key('F5', { alt: true }))).toBeNull();
		expect(shortcutOf(key('F5', { ctrl: true, alt: true }))).toBe('refreshAll');
	});
	it('raccourcis d’Excel ajoutés avec le ruban (audit 3, D7)', () => {
		expect(shortcutOf(key('l', { ctrl: true }))).toBe('formatTable');
		expect(shortcutOf(key('t', { ctrl: true }))).toBe('formatTable');
		expect(shortcutOf(key('(', { ctrl: true, code: 'Digit5' }))).toBe('strike'); // AZERTY
		expect(shortcutOf(key('5', { ctrl: true, code: 'Digit5' }))).toBe('strike');
		expect(shortcutOf(key('ç', { ctrl: true, code: 'Digit9' }))).toBe('hideRows');
		expect(shortcutOf(key('à', { ctrl: true, code: 'Digit0' }))).toBe('hideCols');
		expect(shortcutOf(key('F2', { shift: true }))).toBe('note');
		expect(shortcutOf(key('F3', { shift: true }))).toBe('insertFunction');
		expect(shortcutOf(key('F1', { alt: true }))).toBe('chart');
		expect(shortcutOf(key('F11'))).toBe('chart');
		expect(shortcutOf(key('F2'))).toBeNull();
	});
	it('touches qui rechargeraient la page', () => {
		expect(isReloadKey(key('F5'))).toBe(true);
		expect(isReloadKey(key('F5', { ctrl: true }))).toBe(true);
		expect(isReloadKey(key('r', { ctrl: true }))).toBe(true);
		expect(isReloadKey(key('R', { ctrl: true, shift: true }))).toBe(true);
		expect(isReloadKey(key('r'))).toBe(false);
		expect(isReloadKey(key('F9'))).toBe(false);
	});
	it('touches ordinaires', () => {
		expect(shortcutOf(key('d'))).toBeNull();
		expect(shortcutOf(key(' '))).toBeNull();
		expect(shortcutOf(key('End', { ctrl: true }))).toBeNull();
		expect(shortcutOf(key('PageDown'))).toBeNull();
		expect(shortcutOf(key('l', { ctrl: true, alt: true }))).toBeNull();
	});
});

describe('date et heure', () => {
	const d = new Date(2026, 0, 5, 9, 7);
	it('jj/mm/aaaa et hh:mm', () => {
		expect(todayText(d)).toBe('05/01/2026');
		expect(timeText(d)).toBe('09:07');
	});
});

describe('recopie Ctrl+D / Ctrl+R', () => {
	const val = (r: number, c: number) => `${r}:${c}`;
	it('plusieurs lignes : la première recopiée dans les suivantes', () => {
		const p = copyFillPlan({ r0: 2, c0: 1, r1: 4, c1: 2 }, true)!;
		expect(p.dst).toEqual({ r0: 3, c0: 1, r1: 4, c1: 2 });
		expect(copyFillBlock(p, true, val)).toEqual([
			['2:1', '2:2'],
			['2:1', '2:2']
		]);
	});
	it('une seule ligne : celle du dessus', () => {
		const p = copyFillPlan({ r0: 5, c0: 0, r1: 5, c1: 0 }, true)!;
		expect(p.src).toEqual({ r0: 4, c0: 0, r1: 4, c1: 0 });
		expect(copyFillBlock(p, true, val)).toEqual([['4:0']]);
		expect(copyFillPlan({ r0: 0, c0: 0, r1: 0, c1: 0 }, true)).toBeNull();
	});
	it('vers la droite', () => {
		const p = copyFillPlan({ r0: 1, c0: 3, r1: 2, c1: 3 }, false)!;
		expect(copyFillBlock(p, false, val)).toEqual([['1:2'], ['2:2']]);
		const q = copyFillPlan({ r0: 1, c0: 3, r1: 1, c1: 5 }, false)!;
		expect(copyFillBlock(q, false, val)).toEqual([['1:3', '1:3']]);
	});
	it('Ctrl+Entrée : même texte partout', () => {
		expect(fillBlock({ r0: 0, c0: 0, r1: 1, c1: 2 }, 'x')).toEqual([
			['x', 'x', 'x'],
			['x', 'x', 'x']
		]);
	});
});

describe('texte sur plusieurs lignes', () => {
	it('lignes qui tiennent', () => {
		expect(cellLines('a\nb\nc', 3)).toEqual(['a', 'b', 'c']);
		expect(cellLines('a\nb\nc', 1)).toEqual(['a …']);
		expect(cellLines('a', 0)).toEqual(['a']);
	});
	it('insertion au curseur', () => {
		expect(insertAt('abcd', 2, 3, '\n')).toEqual({ value: 'ab\nd', caret: 3 });
	});
});
