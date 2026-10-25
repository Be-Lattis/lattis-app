package formula

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// numCase : une formule (en français) et son résultat attendu, un nombre à
// tol près (relatif au-delà de 1) ou le texte affiché d'une erreur. Les
// valeurs viennent des exemples des définitions du tableur ou d'un calcul
// fait à part.
type numCase struct {
	src  string
	want string
	tol  float64
}

// checkNum évalue chaque cas sur les deux backends (hors tableau, ou sur la
// ligne n de Ventes avec « @n » en tête).
func checkNum(t *testing.T, cases []numCase) {
	t.Helper()
	f := fixture(t)
	for _, c := range cases {
		src, row := c.src, 0
		if strings.HasPrefix(src, "@") {
			row = int(src[1] - '0')
			src = src[2:]
		}
		n, err := Parse(src, FR)
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		for _, got := range []struct {
			backend string
			v       Value
		}{{"Go", f.evalGo(n, row)}, {"SQL", f.evalSQL(t, n, row)}} {
			want, err := strconv.ParseFloat(strings.ReplaceAll(c.want, ",", "."), 64)
			if err != nil {
				if got.v.String() != c.want {
					t.Errorf("%s (%s) : %q, attendu %q", src, got.backend, got.v.String(), c.want)
				}
				continue
			}
			if got.v.Kind != Number && got.v.Kind != Date {
				t.Errorf("%s (%s) : %q, attendu %v", src, got.backend, got.v.String(), want)
				continue
			}
			tol := c.tol
			if tol == 0 {
				tol = 1e-9
			}
			if d := math.Abs(got.v.Num - want); d > tol*math.Max(1, math.Abs(want)) {
				t.Errorf("%s (%s) : %.15g, attendu %.15g (écart %g)", src, got.backend, got.v.Num, want, d)
			}
		}
	}
}

