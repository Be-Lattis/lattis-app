package msg

// fr : catalogue de référence. Une entrée par message montré à
// l'utilisateur, l'identifiant pointé par domaine (doc, table, sheet,
// script, config…) ; les paramètres sont nommés (voir la syntaxe dans
// msg.go). Ajouter un message : une ligne ici, puis msg.New(id, Args{…})
// ou msg.Wrap(sentinelle, id, Args{…}) là où l'erreur naît.
var fr = map[ID]string{
	// Classeur
	"doc.none": "aucun classeur ouvert",

	// Tableaux et onglets
	"table.invalidName": "tableau : « {name} » n'est pas un nom de tableau (lettres, chiffres et _, sans espace)",
	"table.cellAddress": "tableau : « {name} » est une adresse de cellule, ambiguë comme nom de tableau (essayez « {name}_ »)",
	"table.nameTaken":   "tableau : « {name} » existe déjà",
	"sheet.nameTaken":   "onglet : « {name} » existe déjà (les noms de feuilles ne tiennent pas compte des majuscules)",

	// Scripts
	"script.timeout":  "script arrêté : durée maximale dépassée",
	"script.stopped":  "script arrêté par l'utilisateur",
	"script.memory":   "script arrêté : trop de mémoire utilisée",
	"script.disabled": "les scripts sont désactivés sur ce serveur (scripting web non autorisé par l'administrateur)",

	// Réglages
	"config.locked": "{path} : réglage imposé par la stratégie de votre organisation",
}
