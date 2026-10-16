// Référence des fonctions disponibles dans les scripts Lua (onglet SCRIPT).
//
// La liste suit ce que le bac à sable expose réellement (internal/script :
// script.go pour db, sandbox.go pour les bibliothèques gardées). Le panneau
// d'aide (ScriptReference.svelte) l'affiche, le linter (lualint.ts) s'en sert
// pour signaler un nom inconnu ou un mauvais nombre d'arguments. Chaque
// exemple est un script complet qui tourne sur le classeur d'exemple de
// docs/scripting-lua.md (tableau Commandes : Client, Montant, Payé, Échéance).

export type Category = 'db' | 'base' | 'string' | 'table' | 'math' | 'os' | 'syntaxe';

export interface RefEntry {
	/** Nom complet : "db.query", "string.format", "print", "for". */
	name: string;
	category: Category;
	/** Forme d'appel, telle qu'on l'écrit. */
	signature: string;
	/** Une phrase. */
	summary: string;
	/** Précisions (facultatif). */
	details?: string;
	/** Script d'exemple, inséré tel quel par le bouton Insérer. */
	example?: string;
	/** Nombre d'arguments [min, max] (max null : illimité), vérifié par le linter. */
	args?: [number, number | null];
	/** Ancien nom gardé par compatibilité : connu du linter, mais signalé. */
	deprecated?: string;
}

export const categories: { id: Category; label: string }[] = [
	{ id: 'db', label: 'Classeur (db)' },
	{ id: 'base', label: 'Fonctions de base' },
	{ id: 'string', label: 'Texte (string)' },
	{ id: 'table', label: 'Tables (table)' },
	{ id: 'math', label: 'Maths (math)' },
	{ id: 'os', label: 'Date et heure (os)' },
	{ id: 'syntaxe', label: 'Syntaxe Lua' }
];

