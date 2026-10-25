package formula

// Traductions allemandes de docs_added.go.
func docsAddedDE() map[string]docText {
	x := ta("x", "Zu untersuchender Wert.")
	xPos := ta("x", "Zu untersuchender Wert, positiv.")
	cum := ta("kumuliert", "{=TRUE}: kumulierte Wahrscheinlichkeit (Verteilungsfunktion); {=FALSE}: Dichte.")
	mean := ta("Mittelwert", "Mittelwert der Verteilung.")
	sd := ta("Standabwn", "Standardabweichung der Verteilung (positiv).")
	prob := ta("Wahrsch", "Wahrscheinlichkeit zwischen 0 und 1.")
	df := ta("Freiheitsgrade", "Anzahl der Freiheitsgrade (abgeschnitten).")
	df1 := ta("Freiheitsgrade1", "Freiheitsgrade des Zählers.")
	df2 := ta("Freiheitsgrade2", "Freiheitsgrade des Nenners.")
	alpha := ta("Alpha", "Formparameter.")
	beta := ta("Beta", "Skalenparameter.")
	knownY := ta("Y_Werte", "Spalte der zu erklärenden Werte (Tabelle[Spalte]) oder Bereich.")
	knownX := ta("X_Werte", "Spalte der erklärenden Werte aus derselben Tabelle oder Bereich gleicher Form.")
	konst := ta("Konstante", "{=TRUE} oder weggelassen: Gerade mit Achsenabschnitt; {=FALSE}: durch 0.")
	agg := ta("Zahl1", "Ganze Spalte (Tabelle[Spalte]), Bereich (Cell[A1:B3]), dynamisches Array oder Konstante, Ausdruck über die Spalten einer Tabelle, Zelle oder Wert.")
	s1 := ta("Matrix1", "Erste Spalte (Tabelle[Spalte]) oder Bereich.")
	s2 := ta("Matrix2", "Zweite Spalte aus derselben Tabelle oder Bereich gleicher Form.")
	settle := ta("Abrechnung", "Kaufdatum des Wertpapiers.")
	mat := ta("Fälligkeit", "Fälligkeitsdatum des Wertpapiers.")
	freq := ta("Häufigkeit", "Zinszahlungen pro Jahr: 1, 2 oder 4.")
	basis := ta("Basis", "Tageszählung: 0 oder weggelassen 30/360, 1 tatsächlich/tatsächlich, 2 tatsächlich/360, 3 tatsächlich/365, 4 europäisch 30/360.")
	cost := ta("Ansch_Wert", "Anschaffungswert des Wirtschaftsguts.")
	salvage := ta("Restwert", "Wert am Ende der Abschreibung.")
	life := ta("Nutzungsdauer", "Abschreibungsdauer in Perioden.")
	period := ta("Periode", "Nummer der Periode.")
	db := ta("Datenbank", "Ganze Tabelle, über ihren Namen (Verkäufe).")
	field := ta("Datenbankfeld", "Spaltenname in Anführungszeichen (\"Betrag\") oder ihre Nummer.")
	crit := ta("Suchkriterien", "Zellbereich (Cell[E1:F3]): Spaltenüberschriften, dann eine Bedingung pro Zeile.")
	amorDate := ta("Kaufdatum", "Kaufdatum des Wirtschaftsguts.")
	amorFirst := ta("Erster_Zinstermin", "Enddatum der ersten Periode.")
	amorPer := ta("Periode", "Nummer der Periode (0: die erste, anteilig).")
	amorRate := ta("Rate", "Abschreibungssatz.")
	amorBasis := ta("Basis", "Tageszählung: 0 oder weggelassen 30/360, 1 tatsächlich/tatsächlich, 3 tatsächlich/365, 4 europäisch 30/360.")
	coupon := ta("Zins", "Jährlicher Kuponzins.")
	redemption := ta("Rückzahlung", "Rückzahlungswert je 100 Nennwert.")
	dbText := func(desc, ex string) docText { return tdx(desc, ex, db, field, crit) }
	return map[string]docText{
		"NORM.DIST":       tdx("Gibt die Normalverteilung für einen Mittelwert und eine Standardabweichung zurück.", `=NORM.DIST(42, 40, 1.5, TRUE)`, x, mean, sd, cum),
		"NORM.INV":        tdx("Gibt den Wert der Normalverteilung mit der angegebenen kumulierten Wahrscheinlichkeit zurück.", `=NORM.INV(0.95, 40, 1.5)`, prob, mean, sd),
		"NORM.S.DIST":     tdx("Gibt die Standardnormalverteilung zurück (Mittelwert 0, Standardabweichung 1).", `=NORM.S.DIST(1.96, TRUE)`, ta("z", "Zu untersuchender Wert."), cum),
		"NORM.S.INV":      tdx("Gibt den Wert der Standardnormalverteilung mit der angegebenen kumulierten Wahrscheinlichkeit zurück.", `=NORM.S.INV(0.975)`, prob),
		"NORMSDIST":       tdx("Gibt die kumulierte Standardnormalverteilung zurück (ältere Form).", `=NORMSDIST(1.96)`, ta("z", "Zu untersuchender Wert.")),
		"LOGNORM.DIST":    tdx("Gibt die Lognormalverteilung zurück (ln(x) ist normalverteilt).", `=LOGNORM.DIST(4, 3.5, 1.2, TRUE)`, xPos, ta("Mittelwert", "Mittelwert von ln(x)."), ta("Standabwn", "Standardabweichung von ln(x)."), cum),
		"LOGNORM.INV":     tdx("Gibt den Wert der Lognormalverteilung mit der angegebenen kumulierten Wahrscheinlichkeit zurück.", `=LOGNORM.INV(0.04, 3.5, 1.2)`, prob, ta("Mittelwert", "Mittelwert von ln(x)."), ta("Standabwn", "Standardabweichung von ln(x).")),
		"T.DIST":          tdx("Gibt die t-Verteilung nach Student zurück (linkes Ende).", `=T.DIST(1.5, 10, TRUE)`, x, df, cum),
		"T.DIST.2T":       tdx("Gibt die zweiseitige Wahrscheinlichkeit der t-Verteilung zurück.", `=T.DIST.2T(1.96, 60)`, xPos, df),
		"T.DIST.RT":       tdx("Gibt die rechtsseitige Wahrscheinlichkeit der t-Verteilung zurück.", `=T.DIST.RT(1.96, 60)`, x, df),
		"T.INV":           tdx("Gibt den Wert der t-Verteilung mit der angegebenen linksseitigen Wahrscheinlichkeit zurück.", `=T.INV(0.975, 10)`, prob, df),
		"T.INV.2T":        tdx("Gibt den Wert der t-Verteilung mit der angegebenen zweiseitigen Wahrscheinlichkeit zurück.", `=T.INV.2T(0.05, 10)`, prob, df),
		"CHISQ.DIST":      tdx("Gibt die Chi-Quadrat-Verteilung zurück (linkes Ende).", `=CHISQ.DIST(3, 2, TRUE)`, xPos, df, cum),
		"CHISQ.DIST.RT":   tdx("Gibt die rechtsseitige Wahrscheinlichkeit der Chi-Quadrat-Verteilung zurück.", `=CHISQ.DIST.RT(18.3, 10)`, xPos, df),
		"CHISQ.INV":       tdx("Gibt den Wert der Chi-Quadrat-Verteilung mit der angegebenen linksseitigen Wahrscheinlichkeit zurück.", `=CHISQ.INV(0.95, 10)`, prob, df),
		"CHISQ.INV.RT":    tdx("Gibt den Wert der Chi-Quadrat-Verteilung mit der angegebenen rechtsseitigen Wahrscheinlichkeit zurück.", `=CHISQ.INV.RT(0.05, 10)`, prob, df),
		"F.DIST":          tdx("Gibt die F-Verteilung zurück (linkes Ende).", `=F.DIST(15, 6, 4, TRUE)`, xPos, df1, df2, cum),
		"F.DIST.RT":       tdx("Gibt die rechtsseitige Wahrscheinlichkeit der F-Verteilung zurück.", `=F.DIST.RT(15, 6, 4)`, xPos, df1, df2),
		"F.INV":           tdx("Gibt den Wert der F-Verteilung mit der angegebenen linksseitigen Wahrscheinlichkeit zurück.", `=F.INV(0.95, 6, 4)`, prob, df1, df2),
		"F.INV.RT":        tdx("Gibt den Wert der F-Verteilung mit der angegebenen rechtsseitigen Wahrscheinlichkeit zurück.", `=F.INV.RT(0.05, 6, 4)`, prob, df1, df2),
		"BINOM.DIST":      tdx("Gibt die Wahrscheinlichkeit einer Anzahl Erfolge bei einer Anzahl Versuche zurück (Binomialverteilung).", `=BINOM.DIST(6, 10, 0.5, FALSE)`, ta("Zahl_Erfolge", "Anzahl der Erfolge."), ta("Versuche", "Anzahl unabhängiger Versuche."), ta("Erfolgswahrsch", "Erfolgswahrscheinlichkeit je Versuch."), cum),
		"BINOM.INV":       tdx("Gibt die kleinste Anzahl Erfolge zurück, deren kumulierte Wahrscheinlichkeit eine Schwelle erreicht.", `=BINOM.INV(6, 0.5, 0.75)`, ta("Versuche", "Anzahl der Versuche."), ta("Erfolgswahrsch", "Erfolgswahrscheinlichkeit."), ta("Alpha", "Schwelle der kumulierten Wahrscheinlichkeit.")),
		"NEGBINOM.DIST":   tdx("Gibt die Wahrscheinlichkeit einer Anzahl Misserfolge vor einer Anzahl Erfolge zurück (negative Binomialverteilung).", `=NEGBINOM.DIST(10, 5, 0.25, TRUE)`, ta("Zahl_Misserfolge", "Anzahl der Misserfolge."), ta("Zahl_Erfolge", "Angestrebte Anzahl Erfolge."), ta("Erfolgswahrsch", "Erfolgswahrscheinlichkeit."), cum),
		"HYPGEOM.DIST":    tdx("Gibt die Wahrscheinlichkeit einer Anzahl Erfolge in einer Stichprobe ohne Zurücklegen zurück (hypergeometrische Verteilung).", `=HYPGEOM.DIST(1, 4, 8, 20, TRUE)`, ta("Erfolge_S", "Anzahl der Erfolge in der Stichprobe."), ta("Umfang_S", "Umfang der Stichprobe."), ta("Erfolge_G", "Anzahl der Erfolge in der Grundgesamtheit."), ta("Umfang_G", "Umfang der Grundgesamtheit."), cum),
		"POISSON.DIST":    tdx("Gibt die Poisson-Verteilung zurück: Wahrscheinlichkeit einer Anzahl Ereignisse.", `=POISSON.DIST(2, 5, TRUE)`, ta("x", "Anzahl der Ereignisse."), ta("Mittelwert", "Erwartete Anzahl Ereignisse."), cum),
		"EXPON.DIST":      tdx("Gibt die Exponentialverteilung zurück (Zeit zwischen zwei Ereignissen).", `=EXPON.DIST(0.2, 10, TRUE)`, xPos, ta("Lambda", "Parameter der Verteilung (positiv)."), cum),
		"GAMMA.DIST":      tdx("Gibt die Gammaverteilung zurück.", `=GAMMA.DIST(10, 9, 2, TRUE)`, xPos, alpha, beta, cum),
		"GAMMA.INV":       tdx("Gibt den Wert der Gammaverteilung mit der angegebenen kumulierten Wahrscheinlichkeit zurück.", `=GAMMA.INV(0.07, 9, 2)`, prob, alpha, beta),
		"GAMMALN":         tdx("Gibt den natürlichen Logarithmus der Gammafunktion zurück.", `=GAMMALN(4)`, ta("x", "Positive Zahl.")),
		"GAMMA":           tdx("Gibt den Wert der Gammafunktion zurück.", `=GAMMA(2.5)`, ta("Zahl", "Zahl (weder 0 noch eine negative ganze Zahl).")),
		"BETA.DIST":       tdx("Gibt die Betaverteilung zurück, zwischen 0 und 1 oder zwischen zwei Grenzen.", `=BETA.DIST(2, 8, 10, TRUE, 1, 3)`, ta("x", "Zu untersuchender Wert zwischen A und B."), ta("Alpha", "Erster Formparameter."), ta("Beta", "Zweiter Formparameter."), cum, ta("A", "Untere Grenze (0, wenn weggelassen)."), ta("B", "Obere Grenze (1, wenn weggelassen).")),
		"BETA.INV":        tdx("Gibt den Wert der Betaverteilung mit der angegebenen kumulierten Wahrscheinlichkeit zurück.", `=BETA.INV(0.69, 8, 10, 1, 3)`, prob, ta("Alpha", "Erster Formparameter."), ta("Beta", "Zweiter Formparameter."), ta("A", "Untere Grenze (0, wenn weggelassen)."), ta("B", "Obere Grenze (1, wenn weggelassen).")),
		"WEIBULL.DIST":    tdx("Gibt die Weibull-Verteilung zurück (Lebensdauern, Ausfälle).", `=WEIBULL.DIST(105, 20, 100, TRUE)`, xPos, alpha, beta, cum),
		"CONFIDENCE.NORM": tdx("Gibt die halbe Breite des Konfidenzintervalls eines Mittelwerts zurück (Normalverteilung).", `=CONFIDENCE.NORM(0.05, 2.5, 50)`, ta("Alpha", "Risiko: 0,05 für ein 95-%-Intervall."), ta("Standabwn", "Standardabweichung der Grundgesamtheit."), ta("Umfang", "Umfang der Stichprobe.")),
		"CONFIDENCE.T":    tdx("Gibt die halbe Breite des Konfidenzintervalls eines Mittelwerts zurück (t-Verteilung).", `=CONFIDENCE.T(0.05, 1, 50)`, ta("Alpha", "Risiko: 0,05 für ein 95-%-Intervall."), ta("Standabwn", "Standardabweichung der Stichprobe."), ta("Umfang", "Umfang der Stichprobe.")),
		"STANDARDIZE":     tdx("Gibt den standardisierten Wert zurück: (x − Mittelwert) / Standardabweichung.", `=STANDARDIZE([Note], 12, 3)`, ta("x", "Zu standardisierender Wert."), mean, sd),
		"PHI":             tdx("Gibt die Dichte der Standardnormalverteilung zurück.", `=PHI(0.75)`, x),
		"GAUSS":           tdx("Gibt die Wahrscheinlichkeit zurück, dass ein standardnormalverteilter Wert zwischen 0 und z liegt.", `=GAUSS(2)`, ta("z", "Zu untersuchender Wert.")),
		"FISHER":          tdx("Gibt die Fisher-Transformation eines Korrelationskoeffizienten zurück.", `=FISHER(0.75)`, ta("x", "Wert echt zwischen −1 und 1.")),
		"FISHERINV":       tdx("Gibt die Umkehrung der Fisher-Transformation zurück.", `=FISHERINV(0.97)`, ta("y", "Transformierter Wert.")),

		"KURT":            tdx("Gibt die Kurtosis (Wölbung) einer Reihe zurück: mindestens 4 Werte.", `=KURT(Verkäufe[Betrag])`, agg),
		"SKEW":            tdx("Gibt die Schiefe einer Reihe (Stichprobe) zurück: mindestens 3 Werte.", `=SKEW(Verkäufe[Betrag])`, agg),
		"SKEW.P":          tdx("Gibt die Schiefe einer Grundgesamtheit zurück: mindestens 3 Werte.", `=SKEW.P(Verkäufe[Betrag])`, agg),
		"AVEDEV":          tdx("Gibt den Mittelwert der absoluten Abweichungen vom Mittelwert zurück.", `=AVEDEV(Verkäufe[Betrag])`, agg),
		"DEVSQ":           tdx("Gibt die Summe der quadrierten Abweichungen vom Mittelwert zurück.", `=DEVSQ(Verkäufe[Betrag])`, agg),
		"GEOMEAN":         tdx("Gibt das geometrische Mittel positiver Zahlen zurück.", `=GEOMEAN(Sätze[Faktor])`, agg),
		"HARMEAN":         tdx("Gibt das harmonische Mittel positiver Zahlen zurück.", `=HARMEAN(Fahrten[Tempo])`, agg),
		"COVARIANCE.S":    tdx("Gibt die Stichprobenkovarianz zweier Spalten zurück.", `=COVARIANCE.S(Verkäufe[Preis], Verkäufe[Menge])`, s1, s2),
		"COVARIANCE.P":    tdx("Gibt die Kovarianz der Grundgesamtheit zweier Spalten zurück.", `=COVARIANCE.P(Verkäufe[Preis], Verkäufe[Menge])`, s1, s2),
		"RSQ":             tdx("Gibt das Bestimmtheitsmaß R² der linearen Regression zurück.", `=RSQ(Verkäufe[Betrag], Verkäufe[Menge])`, knownY, knownX),
		"STEYX":           tdx("Gibt den Standardfehler der von der linearen Regression geschätzten Werte zurück.", `=STEYX(Verkäufe[Betrag], Verkäufe[Menge])`, knownY, knownX),
		"FORECAST.LINEAR": tdx("Schätzt einen Wert mit linearer Regression.", `=FORECAST.LINEAR(30, Verkäufe[Betrag], Verkäufe[Menge])`, ta("x", "Wert, für den geschätzt wird."), knownY, knownX),
		"TREND":           tdx("Gibt den Wert der Regressionsgeraden für ein neues x zurück.", `=TREND(Verkäufe[Betrag], Verkäufe[Menge], [Menge])`, knownY, knownX, ta("Neues_x", "Wert, für den geschätzt wird, oft eine Spalte der Zeile ([Menge])."), konst),
		"GROWTH":          tdx("Gibt den Wert der angepassten Exponentialkurve für ein neues x zurück.", `=GROWTH(Verkäufe[Betrag], Verkäufe[Menge], 10)`, knownY, knownX, ta("Neues_x", "Wert, für den geschätzt wird."), ta("Konstante", "{=TRUE} oder weggelassen: normale Berechnung; {=FALSE}: Kurve mit dem Wert 1 bei 0.")),
		"LINEST":          tdx("Gibt Steigung und Achsenabschnitt der Regressionsgeraden und ihre Statistiken zurück; das Ergebnis wird übertragen.", `=LINEST(Verkäufe[Betrag], Verkäufe[Menge])`, knownY, knownX, konst, ta("Stat", "{=TRUE}: 5 Zeilen Statistik; {=FALSE} oder weggelassen: eine Zeile (in der Formel geschrieben).")),
		"LOGEST":          tdx("Gibt die Koeffizienten der angepassten Exponentialkurve und ihre Statistiken zurück; das Ergebnis wird übertragen.", `=LOGEST(Verkäufe[Betrag], Verkäufe[Menge])`, knownY, knownX, ta("Konstante", "{=TRUE} oder weggelassen: normale Berechnung; {=FALSE}: Koeffizient b gleich 1."), ta("Stat", "{=TRUE}: 5 Zeilen Statistik; {=FALSE} oder weggelassen: eine Zeile (in der Formel geschrieben).")),
		"T.TEST":          tdx("Gibt die Wahrscheinlichkeit des t-Tests nach Student für zwei Reihen zurück.", `=T.TEST(Tests[Vorher], Tests[Nachher], 2, 1)`, s1, s2, ta("Seiten", "1: einseitiger Test; 2: zweiseitig."), ta("Typ", "1: gepaarte Reihen; 2: gleiche Varianzen; 3: ungleiche Varianzen.")),
		"F.TEST":          tdx("Gibt die zweiseitige Wahrscheinlichkeit zurück, dass zwei Reihen dieselbe Varianz haben.", `=F.TEST(Tests[Vorher], Tests[Nachher])`, s1, s2),
		"Z.TEST":          tdx("Gibt die einseitige Wahrscheinlichkeit des z-Tests zurück (Mittelwert größer als x).", `=Z.TEST(Tests[Messung], 4)`, ta("Matrix", "Spalte mit Werten (Tabelle[Spalte]) oder Bereich."), ta("x", "Zu prüfender Wert."), ta("Sigma", "Standardabweichung der Grundgesamtheit (die der Stichprobe, wenn weggelassen).")),
		"CHISQ.TEST":      tdx("Gibt die Wahrscheinlichkeit des Chi-Quadrat-Unabhängigkeitstests zurück.", `=CHISQ.TEST(Cell[B2:C4], Cell[E2:F4])`, ta("Beob_Messwerte", "Beobachtete Werte: Spalte oder Bereich."), ta("Erwart_Werte", "Erwartete Werte, gleicher Form.")),
		"RANK.AVG":        tdx("Gibt den Rang eines Werts in einer Spalte zurück; gleiche Werte erhalten ihren mittleren Rang.", `=RANK.AVG([Punkte], Spieler[Punkte])`, ta("Zahl", "Einzuordnender Wert."), ta("Bezug", "Spalte, in der der Wert eingeordnet wird (Tabelle[Spalte])."), ta("Reihenfolge", "0 oder weggelassen: absteigend; 1: aufsteigend.")),
		"AGGREGATE": tdx("Berechnet ein Aggregat und ignoriert wahlweise Fehler oder vom Filter ausgeblendete Zeilen.", `=AGGREGATE(9, 6, Verkäufe[Betrag])`,
			ta("Funktion", "1 {fn:AVERAGE}, 2 {fn:COUNT}, 3 {fn:COUNTA}, 4 {fn:MAX}, 5 {fn:MIN}, 6 {fn:PRODUCT}, 7 {fn:STDEV.S}, 8 {fn:STDEV.P}, 9 {fn:SUM}, 10 {fn:VAR.S}, 11 {fn:VAR.P}, 12 {fn:MEDIAN}, 13 {fn:MODE.SNGL}, 14 {fn:LARGE}, 15 {fn:SMALL}, 16 {fn:PERCENTILE.INC}, 17 {fn:QUARTILE.INC}, 18 {fn:PERCENTILE.EXC}, 19 {fn:QUARTILE.EXC}."),
			ta("Optionen", "0 bis 7: 6 ignoriert Fehler, 5 ausgeblendete Zeilen, 7 beides (0: nichts)."),
			ta("Bezug1", "Spalte (Tabelle[Spalte]), Bereich oder Ausdruck; für 14 bis 19 gefolgt von k.")),

		"XNPV":       tdx("Gibt den Nettobarwert datierter Zahlungsströme zurück.", `=XNPV(8%, Ströme[Betrag], Ströme[Datum])`, ta("Zins", "Jährlicher Abzinsungssatz."), ta("Werte", "Spalte der Zahlungen (Tabelle[Spalte])."), ta("Zeitpkte", "Spalte der Zahlungsdaten aus derselben Tabelle; das erste ist der Ursprung.")),
		"XIRR":       tdx("Gibt den internen Zinsfuß datierter Zahlungsströme zurück.", `=XIRR(Ströme[Betrag], Ströme[Datum])`, ta("Werte", "Spalte der Zahlungen mit mindestens einer positiven und einer negativen."), ta("Zeitpkte", "Spalte der Zahlungsdaten aus derselben Tabelle."), ta("Schätzwert", "Startzins der Berechnung (10 %, wenn weggelassen).")),
		"EFFECT":     tdx("Gibt den effektiven Jahreszins eines mehrmals jährlich verzinsten Nominalzinses zurück.", `=EFFECT(5%, 12)`, ta("Nominalzins", "Nominaler Jahreszins."), ta("Perioden", "Anzahl der Zinsperioden pro Jahr.")),
		"NOMINAL":    tdx("Gibt den nominalen Jahreszins eines Effektivzinses zurück.", `=NOMINAL(5.12%, 12)`, ta("Effektiver_Zins", "Effektiver Jahreszins."), ta("Perioden", "Anzahl der Zinsperioden pro Jahr.")),
		"SLN":        tdx("Gibt die lineare Abschreibung eines Wirtschaftsguts für eine Periode zurück.", `=SLN(30000, 7500, 10)`, cost, salvage, life),
		"SYD":        tdx("Gibt die arithmetisch-degressive Abschreibung eines Wirtschaftsguts für eine Periode zurück.", `=SYD(30000, 7500, 10, 1)`, cost, salvage, life, ta("Zr", "Nummer der Periode, von 1 bis zur Nutzungsdauer.")),
		"DB":         tdx("Gibt die geometrisch-degressive Abschreibung zu festem Satz für eine Periode zurück.", `=DB(1000000, 100000, 6, 1, 7)`, cost, salvage, life, period, ta("Monate", "Monate im ersten Jahr (12, wenn weggelassen).")),
		"DDB":        tdx("Gibt die doppelt degressive (oder mit anderem Faktor) Abschreibung für eine Periode zurück.", `=DDB(2400, 300, 10, 1)`, cost, salvage, life, period, ta("Faktor", "Degressionsfaktor (2, wenn weggelassen).")),
		"VDB":        tdx("Gibt die degressive Abschreibung zwischen zwei Perioden zurück, mit Wechsel zur linearen.", `=VDB(2400, 300, 10, 0, 1)`, cost, salvage, life, ta("Anfang", "Beginn des Zeitraums (0: Kauf)."), ta("Ende", "Ende des Zeitraums."), ta("Faktor", "Degressionsfaktor (2, wenn weggelassen)."), ta("Nicht_wechseln", "{=TRUE}: wechselt nie zur linearen Abschreibung.")),
		"AMORLINC":   tdx("Gibt die lineare Abschreibung einer Periode nach französischem Buchführungssystem zurück (erste Periode anteilig).", `=AMORLINC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"AMORDEGRC":  tdx("Gibt die degressive Abschreibung einer Periode nach französischem Buchführungssystem zurück.", `=AMORDEGRC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"PRICE":      tdx("Gibt den Kurs eines Wertpapiers mit Zinsscheinen je 100 Nennwert zurück.", `=PRICE(DATE(2026,2,15), DATE(2035,11,15), 5.75%, 6.5%, 100, 2, 0)`, settle, mat, coupon, ta("Rendite", "Jährliche Rendite des Wertpapiers."), redemption, freq, basis),
		"YIELD":      tdx("Gibt die Rendite eines Wertpapiers mit Zinsscheinen aus seinem Kurs zurück.", `=YIELD(DATE(2026,2,15), DATE(2034,11,15), 5.75%, 95.04, 100, 2, 0)`, settle, mat, coupon, ta("Kurs", "Kurs je 100 Nennwert."), redemption, freq, basis),
		"ACCRINT":    tdx("Gibt die aufgelaufenen Zinsen eines Wertpapiers zurück.", `=ACCRINT(DATE(2026,3,1), DATE(2026,8,31), DATE(2026,5,1), 10%, 1000, 2, 0)`, ta("Emission", "Emissionsdatum des Wertpapiers."), ta("Erster_Zinstermin", "Datum des ersten Zinstermins."), settle, coupon, ta("Nennwert", "Nennwert des Wertpapiers."), freq, basis, ta("Berechnungsmethode", "{=TRUE} oder weggelassen: ab Emission; {=FALSE}: ab dem ersten Zinstermin.")),
		"COUPNUM":    tdx("Gibt die Anzahl der zwischen Abrechnung und Fälligkeit noch zu zahlenden Zinstermine zurück.", `=COUPNUM(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPPCD":    tdx("Gibt das Datum des Zinstermins vor der Abrechnung zurück.", `=COUPPCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPNCD":    tdx("Gibt das Datum des Zinstermins nach der Abrechnung zurück.", `=COUPNCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYS":   tdx("Gibt die Anzahl der Tage der Zinsperiode zurück, die die Abrechnung enthält.", `=COUPDAYS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYBS":  tdx("Gibt die Anzahl der Tage vom vorigen Zinstermin bis zur Abrechnung zurück.", `=COUPDAYBS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYSNC": tdx("Gibt die Anzahl der Tage von der Abrechnung bis zum nächsten Zinstermin zurück.", `=COUPDAYSNC(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),

		"DSUM":     dbText("Addiert die Werte einer Spalte für die Zeilen, die die Kriterien erfüllen.", `=DSUM(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DAVERAGE": dbText("Gibt den Mittelwert einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DAVERAGE(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DCOUNT":   dbText("Zählt die Zahlen einer Spalte für die Zeilen, die die Kriterien erfüllen.", `=DCOUNT(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DCOUNTA":  dbText("Zählt die nicht leeren Werte einer Spalte für die Zeilen, die die Kriterien erfüllen.", `=DCOUNTA(Verkäufe, "Kunde", Cell[E1:F3])`),
		"DMAX":     dbText("Gibt den größten Wert einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DMAX(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DMIN":     dbText("Gibt den kleinsten Wert einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DMIN(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DPRODUCT": dbText("Multipliziert die Werte einer Spalte für die Zeilen, die die Kriterien erfüllen.", `=DPRODUCT(Sätze, "Faktor", Cell[E1:E2])`),
		"DSTDEV":   dbText("Gibt die Stichproben-Standardabweichung einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DSTDEV(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DSTDEVP":  dbText("Gibt die Standardabweichung der Grundgesamtheit einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DSTDEVP(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DVAR":     dbText("Gibt die Stichprobenvarianz einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DVAR(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DVARP":    dbText("Gibt die Varianz der Grundgesamtheit einer Spalte für die Zeilen zurück, die die Kriterien erfüllen.", `=DVARP(Verkäufe, "Betrag", Cell[E1:F3])`),
		"DGET":     dbText("Gibt den Wert einer Spalte der einzigen Zeile zurück, die die Kriterien erfüllt.", `=DGET(Kunden, "Ort", Cell[E1:E2])`),

		"OFFSET": tdx("Gibt einen um Zeilen und Spalten verschobenen Bezug in der gewünschten Größe zurück.", `=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1], 1))`,
			ta("Bezug", "Ausgangszelle (Cell[A1]), Bereich, Spalte (Tabelle[Spalte]) oder Zeile (Tabelle[Spalte][2])."),
			ta("Zeilen", "Verschiebung nach unten (negativ: nach oben)."), ta("Spalten", "Verschiebung nach rechts (negativ: nach links)."),
			ta("Höhe", "Anzahl der Zeilen des Ergebnisses (die des Bezugs, wenn weggelassen)."),
			ta("Breite", "Anzahl der Spalten des Ergebnisses (die des Bezugs, wenn weggelassen).")),
		"INDIRECT": tdx("Gibt den in einem Text geschriebenen Bezug zurück: \"B3\", \"Blatt2!B3\", \"Cell[A1:A9]\", \"Verkäufe[Preis]\".", `=INDIRECT("Blatt2!B"&Cell[A1])`,
			ta("Bezug_Text", "Text des Bezugs; eine Adresse wie B3 bezeichnet eine freie Zelle."),
			ta("A1", "{=TRUE} oder weggelassen; {=FALSE} (Z1S1-Bezugsart) wird nicht unterstützt.")),
		"CELL": tdx("Gibt eine Information über eine Zelle zurück: \"address\", \"row\", \"col\", \"contents\" oder \"type\" (b leer, l Text, v Wert).", `=CELL("address", Cell[B3])`,
			ta("Infotyp", "Gewünschte Information, in Anführungszeichen."), ta("Bezug", "Zelle, Bereich (seine erste Zelle) oder Tabellenbezug.")),
		"TRANSPOSE": tdx("Vertauscht Zeilen und Spalten eines Zellbereichs oder einer Konstante; das Ergebnis wird übertragen.", `=TRANSPOSE(Cell[A1:C2])`,
			ta("Matrix", "Zellbereich (Cell[A1:C2]) oder Konstante ({1;2;3}).")),
	}
}