// Lois de probabilité : exemples des définitions (résultats donnés à 6 à 9
// chiffres), erreurs de domaine.
func TestDistributions(t *testing.T) {
	checkNum(t, []numCase{
		{`=LOI.NORMALE.N(42;40;1,5;VRAI)`, "0,908788780274132", 1e-12},
		{`=LOI.NORMALE.N(42;40;1,5;FAUX)`, "0,109340049783996", 1e-12},
		{`=LOI.NORMALE(42;40;1,5;VRAI)`, "0,908788780274132", 1e-12}, // ancien nom
		{`=LOI.NORMALE.N(42;40;0;VRAI)`, "#NOMBRE!", 0},
		{`=LOI.NORMALE.N("a";40;1;VRAI)`, "#VALEUR!", 0},
		{`=LOI.NORMALE.INVERSE.N(0,908789;40;1,5)`, "42,000002", 1e-7},
		{`=LOI.NORMALE.INVERSE.N(1;40;1,5)`, "#NOMBRE!", 0},
		{`=LOI.NORMALE.STANDARD.N(1,333333;VRAI)`, "0,908788726", 1e-8},
		{`=LOI.NORMALE.STANDARD.N(1,333333;FAUX)`, "0,164010148", 1e-8},
		{`=LOI.NORMALE.STANDARD(1,333333)`, "0,908788726", 1e-8},
		{`=LOI.NORMALE.STANDARD.INVERSE.N(0,908789)`, "1,3333347", 1e-6},
		{`=LOI.NORMALE.STANDARD.INVERSE.N(0)`, "#NOMBRE!", 0},
		{`=LOI.LOGNORMALE.N(4;3,5;1,2;VRAI)`, "0,0390835557", 1e-8},
		{`=LOI.LOGNORMALE.N(4;3,5;1,2;FAUX)`, "0,0176175966", 1e-8},
		{`=LOI.LOGNORMALE.INVERSE.N(0,039084;3,5;1,2)`, "4,0000252", 1e-7},
		{`=LOI.LOGNORMALE.N(0;3,5;1,2;VRAI)`, "#NOMBRE!", 0},
		{`=LOI.STUDENT.N(60;1;VRAI)`, "0,99469533", 1e-8},
		{`=LOI.STUDENT.N(8;3;FAUX)`, "0,00073691", 1e-5},
		{`=LOI.STUDENT.N(1;0;VRAI)`, "#NOMBRE!", 0},
		{`=LOI.STUDENT.BILATERALE(1,959999998;60)`, "0,054644930", 1e-8},
		{`=LOI.STUDENT.BILATERALE(-1;60)`, "#NOMBRE!", 0},
		{`=LOI.STUDENT.DROITE(1,959999998;60)`, "0,027322465", 1e-8},
		{`=LOI.STUDENT.INVERSE.N(0,75;2)`, "0,8164966", 1e-7},
		{`=LOI.STUDENT.INVERSE.BILATERALE(0,546449;60)`, "0,606533", 1e-6},
		{`=LOI.STUDENT.INVERSE.N(0;2)`, "#NOMBRE!", 0},
		{`=LOI.KHIDEUX.N(0,5;1;VRAI)`, "0,52049988", 1e-8},
		{`=LOI.KHIDEUX.N(2;3;FAUX)`, "0,20755375", 1e-8},
		{`=LOI.KHIDEUX.DROITE(18,307;10)`, "0,0500006", 1e-6},
		{`=LOI.KHIDEUX.INVERSE(0,93;1)`, "3,283020287", 1e-8},
		{`=LOI.KHIDEUX.INVERSE(0,6;2)`, "1,832581464", 1e-8},
		{`=LOI.KHIDEUX.INVERSE.DROITE(0,050001;10)`, "18,306973", 1e-6},
		{`=LOI.KHIDEUX.N(-1;1;VRAI)`, "#NOMBRE!", 0},
		{`=LOI.F.N(15,2069;6;4;VRAI)`, "0,99", 1e-5},
		{`=LOI.F.N(15,2069;6;4;FAUX)`, "0,0012238", 1e-4},
		{`=LOI.F.DROITE(15,2068649;6;4)`, "0,01", 1e-6},
		{`=INVERSE.LOI.F.N(0,01;6;4)`, "0,10930991", 1e-7},
		{`=INVERSE.LOI.F.DROITE(0,01;6;4)`, "15,20686", 1e-6},
		{`=LOI.BINOMIALE.N(6;10;0,5;FAUX)`, "0,205078125", 1e-12},
		{`=LOI.BINOMIALE.N(6;10;0,5;VRAI)`, "0,828125", 1e-12},
		{`=LOI.BINOMIALE.N(11;10;0,5;VRAI)`, "#NOMBRE!", 0},
		{`=LOI.BINOMIALE.INVERSE(6;0,5;0,75)`, "4", 0},
		{`=LOI.BINOMIALE.NEG.N(10;5;0,25;VRAI)`, "0,3135141", 1e-7},
		{`=LOI.BINOMIALE.NEG.N(10;5;0,25;FAUX)`, "0,05504866", 1e-7},
		{`=LOI.HYPERGEOMETRIQUE.N(1;4;8;20;VRAI)`, "0,4654", 1e-4},
		{`=LOI.HYPERGEOMETRIQUE.N(1;4;8;20;FAUX)`, "0,3633", 1e-4},
		{`=LOI.HYPERGEOMETRIQUE.N(5;4;8;20;FAUX)`, "#NOMBRE!", 0},
		{`=LOI.POISSON.N(2;5;VRAI)`, "0,124652019", 1e-8},
		{`=LOI.POISSON.N(2;5;FAUX)`, "0,084224337", 1e-8},
		{`=LOI.POISSON.N(-1;5;FAUX)`, "#NOMBRE!", 0},
		{`=LOI.EXPONENTIELLE.N(0,2;10;VRAI)`, "0,864664717", 1e-8},
		{`=LOI.EXPONENTIELLE.N(0,2;10;FAUX)`, "1,353352832", 1e-8},
		{`=LOI.EXPONENTIELLE.N(0,2;0;FAUX)`, "#NOMBRE!", 0},
		{`=LOI.GAMMA.N(10,00001131;9;2;FAUX)`, "0,032639", 1e-5},
		{`=LOI.GAMMA.N(10,00001131;9;2;VRAI)`, "0,068094", 1e-5},
		{`=LOI.GAMMA.INVERSE.N(0,068094;9;2)`, "10,0000112", 1e-6},
		{`=LNGAMMA(4)`, "1,791759469", 1e-9},
		{`=LNGAMMA.PRECIS(4)`, "1,791759469", 1e-9},
		{`=LNGAMMA(0)`, "#NOMBRE!", 0},
		{`=GAMMA(2,5)`, "1,329340388", 1e-9},
		{`=GAMMA(-1)`, "#NOMBRE!", 0},
		{`=LOI.BETA.N(2;8;10;VRAI;1;3)`, "0,6854706", 1e-7},
		{`=LOI.BETA.N(2;8;10;FAUX;1;3)`, "1,4837646", 1e-6},
		{`=BETA.INVERSE.N(0,685470581;8;10;1;3)`, "2", 1e-8},
		{`=LOI.BETA.N(4;8;10;VRAI;1;3)`, "#NOMBRE!", 0},
		{`=LOI.WEIBULL.N(105;20;100;VRAI)`, "0,929581", 1e-6},
		{`=LOI.WEIBULL.N(105;20;100;FAUX)`, "0,035589", 1e-5},
		{`=INTERVALLE.CONFIANCE.NORMAL(0,05;2,5;50)`, "0,692951912", 1e-8},
		{`=INTERVALLE.CONFIANCE.STUDENT(0,05;1;50)`, "0,284196855", 1e-8},
		{`=INTERVALLE.CONFIANCE.STUDENT(0,05;1;1)`, "#DIV/0!", 0},
		{`=INTERVALLE.CONFIANCE.NORMAL(0;2,5;50)`, "#NOMBRE!", 0},
		{`=CENTREE.REDUITE(42;40;1,5)`, "1,333333333", 1e-9},
		{`=PHI(0,75)`, "0,301137432", 1e-9},
		{`=GAUSS(2)`, "0,477249868", 1e-9},
		{`=FISHER(0,75)`, "0,972955075", 1e-9},
		{`=FISHER(1)`, "#NOMBRE!", 0},
		{`=FISHER.INVERSE(0,972955)`, "0,75", 1e-6},
	})
}

