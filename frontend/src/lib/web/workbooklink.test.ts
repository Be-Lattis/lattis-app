import { describe, expect, it } from 'vitest';
import { withoutWorkbookParam, workbookParam } from './workbooklink';

describe('lien /app/?workbook=<id>', () => {
	it('lit le classeur demandé', () => {
		expect(workbookParam('?workbook=2cc356a7-80dd')).toBe('2cc356a7-80dd');
		expect(workbookParam('?x=1')).toBe('');
		expect(workbookParam('')).toBe('');
	});
	it('retire seulement le paramètre workbook', () => {
		expect(withoutWorkbookParam('https://l.fr/app/?workbook=abc')).toBe('/app/');
		expect(withoutWorkbookParam('https://l.fr/app/?lang=en&workbook=abc#x')).toBe('/app/?lang=en#x');
	});
});