export const reference: RefEntry[] = [
	// --- Classeur ---------------------------------------------------------------
	{
		name: 'db.query',
		category: 'db',
		signature: 'db.query(sql, ...) → { ligne, ... }',
		summary: 'Exécute une requête SQL de lecture et renvoie toutes les lignes.',
		details:
			'Chaque ligne est une table { colonne = valeur }. Les tableaux et colonnes se nomment comme dans la grille ; un nom avec espace, accent ou majuscule se met entre guillemets doubles ("Montant HT"). Chaque tableau a une colonne cachée _id, la clé de la ligne. Les ? prennent les paramètres suivants (nombre, texte, booléen ou nil). Au plus 100 000 lignes : au-delà, utilisez db.rows. Une cellule vide donne nil (la clé est absente de la ligne), un booléen true/false, une date un texte "aaaa-mm-jj".',
		example: `local lignes = db.query("SELECT Client, Montant FROM Commandes WHERE Montant > ?", 50)
for _, l in ipairs(lignes) do
  print(l.Client, l.Montant)
end`,
		args: [1, null]
	},
	{
		name: 'db.rows',
		category: 'db',
		signature: 'for ligne in db.rows(sql, ...) do ... end',
		summary: 'Parcourt les lignes d’une requête une à une, sans limite de nombre.',
		details: 'Mêmes règles que db.query, mais les lignes ne sont pas toutes gardées en mémoire : à préférer pour les gros tableaux.',
		example: `local total = 0
for l in db.rows("SELECT Montant FROM Commandes") do
  total = total + (l.Montant or 0)
end
print("Total :", total)`,
		args: [1, null]
	},
	{
		name: 'db.value',
		category: 'db',
		signature: 'db.value(sql, ...) → valeur ou nil',
		summary: 'Renvoie la première colonne de la première ligne, ou nil s’il n’y a aucune ligne.',
		example: `local nb = db.value([[SELECT count(*) FROM Commandes WHERE NOT "Payé"]])
print(nb .. " commande(s) non payée(s)")`,
		args: [1, null]
	},
	{
		name: 'db.tables',
		category: 'db',
		signature: 'db.tables() → { {name, sheet, columns}, ... }',
		summary: 'Liste les tableaux du classeur avec leurs colonnes.',
		details:
			'Chaque colonne est { name, type, fixed } ; type vaut "integer", "decimal", "text", "boolean" ou "date", fixed est true si le type a été fixé à la main.',
		example: `for _, t in ipairs(db.tables()) do
  print(t.sheet .. " / " .. t.name)
  for _, c in ipairs(t.columns) do
    print("  " .. c.name, c.type)
  end
end`,
		args: [0, 0]
	},
	{
		name: 'db.insert',
		category: 'db',
		signature: 'db.insert(tableau, { colonne = valeur }) → _id',
		summary: 'Ajoute une ligne en fin de tableau et renvoie sa clé _id.',
		details:
			'Les valeurs sont traitées comme une frappe dans la grille : une colonne au type inféré s’élargit, une colonne au type fixé marque la saisie invalide, "" laisse la cellule vide. true et false s’écrivent vrai et faux ; une date se donne en texte ("16/10/2026"). Les noms de tableau et de colonne doivent être exacts (majuscules et accents compris).',
		example: `local id = db.insert("Commandes", {Client = "Petit", Montant = 15, ["Payé"] = false})
print("Ligne ajoutée :", id)`,
		args: [2, 2]
	},
	{
		name: 'db.update',
		category: 'db',
		signature: 'db.update(tableau, _id, { colonne = valeur })',
		summary: 'Modifie les colonnes données de la ligne _id ; les autres ne bougent pas.',
		details: 'Mêmes règles d’écriture que db.insert. "" efface la cellule.',
		example: `for l in db.rows("SELECT _id FROM Commandes WHERE Client = ?", "Durand") do
  db.update("Commandes", l._id, {["Payé"] = true})
end`,
		args: [3, 3]
	},
	{
		name: 'db.delete',
		category: 'db',
		signature: 'db.delete(tableau, _id)',
		summary: 'Supprime la ligne _id ; le tableau rétrécit d’une ligne.',
		example: `local id = db.value("SELECT _id FROM Commandes WHERE Montant = 0")
if id then db.delete("Commandes", id) end`,
		args: [2, 2]
	},
	{
		name: 'db.execute',
		category: 'db',
		signature: 'db.execute(sql, ...) → nombre de lignes touchées',
		summary: 'Exécute un INSERT, UPDATE ou DELETE avec les noms affichés.',
		details:
			'Annulable comme le reste du script. Les colonnes sont typées strictement : une valeur du mauvais type fait échouer toute la requête (un booléen s’écrit 1 ou 0, une date \'aaaa-mm-jj\'). Refusés pour l’instant : CREATE, ALTER, DROP et les tables internes _slitable_*.',
		example: `local n = db.execute([[UPDATE Commandes SET "Payé" = 1 WHERE Client = ?]], "Durand")
print(n .. " ligne(s) modifiée(s)")`,
		args: [1, null]
	},

	// --- Fonctions de base ------------------------------------------------------
	{
		name: 'print',
		category: 'base',
		signature: 'print(v1, v2, ...)',
		summary: 'Affiche des valeurs dans la sortie, séparées par une tabulation.',
		example: `print("Total :", 42, true)`
	},
	{
		name: 'type',
		category: 'base',
		signature: 'type(v) → "nil" | "boolean" | "number" | "string" | "table" | "function"',
		summary: 'Renvoie le type d’une valeur, sous forme de texte.',
		example: `print(type(42), type("a"), type(nil), type({}))`,
		args: [1, 1]
	},
	{
		name: 'tostring',
		category: 'base',
		signature: 'tostring(v) → texte',
		summary: 'Convertit une valeur en texte.',
		example: `print(tostring(3.5) .. " €")`,
		args: [1, 1]
	},
	{
		name: 'tonumber',
		category: 'base',
		signature: 'tonumber(v [, base]) → nombre ou nil',
		summary: 'Convertit un texte en nombre ; renvoie nil si ce n’est pas un nombre.',
		details: 'Le séparateur décimal est le point : tonumber("80,5") donne nil.',
		example: `print(tonumber("80.5") + 1, tonumber("ff", 16), tonumber("abc"))`,
		args: [1, 2]
	},
	{
		name: 'ipairs',
		category: 'base',
		signature: 'for i, v in ipairs(liste) do ... end',
		summary: 'Parcourt une liste dans l’ordre (indices 1, 2, 3...) jusqu’au premier trou.',
		example: `for i, nom in ipairs({"Durand", "Martin"}) do
  print(i, nom)
end`,
		args: [1, 1]
	},
	{
		name: 'pairs',
		category: 'base',
		signature: 'for cle, v in pairs(t) do ... end',
		summary: 'Parcourt toutes les clés d’une table, sans ordre garanti.',
		example: `for cle, v in pairs({Durand = 160, Martin = 80.5}) do
  print(cle, v)
end`,
		args: [1, 1]
	},
	{
		name: 'next',
		category: 'base',
		signature: 'next(t [, cle]) → cle, valeur',
		summary: 'Renvoie la clé suivante d’une table ; next(t) == nil teste si elle est vide.',
		example: `print(next({}) == nil)`,
		args: [1, 2]
	},
	{
		name: 'select',
		category: 'base',
		signature: 'select(n, ...) | select("#", ...)',
		summary: 'Renvoie les arguments à partir du n-ième, ou leur nombre avec "#".',
		example: `print(select("#", "a", "b", "c"), select(2, "a", "b", "c"))`,
		args: [1, null]
	},
	{
		name: 'unpack',
		category: 'base',
		signature: 'unpack(liste [, i [, j]]) → v1, v2, ...',
		summary: 'Renvoie les éléments d’une liste comme valeurs séparées.',
		example: `print(math.max(unpack({3, 9, 1})))`,
		args: [1, 3]
	},
	{
		name: 'pcall',
		category: 'base',
		signature: 'pcall(f, ...) → ok, résultat ou message',
		summary: 'Appelle f en rattrapant son erreur ; ok vaut false en cas d’erreur.',
		example: `local ok, err = pcall(db.insert, "Inconnu", {})
if not ok then print("Erreur :", err) end`,
		args: [1, null]
	},
	{
		name: 'xpcall',
		category: 'base',
		signature: 'xpcall(f, gestionnaire) → ok, résultat',
		summary: 'Comme pcall, mais passe le message d’erreur à une fonction de traitement.',
		example: `print(xpcall(function() error("oups") end, function(m) return "rattrapé : " .. m end))`,
		args: [2, 2]
	},
	{
		name: 'error',
		category: 'base',
		signature: 'error(message [, niveau])',
		summary: 'Arrête le script (ou le pcall en cours) avec un message.',
		example: `local montant = -5
if montant < 0 then error("montant négatif") end`,
		args: [0, 2]
	},
	{
		name: 'assert',
		category: 'base',
		signature: 'assert(condition [, message]) → condition',
		summary: 'Arrête le script avec le message si la condition est fausse (nil ou false).',
		example: `local nb = db.value("SELECT count(*) FROM Commandes WHERE Client IS NULL")
assert(nb == 0, nb .. " commande(s) sans client")`,
		args: [1, null]
	},
	{
		name: 'setmetatable',
		category: 'base',
		signature: 'setmetatable(t, mt) → t',
		summary: 'Associe une métatable à une table (valeurs par défaut, opérateurs...).',
		example: `local compteur = setmetatable({}, {__index = function() return 0 end})
compteur.Durand = compteur.Durand + 1
print(compteur.Durand, compteur.Martin)`,
		args: [2, 2]
	},
	{
		name: 'getmetatable',
		category: 'base',
		signature: 'getmetatable(v) → métatable ou nil',
		summary: 'Renvoie la métatable d’une valeur.',
		args: [1, 1]
	},
	{
		name: 'rawget',
		category: 'base',
		signature: 'rawget(t, cle)',
		summary: 'Lit une clé sans passer par la métatable.',
		args: [2, 2]
	},
	{
		name: 'rawset',
		category: 'base',
		signature: 'rawset(t, cle, valeur)',
		summary: 'Écrit une clé sans passer par la métatable.',
		args: [3, 3]
	},
	{
		name: 'rawequal',
		category: 'base',
		signature: 'rawequal(a, b)',
		summary: 'Compare deux valeurs sans passer par la métatable.',
		args: [2, 2]
	},

	// --- Texte ------------------------------------------------------------------
	{
		name: 'string.format',
		category: 'string',
		signature: 'string.format(modèle, ...) → texte',
		summary: 'Met en forme des valeurs : %s texte, %d entier, %.2f décimal, %5s largeur, %% le signe %.',
		example: `print(string.format("%-10s %8.2f €", "Durand", 80.5))`,
		args: [1, null]
	},
	{
		name: 'string.sub',
		category: 'string',
		signature: 's:sub(i [, j]) → texte',
		summary: 'Extrait les caractères de i à j (indices à partir de 1, négatifs depuis la fin).',
		details: 'Les indices comptent des octets : une lettre accentuée en occupe deux.',
		example: `print(("Durand"):sub(1, 3), ("Durand"):sub(-2))`,
		args: [2, 3]
	},
	{
		name: 'string.upper',
		category: 'string',
		signature: 's:upper() → texte',
		summary: 'Met en majuscules (accents compris).',
		example: `print(("Élodie"):upper())`,
		args: [1, 1]
	},
	{
		name: 'string.lower',
		category: 'string',
		signature: 's:lower() → texte',
		summary: 'Met en minuscules.',
		example: `print(("DURAND"):lower())`,
		args: [1, 1]
	},
	{
		name: 'string.len',
		category: 'string',
		signature: 's:len() → nombre (ou #s)',
		summary: 'Longueur en octets : #"Payé" vaut 5.',
		example: `print(("Durand"):len(), #"Payé")`,
		args: [1, 1]
	},
	{
		name: 'string.find',
		category: 'string',
		signature: 's:find(motif [, début [, simple]]) → début, fin ou nil',
		summary: 'Cherche un motif ; simple = true cherche le texte tel quel.',
		details: 'Motifs : %d chiffre, %a lettre, %s espace, %w alphanumérique, . tout caractère, * + - ? répétitions, ^ $ ancrages, % pour échapper. Ce ne sont pas des expressions régulières.',
		example: `print(("Facture 2026-10"):find("%d+"))
print(("a.b"):find(".", 1, true))`,
		args: [2, 4]
	},
	{
		name: 'string.match',
		category: 'string',
		signature: 's:match(motif [, début]) → captures ou nil',
		summary: 'Renvoie ce qui correspond au motif, ou ses captures entre parenthèses.',
		example: `local a, m, j = ("2026-10-16"):match("(%d+)-(%d+)-(%d+)")
print(j .. "/" .. m .. "/" .. a)
print(("  texte  "):match("^%s*(.-)%s*$"))`,
		args: [2, 3]
	},
	{
		name: 'string.gmatch',
		category: 'string',
		signature: 'for m in s:gmatch(motif) do ... end',
		summary: 'Parcourt toutes les correspondances d’un motif.',
		example: `for mot in ("a;b;c"):gmatch("[^;]+") do print(mot) end`,
		args: [2, 2]
	},
	{
		name: 'string.gsub',
		category: 'string',
		signature: 's:gsub(motif, remplacement [, n]) → texte, nombre',
		summary: 'Remplace les correspondances ; le remplacement peut être un texte, une table ou une fonction.',
		example: `local propre = ("a  b   c"):gsub("%s+", " ")
print(propre)`,
		args: [3, 4]
	},
	{
		name: 'string.rep',
		category: 'string',
		signature: 's:rep(n) → texte',
		summary: 'Répète un texte n fois (résultat limité à 16 Mo).',
		example: `print(("-"):rep(20))`,
		args: [2, 2]
	},
	{
		name: 'string.reverse',
		category: 'string',
		signature: 's:reverse() → texte',
		summary: 'Inverse un texte (octet par octet).',
		args: [1, 1]
	},
	{
		name: 'string.byte',
		category: 'string',
		signature: 's:byte([i [, j]]) → codes',
		summary: 'Renvoie les codes des octets de i à j.',
		example: `print(("A"):byte())`,
		args: [1, 3]
	},
	{
		name: 'string.char',
		category: 'string',
		signature: 'string.char(code, ...) → texte',
		summary: 'Construit un texte à partir de codes d’octets.',
		example: `print(string.char(72, 105))`,
		args: [0, null]
	},
	{ name: 'string.gfind', category: 'string', signature: 'string.gfind(s, motif)', summary: 'Ancien nom de string.gmatch.', deprecated: 'string.gmatch', args: [2, 2] },

	// --- Tables -----------------------------------------------------------------
	{
		name: 'table.insert',
		category: 'table',
		signature: 'table.insert(liste, [position,] valeur)',
		summary: 'Ajoute une valeur en fin de liste, ou à la position donnée.',
		example: `local noms = {"Durand"}
table.insert(noms, "Martin")
table.insert(noms, 1, "Petit")
print(table.concat(noms, ", "))`,
		args: [2, 3]
	},
	{
		name: 'table.remove',
		category: 'table',
		signature: 'table.remove(liste [, position]) → valeur retirée',
		summary: 'Retire le dernier élément, ou celui de la position donnée.',
		example: `local noms = {"Durand", "Martin", "Petit"}
print(table.remove(noms), table.remove(noms, 1), #noms)`,
		args: [1, 2]
	},
	{
		name: 'table.sort',
		category: 'table',
		signature: 'table.sort(liste [, avant])',
		summary: 'Trie une liste sur place ; avant(a, b) renvoie true si a doit passer avant b.',
		example: `local l = {{n = "B", v = 2}, {n = "A", v = 5}}
table.sort(l, function(a, b) return a.v > b.v end)
print(l[1].n)`,
		args: [1, 2]
	},
	{
		name: 'table.concat',
		category: 'table',
		signature: 'table.concat(liste [, séparateur [, i [, j]]]) → texte',
		summary: 'Assemble les éléments d’une liste en un texte.',
		example: `print(table.concat({"a", "b", "c"}, ", "))`,
		args: [1, 4]
	},
	{
		name: 'table.maxn',
		category: 'table',
		signature: 'table.maxn(t) → nombre',
		summary: 'Renvoie le plus grand indice numérique d’une table.',
		args: [1, 1]
	},
	{ name: 'table.getn', category: 'table', signature: 'table.getn(t)', summary: 'Ancien équivalent de #t.', deprecated: '#t', args: [1, 1] },

	// --- Maths ------------------------------------------------------------------
	{ name: 'math.floor', category: 'math', signature: 'math.floor(x)', summary: 'Arrondit à l’entier inférieur ; math.floor(a / b) fait une division entière.', example: `print(math.floor(7 / 2), math.floor(-2.5))`, args: [1, 1] },
	{ name: 'math.ceil', category: 'math', signature: 'math.ceil(x)', summary: 'Arrondit à l’entier supérieur.', example: `print(math.ceil(2.1))`, args: [1, 1] },
	{
		name: 'math.abs',
		category: 'math',
		signature: 'math.abs(x)',
		summary: 'Valeur absolue.',
		example: `print(math.abs(-4))`,
		args: [1, 1]
	},
	{ name: 'math.min', category: 'math', signature: 'math.min(x, ...)', summary: 'Plus petite des valeurs.', example: `print(math.min(3, 9, 1))`, args: [1, null] },
	{ name: 'math.max', category: 'math', signature: 'math.max(x, ...)', summary: 'Plus grande des valeurs.', example: `print(math.max(3, 9, 1))`, args: [1, null] },
	{ name: 'math.sqrt', category: 'math', signature: 'math.sqrt(x)', summary: 'Racine carrée.', example: `print(math.sqrt(16))`, args: [1, 1] },
	{ name: 'math.pow', category: 'math', signature: 'math.pow(x, y)', summary: 'Puissance ; s’écrit aussi x ^ y.', example: `print(math.pow(2, 10), 2 ^ 10)`, args: [2, 2] },
	{ name: 'math.exp', category: 'math', signature: 'math.exp(x)', summary: 'Exponentielle.', args: [1, 1] },
	{ name: 'math.log', category: 'math', signature: 'math.log(x)', summary: 'Logarithme népérien.', args: [1, 1] },
	{ name: 'math.log10', category: 'math', signature: 'math.log10(x)', summary: 'Logarithme décimal.', args: [1, 1] },
	{ name: 'math.fmod', category: 'math', signature: 'math.fmod(x, y)', summary: 'Reste de la division (signe de x) ; voir aussi l’opérateur %.', example: `print(math.fmod(7, 3), -7 % 3)`, args: [2, 2] },
	{ name: 'math.modf', category: 'math', signature: 'math.modf(x) → entier, fraction', summary: 'Sépare partie entière et partie décimale.', example: `print(math.modf(3.75))`, args: [1, 1] },
	{
		name: 'math.random',
		category: 'math',
		signature: 'math.random([m [, n]])',
		summary: 'Nombre au hasard : réel dans [0, 1[, entier dans [1, m] ou [m, n].',
		example: `print(math.random(1, 6))`,
		args: [0, 2]
	},
	{ name: 'math.randomseed', category: 'math', signature: 'math.randomseed(x)', summary: 'Initialise le générateur de math.random.', example: `math.randomseed(os.time())`, args: [1, 1] },
	{ name: 'math.huge', category: 'math', signature: 'math.huge', summary: 'L’infini (valeur, pas une fonction).' },
	{ name: 'math.pi', category: 'math', signature: 'math.pi', summary: 'Le nombre π (valeur, pas une fonction).' },
	{ name: 'math.sin', category: 'math', signature: 'math.sin(x)', summary: 'Sinus (x en radians).', args: [1, 1] },
	{ name: 'math.cos', category: 'math', signature: 'math.cos(x)', summary: 'Cosinus (x en radians).', args: [1, 1] },
	{ name: 'math.tan', category: 'math', signature: 'math.tan(x)', summary: 'Tangente (x en radians).', args: [1, 1] },
	{ name: 'math.asin', category: 'math', signature: 'math.asin(x)', summary: 'Arc sinus.', args: [1, 1] },
	{ name: 'math.acos', category: 'math', signature: 'math.acos(x)', summary: 'Arc cosinus.', args: [1, 1] },
	{ name: 'math.atan', category: 'math', signature: 'math.atan(x)', summary: 'Arc tangente.', args: [1, 1] },
	{ name: 'math.atan2', category: 'math', signature: 'math.atan2(y, x)', summary: 'Arc tangente de y/x, dans le bon quadrant.', args: [2, 2] },
	{ name: 'math.sinh', category: 'math', signature: 'math.sinh(x)', summary: 'Sinus hyperbolique.', args: [1, 1] },
	{ name: 'math.cosh', category: 'math', signature: 'math.cosh(x)', summary: 'Cosinus hyperbolique.', args: [1, 1] },
	{ name: 'math.tanh', category: 'math', signature: 'math.tanh(x)', summary: 'Tangente hyperbolique.', args: [1, 1] },
	{ name: 'math.deg', category: 'math', signature: 'math.deg(x)', summary: 'Radians vers degrés.', args: [1, 1] },
	{ name: 'math.rad', category: 'math', signature: 'math.rad(x)', summary: 'Degrés vers radians.', args: [1, 1] },
	{ name: 'math.frexp', category: 'math', signature: 'math.frexp(x) → m, e', summary: 'Décompose x en m × 2^e.', args: [1, 1] },
	{ name: 'math.ldexp', category: 'math', signature: 'math.ldexp(m, e)', summary: 'Calcule m × 2^e.', args: [2, 2] },
	{ name: 'math.mod', category: 'math', signature: 'math.mod(x, y)', summary: 'Ancien nom de math.fmod.', deprecated: 'math.fmod', args: [2, 2] },

	// --- Date et heure ----------------------------------------------------------
	{
		name: 'os.date',
		category: 'os',
		signature: 'os.date([format [, temps]]) → texte',
		summary: 'Met en forme une date : %d jour, %m mois, %Y année, %H:%M:%S heure.',
		details: 'Les dates du classeur sont au format "aaaa-mm-jj", soit os.date("%Y-%m-%d") ; elles se comparent alors comme des textes.',
		example: `print(os.date("%d/%m/%Y"))
local aujourdhui = os.date("%Y-%m-%d")
print(db.value([[SELECT count(*) FROM Commandes WHERE "Échéance" < ?]], aujourdhui) .. " échéance(s) passée(s)")`,
		args: [0, 2]
	},
	{
		name: 'os.time',
		category: 'os',
		signature: 'os.time([{year, month, day, hour, min, sec}]) → secondes',
		summary: 'Renvoie l’heure actuelle, ou celle de la date donnée, en secondes.',
		example: `local echeance = os.time({year = 2026, month = 12, day = 31, hour = 0})
print(math.floor(os.difftime(echeance, os.time()) / 86400) .. " jour(s) avant la fin de l'année")`,
		args: [0, 1]
	},
	{
		name: 'os.difftime',
		category: 'os',
		signature: 'os.difftime(t2, t1) → secondes',
		summary: 'Écart entre deux temps renvoyés par os.time.',
		args: [2, 2]
	},
	{
		name: 'os.clock',
		category: 'os',
		signature: 'os.clock() → secondes',
		summary: 'Temps processeur écoulé : sert à mesurer la durée d’un traitement.',
		example: `local t0 = os.clock()
local n = db.value("SELECT count(*) FROM Commandes")
print(string.format("%d ligne(s) en %.3f s", n, os.clock() - t0))`,
		args: [0, 0]
	},

	// --- Syntaxe ----------------------------------------------------------------
	{
		name: 'local',
		category: 'syntaxe',
		signature: 'local nom = valeur',
		summary: 'Déclare une variable locale. Sans local, la variable est globale.',
		details: 'Un nom ne contient que des lettres sans accent, des chiffres et _. Seuls nil et false sont faux : 0 et "" sont vrais.',
		example: `local client, montant = "Durand", 120
local remise = nil or 0`
	},
	{
		name: 'if',
		category: 'syntaxe',
		signature: 'if cond then ... elseif cond then ... else ... end',
		summary: 'Condition. Différent s’écrit ~= ; and, or et not sont les opérateurs logiques.',
		example: `local montant = 80
if montant >= 100 then
  print("gros")
elseif montant ~= 0 then
  print("moyen")
else
  print("nul")
end`
	},
	{
		name: 'for',
		category: 'syntaxe',
		signature: 'for i = début, fin [, pas] do ... end',
		summary: 'Boucle numérique ; for ... in sert aux itérateurs (ipairs, pairs, db.rows).',
		details: 'break sort de la boucle ; il n’y a pas de continue.',
		example: `for i = 1, 3 do print(i) end
for i = 10, 1, -3 do print(i) end`
	},
	{
		name: 'while',
		category: 'syntaxe',
		signature: 'while cond do ... end  |  repeat ... until cond',
		summary: 'Boucles conditionnelles. Une boucle sans fin est arrêtée au bout de 30 s ou par le bouton Arrêter.',
		example: `local n = 0
while n < 3 do n = n + 1 end
repeat n = n - 1 until n == 0
print(n)`
	},
	{
		name: 'function',
		category: 'syntaxe',
		signature: 'local function nom(a, b) ... return x end',
		summary: 'Définit une fonction ; elle peut renvoyer plusieurs valeurs.',
		example: `local function ttc(ht, taux)
  taux = taux or 0.2
  return ht * (1 + taux)
end
print(ttc(100), ttc(100, 0.055))`
	},
	{
		name: 'table',
		category: 'syntaxe',
		signature: '{ v1, v2 }  |  { cle = valeur, ["clé"] = valeur }',
		summary: 'La seule structure de données : liste (indices à partir de 1) ou dictionnaire.',
		details: '#t donne la longueur d’une liste. Une clé avec accent ou espace s’écrit t["Payé"].',
		example: `local noms = {"Durand", "Martin"}
local ligne = {Client = "Durand", ["Payé"] = true}
print(#noms, noms[1], ligne.Client, ligne["Payé"])`
	},
	{
		name: '..',
		category: 'syntaxe',
		signature: 'a .. b',
		summary: 'Concatène deux textes (ou nombres). # donne la longueur, [[ ... ]] écrit un texte sur plusieurs lignes.',
		example: `local sql = [[SELECT Client FROM Commandes WHERE "Payé"]]
print("Requête : " .. sql, #sql)`
	},
	{
		name: '--',
		category: 'syntaxe',
		signature: '-- commentaire  |  --[[ commentaire long ]]',
		summary: 'Commentaire sur une ligne, ou sur plusieurs lignes.'
	}
];