// Statistiques sur colonnes : exemples des définitions (données de Mesures).
func TestColumnStatistics(t *testing.T) {
	checkNum(t, []numCase{
		// T.TEST({3;4;5;8;9;1;2;4;5}; {6;19;3;2;14;4;5;17;1}; 2; 1) = 0,196016.
		{`=T.TEST(Mesures[A];Mesures[B];2;1)`, "0,196015785", 1e-8},
		{`=T.TEST(Mesures[A];Mesures[B];1;2)`, "0,095997943", 1e-8},
		{`=T.TEST(Mesures[A];Mesures[B];2;3)`, "0,202293923", 1e-8},
		{`=T.TEST(Mesures[A];Mesures[B];3;1)`, "#NOMBRE!", 0},
		{`=Z.TEST(Mesures[Z];4)`, "0,090574", 1e-6},
		{`=Z.TEST(Mesures[Z];6)`, "0,863043", 1e-6},
		{`=F.TEST(Mesures[A];Mesures[B])`, "0,0128", 1e-3},
		// PREVISION(30; {6;7;9;15;21}; {20;28;31;38;40}) = 10,607253.
		{`=PREVISION.LINEAIRE(30;Mesures[Y];Mesures[X])`, "10,607253", 1e-7},
		{`=PREVISION(30;Mesures[Y];Mesures[X])`, "10,607253", 1e-7},
		{`=TENDANCE(Mesures[Y];Mesures[X];30)`, "10,607253", 1e-7},
		{`=TENDANCE(Mesures[Y];Mesures[X];30;FAUX)`, "11,591828869", 1e-9},
		{`=CROISSANCE(Mesures[Y];Mesures[X];30)`, "9,491005345", 1e-9},
		{`=PREVISION.LINEAIRE("a";Mesures[Y];Mesures[X])`, "#VALEUR!", 0},
		{`=PREVISION.LINEAIRE(1;Mesures[Y];Mesures[X]*0)`, "#DIV/0!", 0},
		{`=COVARIANCE.PEARSON(Mesures[Y];Mesures[X])`, "36,76", 1e-12},
		{`=COVARIANCE(Mesures[Y];Mesures[X])`, "36,76", 1e-12},
		{`=COVARIANCE.STANDARD(Mesures[Y];Mesures[X])`, "45,95", 1e-12},
		{`=COEFFICIENT.DETERMINATION(Mesures[Y];Mesures[X])`, "0,818677686", 1e-8},
		{`=ERREUR.TYPE.XY(Mesures[Y];Mesures[X])`, "3,101962408", 1e-8},
		{`=PEARSON(Mesures[Y];Mesures[X])`, "0,904808094", 1e-8},
		{`=KURTOSIS(3;4;5;2;3;4;5;6;4;7)`, "-0,151799637", 1e-9},
		{`=KURTOSIS(1;2;3)`, "#DIV/0!", 0},
		{`=COEFFICIENT.ASYMETRIE(3;4;5;2;3;4;5;6;4;7)`, "0,359543", 1e-6},
		{`=COEFFICIENT.ASYMETRIE.P(3;4;5;2;3;4;5;6;4;7)`, "0,303193", 1e-6},
		{`=COEFFICIENT.ASYMETRIE(1;1)`, "#DIV/0!", 0},
		{`=ECART.MOYEN(4;5;6;7;5;4;3)`, "1,020408", 1e-6},
		{`=SOMME.CARRES.ECARTS(4;5;8;7;11;4;3)`, "48", 1e-12},
		{`=MOYENNE.GEOMETRIQUE(4;5;8;7;11;4;3)`, "5,476987", 1e-6},
		{`=MOYENNE.HARMONIQUE(4;5;8;7;11;4;3)`, "5,028376", 1e-6},
		{`=MOYENNE.GEOMETRIQUE(4;0)`, "#NOMBRE!", 0},
		{`=KURTOSIS(Mesures[Z])`, "-0,944211216", 1e-8},
		{`=MOYENNE.RANG(5;Mesures[A])`, "3,5", 0},
		{`=MOYENNE.RANG(5;Mesures[A];1)`, "6,5", 0},
		{`=MOYENNE.RANG(7;Mesures[A])`, "#N/A", 0},
		{`=EQUATION.RANG(5;Mesures[A])`, "3", 0},
		{`=CHISQ.TEST(Cell[B1:B3];Cell[B1:B3]*1)`, "1", 1e-12},
		{`=CHISQ.TEST(Mesures[A];Mesures[A]*0)`, "#DIV/0!", 0},
	})
}

