// Fichier › Historique des versions : présentation des versions gardées
// (internal/versions) ; la boîte est VersionHistory.svelte.

/** Taille lisible : « 820 o », « 12 Ko », « 3,4 Mo ». */
export function fmtSize(bytes: number, locale: string, units: readonly string[]): string {
	let v = bytes;
	let u = 0;
	while (v >= 1024 && u < units.length - 1) {
		v /= 1024;
		u++;
	}
	const digits = u === 0 || v >= 10 ? 0 : 1;
	return `${v.toLocaleString(locale, { maximumFractionDigits: digits, minimumFractionDigits: digits })} ${units[u]}`;
}

/** Version telle que listée : son nom, sinon « auto » (version automatique). */
export function versionLabel(v: { name?: string }, auto: string): string {
	return v.name?.trim() || auto;
}