/** Index par nom complet. */
export const byName: Map<string, RefEntry> = new Map(reference.map((e) => [e.name, e]));

/** Bibliothèques exposées au script et leurs membres (pour le linter). */
export const libraries: Record<string, Set<string>> = {};
for (const e of reference) {
	const dot = e.name.indexOf('.');
	if (dot > 0 && e.category !== 'syntaxe') {
		(libraries[e.name.slice(0, dot)] ??= new Set()).add(e.name.slice(dot + 1));
	}
}

/** Variables globales prédéfinies. */
export const globals: Set<string> = new Set([
	...Object.keys(libraries),
	...reference.filter((e) => e.category === 'base').map((e) => e.name),
	'_G',
	'_VERSION'
]);

/** Ce qui existe en Lua mais que le bac à sable retire, avec la raison affichée. */
export const unavailable: Record<string, string> = {
	io: 'pas d’accès aux fichiers dans un script',
	require: 'pas de chargement de module dans un script',
	dofile: 'pas d’accès aux fichiers dans un script',
	loadfile: 'pas d’accès aux fichiers dans un script',
	load: 'pas de chargement de code dans un script',
	loadstring: 'pas de chargement de code dans un script',
	debug: 'bibliothèque debug non disponible',
	coroutine: 'bibliothèque coroutine non disponible',
	collectgarbage: 'non disponible dans un script',
	getfenv: 'non disponible dans un script',
	setfenv: 'non disponible dans un script',
	module: 'pas de chargement de module dans un script',
	package: 'pas de chargement de module dans un script',
	newproxy: 'non disponible dans un script',
	'os.execute': 'pas de lancement de programme dans un script',
	'os.exit': 'non disponible dans un script',
	'os.getenv': 'pas d’accès aux variables d’environnement',
	'os.remove': 'pas d’accès aux fichiers dans un script',
	'os.rename': 'pas d’accès aux fichiers dans un script',
	'os.tmpname': 'pas d’accès aux fichiers dans un script',
	'os.setlocale': 'non disponible dans un script',
	'string.dump': 'non disponible dans un script'
};

/** Recherche dans la référence (nom, signature, résumé, précisions), sans tenir compte des accents. */
export function search(q: string, entries: RefEntry[] = reference): RefEntry[] {
	const norm = (s: string) => s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
	const words = norm(q).split(/\s+/).filter(Boolean);
	if (!words.length) return entries;
	const scored: { e: RefEntry; score: number }[] = [];
	for (const e of entries) {
		const name = norm(e.name);
		const text = norm(`${e.signature} ${e.summary} ${e.details ?? ''}`);
		let score = 0;
		for (const w of words) {
			if (name === w || name.endsWith('.' + w)) score += 100;
			else if (name.includes(w)) score += 50;
			else if (text.includes(w)) score += 10;
			else {
				score = 0;
				break;
			}
		}
		if (score) scored.push({ e, score });
	}
	return scored.sort((a, b) => b.score - a.score).map((s) => s.e);
}