// Finances : exemples des définitions du tableur.
func TestFinanceAdded(t *testing.T) {
	checkNum(t, []numCase{
		{`=AMORLIN(30000;7500;10)`, "2250", 0},
		{`=AMORLIN(30000;7500;0)`, "#DIV/0!", 0},
		{`=SYD(30000;7500;10;1)`, "4090,909090909", 1e-9},
		{`=SYD(30000;7500;10;10)`, "409,0909090909", 1e-9},
		{`=SYD(30000;7500;10;11)`, "#NOMBRE!", 0},
		{`=DB(1000000;100000;6;1;7)`, "186083,333333", 1e-9},
		{`=DB(1000000;100000;6;2;7)`, "259639,416666", 1e-9},
		{`=DB(1000000;100000;6;7;7)`, "15845,1048391", 1e-6},
		{`=DB(1000000;100000;6;8;7)`, "#NOMBRE!", 0},
		{`=DDB(2400;300;10*365;1)`, "1,315068493", 1e-9},
		{`=DDB(2400;300;10*12;1;2)`, "40", 1e-12},
		{`=DDB(2400;300;10;1;2)`, "480", 1e-12},
		{`=DDB(2400;300;10;2;1,5)`, "306", 1e-12},
		{`=DDB(2400;300;10;10)`, "22,1225472", 1e-6},
		{`=DDB(2400;300;10;11)`, "#NOMBRE!", 0},
		{`=VDB(2400;300;10*365;0;1)`, "1,315068493", 1e-9},
		{`=VDB(2400;300;10*12;0;1)`, "40", 1e-12},
		{`=VDB(2400;300;10;0;1)`, "480", 1e-12},
		{`=VDB(2400;300;10*12;6;18)`, "396,306", 1e-5},
		{`=VDB(2400;300;10*12;6;18;1,5)`, "311,8089366", 1e-6},
		{`=VDB(2400;300;10;0;0,875;1,5)`, "315", 1e-9},
		{`=VDB(2400;300;10;0;10;2;VRAI)`, "2100", 1e-9},
		{`=VDB(2400;300;10;0;10)`, "2100", 1e-9},
		{`=VDB(2400;300;10;3;2)`, "#NOMBRE!", 0},
		{`=AMORLINC(2400;DATE(2008;8;19);DATE(2008;12;31);300;1;0,15;1)`, "360", 1e-9},
		{`=AMORLINC(2400;DATE(2008;8;19);DATE(2008;12;31);300;0;0,15;1)`, "131,803278688", 1e-9},
		{`=AMORDEGRC(2400;DATE(2008;8;19);DATE(2008;12;31);300;1;0,15;1)`, "776", 0},
		{`=AMORDEGRC(2400;DATE(2008;8;19);DATE(2008;12;31);300;0;0,15;1)`, "330", 0},
		{`=AMORDEGRC(2400;DATE(2008;8;19);DATE(2008;12;31);300;1;0,15;2)`, "#NOMBRE!", 0},
		{`=TAUX.EFFECTIF(5,25%;4)`, "0,053542667", 1e-9},
		{`=TAUX.EFFECTIF(0;4)`, "#NOMBRE!", 0},
		{`=TAUX.NOMINAL(5,3543%;4)`, "0,052500319", 1e-9},
		{`=PRIX.TITRE(DATE(2008;2;15);DATE(2017;11;15);5,75%;6,5%;100;2;0)`, "94,6343616", 1e-9},
		{`=RENDEMENT.TITRE(DATE(2008;2;15);DATE(2016;11;15);5,75%;95,04287;100;2;0)`, "0,065", 1e-6},
		{`=PRIX.TITRE(DATE(2008;2;15);DATE(2008;2;15);5,75%;6,5%;100;2;0)`, "#NOMBRE!", 0},
		{`=PRIX.TITRE(DATE(2008;2;15);DATE(2017;11;15);5,75%;6,5%;100;3;0)`, "#NOMBRE!", 0},
		{`=INTERET.ACC(DATE(2008;3;1);DATE(2008;8;31);DATE(2008;5;1);10%;1000;2;0)`, "16,666666667", 1e-9},
		{`=INTERET.ACC(DATE(2008;3;5);DATE(2008;8;31);DATE(2008;5;1);10%;1000;2;0;FAUX)`, "15,555555556", 1e-9},
		{`=NB.COUPONS(DATE(2011;1;25);DATE(2011;11;15);2;1)`, "2", 0},
		{`=DATE.COUPON.PREC(DATE(2011;1;25);DATE(2011;11;15);2;1)*1`, "40497", 0},
		{`=DATE.COUPON.SUIV(DATE(2011;1;25);DATE(2011;11;15);2;1)*1`, "40678", 0},
		{`=NB.JOURS.COUPONS(DATE(2011;1;25);DATE(2011;11;15);2;1)`, "181", 0},
		{`=NB.JOURS.COUPON.PREC(DATE(2011;1;25);DATE(2011;11;15);2;1)`, "71", 0},
		{`=NB.JOURS.COUPON.SUIV(DATE(2011;1;25);DATE(2011;11;15);2;1)`, "110", 0},
		{`=NB.JOURS.COUPONS(DATE(2011;1;25);DATE(2011;11;15);2;0)`, "180", 0},
		{`=VAN.PAIEMENTS(0,09;Flux[Montant];Flux[Jour])`, "2086,647602", 1e-9},
		{`=TRI.PAIEMENTS(Flux[Montant];Flux[Jour])`, "0,373362535", 1e-8},
		{`=TRI.PAIEMENTS(Flux[Montant]*0+1;Flux[Jour])`, "#NOMBRE!", 0},
		{`=VAN.PAIEMENTS(-1;Flux[Montant];Flux[Jour])`, "#NOMBRE!", 0},
	})
}

// CHISQ.TEST sur un tableau de contingence (3 lignes × 2 colonnes) :
// exemple de la définition, 0,0003082.
func TestChiTestContingency(t *testing.T) {
	c := &chiTest{rows: 3, cols: 2}
	actual := []float64{58, 35, 11, 25, 10, 23}
	expected := []float64{45.35, 47.65, 17.56, 18.44, 16.09, 16.91}
	for i := range actual {
		c.add([]Value{Num(actual[i]), Num(expected[i])})
	}
	if v := c.result(); math.Abs(v.Num-0.000308192) > 1e-9 {
		t.Errorf("CHISQ.TEST : %v", v)
	}
}
