// Relance de la visite guidée depuis l'aide (? › Visite guidée) : la page
// suit seq et lance la visite (sur un classeur neuf).

export const tourRequest = $state({ seq: 0 });

export function requestTour() {
	tourRequest.seq++;
}
