package formula

import (
	"math"
	"time"
)

// Amortissements (AMORLIN, SYD, DB, DDB, VDB, AMORLINC, AMORDEGRC), taux
// (TAUX.EFFECTIF, TAUX.NOMINAL), titres à coupons (PRIX.TITRE,
// RENDEMENT.TITRE, INTERET.ACC, famille des coupons) et flux datés
// (VAN.PAIEMENTS, TRI.PAIEMENTS), d'après les définitions du tableur.

// --- Amortissements ------------------------------------------------------------------

// fnSln : AMORLIN(coût; valeur_rés; durée).
func fnSln(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	if x[2] == 0 {
		return Err(ErrDiv0)
	}
	return number((x[0] - x[1]) / x[2])
}

// fnSyd : SYD(coût; valeur_rés; durée; période), amortissement par
// l'américaine « somme des chiffres de l'année ».
func fnSyd(a []Value) Value {
	x, e := distArgs(a, 4)
	if e.IsErr() {
		return e
	}
	c, s, l, p := x[0], x[1], x[2], x[3]
	if l <= 0 || p <= 0 || p > l || s < 0 {
		return Err(ErrNum)
	}
	return number((c - s) * (l - p + 1) * 2 / (l * (l + 1)))
}

// fnDb : DB(coût; valeur_rés; durée; période; [mois]), dégressif à taux fixe
// (taux arrondi à trois décimales, première année au prorata des mois).
func fnDb(a []Value) Value {
	x, e := distArgs(a, 5)
	if e.IsErr() {
		return e
	}
	c, s, l, p, m := x[0], x[1], x[2], math.Trunc(x[3]), 12.0
	if len(a) > 4 && !a[4].IsEmpty() {
		m = math.Trunc(x[4])
	}
	if c < 0 || s < 0 || l <= 0 || p <= 0 || m < 1 || m > 12 || p > l+1 || m == 12 && p > l {
		return Err(ErrNum)
	}
	if c == 0 {
		return Num(0)
	}
	rate := math.Round((1-math.Pow(s/c, 1/l))*1000) / 1000
	total := c * rate * m / 12
	if p == 1 {
		return number(total)
	}
	var dep float64
	for i := 2.0; i <= p; i++ {
		if i == l+1 {
			dep = (c - total) * rate * (12 - m) / 12
		} else {
			dep = (c - total) * rate
		}
		total += dep
	}
	return number(dep)
}

// ddb : amortissement dégressif de la période p (réelle), au coefficient f.
func ddb(c, s, l, p, f float64) float64 {
	rate := f / l
	var old float64
	if rate >= 1 {
		rate = 1
		if p == 1 {
			old = c
		}
	} else {
		old = c * math.Pow(1-rate, p-1)
	}
	next := c * math.Pow(1-rate, p)
	d := old - next
	if next < s {
		d = old - s
	}
	return math.Max(d, 0)
}

// fnDdb : DDB(coût; valeur_rés; durée; période; [facteur]), dégressif à
// taux double (facteur 2 si omis).
func fnDdb(a []Value) Value {
	x, e := distArgs(a, 5)
	if e.IsErr() {
		return e
	}
	f := 2.0
	if len(a) > 4 && !a[4].IsEmpty() {
		f = x[4]
	}
	c, s, l, p := x[0], x[1], x[2], x[3]
	if c < 0 || s < 0 || l <= 0 || p <= 0 || f <= 0 || p > l {
		return Err(ErrNum)
	}
	return number(ddb(c, s, l, p, f))
}

// interVDB : amortissement cumulé des périodes 1 à period (dernière au
// prorata), dégressif puis linéaire quand le linéaire devient plus fort.
func interVDB(c, s, life, life1, period, f float64) float64 {
	rest := c - s
	var sln, total float64
	linear := false
	end := int(math.Ceil(period))
	for i := 1; i <= end; i++ {
		var term float64
		if !linear {
			d := ddb(c, s, life, float64(i), f)
			sln = rest / (life1 - float64(i-1))
			if sln > d {
				term, linear = sln, true
			} else {
				term = d
				rest -= d
			}
		} else {
			term = sln
		}
		if i == end {
			term *= period + 1 - float64(end)
		}
		total += term
	}
	return total
}

// fnVdb : VDB(coût; valeur_rés; durée; période_début; période_fin; [facteur];
// [pas_de_bascule]), amortissement entre deux périodes (fractions
// comprises), avec bascule vers le linéaire sauf si pas_de_bascule.
func fnVdb(a []Value) Value {
	x, e := distArgs(a, 6)
	if e.IsErr() {
		return e
	}
	f := 2.0
	if len(a) > 5 && !a[5].IsEmpty() {
		f = x[5]
	}
	noSwitch := false
	if len(a) > 6 && !a[6].IsEmpty() {
		b, e := Truth(a[6])
		if e.IsErr() {
			return e
		}
		noSwitch = b
	}
	c, s, l, start, end := x[0], x[1], x[2], x[3], x[4]
	if start < 0 || end < start || end > l || c < 0 || s > c || f <= 0 || l <= 0 {
		return Err(ErrNum)
	}
	is, ie := math.Floor(start), math.Ceil(end)
	if noSwitch {
		total := 0.0
		for i := is + 1; i <= ie; i++ {
			term := ddb(c, s, l, i, f)
			switch {
			case i == is+1:
				term *= math.Min(end, is+1) - start
			case i == ie:
				term *= end + 1 - ie
			}
			total += term
		}
		return number(total)
	}
	part := 0.0
	if start != is {
		v := c - interVDB(c, s, l, l, is, f)
		part += (start - is) * interVDB(v, s, l, l-is, 1, f)
	}
	if end != ie {
		ts := ie - 1
		v := c - interVDB(c, s, l, l, ts, f)
		part += (ie - end) * interVDB(v, s, l, l-ts, 1, f)
	}
	c -= interVDB(c, s, l, l, is, f)
	return number(interVDB(c, s, l, l-is, ie-is, f) - part)
}

// amorArgs lit les arguments de AMORLINC et AMORDEGRC : coût, date d'achat,
// fin de la première période, valeur résiduelle, période, taux, [base] ; et
// la part d'année de la première période.
func amorArgs(a []Value) (c, s, per, rate, first float64, err Value) {
	x, e := distArgs(a, 7)
	if e.IsErr() {
		return 0, 0, 0, 0, 0, e
	}
	basis := 0.0
	if len(a) > 6 && !a[6].IsEmpty() {
		basis = math.Trunc(x[6])
	}
	c, s, per, rate = x[0], x[3], math.Trunc(x[4]), x[5]
	if basis < 0 || basis > 4 || basis == 2 || c < 0 || s < 0 || s > c || per < 0 || rate <= 0 || x[1] > x[2] {
		return 0, 0, 0, 0, 0, Err(ErrNum)
	}
	yf := fnYearFrac([]Value{a[1], a[2], Num(basis)})
	if yf.IsErr() {
		return 0, 0, 0, 0, 0, yf
	}
	return c, s, per, rate, yf.Num, Value{}
}

// fnAmorlinc : AMORLINC(coût; date_achat; première_pér; valeur_rés; période;
// taux; [base]), amortissement linéaire à la française, première période au
// prorata.
func fnAmorlinc(a []Value) Value {
	c, s, per, rate, first, e := amorArgs(a)
	if e.IsErr() {
		return e
	}
	one := c * rate
	f0 := first * rate * c
	full := math.Floor((c - s - f0) / one)
	var r float64
	switch {
	case per == 0:
		r = f0
	case per <= full:
		r = one
	case per == full+1:
		r = c - s - one*full - f0
	}
	return number(math.Max(r, 0))
}

// fnAmordegrc : AMORDEGRC(…), amortissement dégressif à la française :
// coefficient 1,5, 2 ou 2,5 selon la durée (1/taux), montants arrondis à
// l'unité, la moitié du reste à l'avant-dernière période.
func fnAmordegrc(a []Value) Value {
	c, s, per, rate, first, e := amorArgs(a)
	if e.IsErr() {
		return e
	}
	life := 1 / rate
	var coef float64
	switch {
	case life < 3:
		if life > 0 && life != 1 && life != 2 {
			return Err(ErrNum) // durées de 0 à 1, 1 à 2, 2 à 3 ans
		}
		coef = 1
	case life < 4:
		coef = 1.5
	case life < 5:
		if life != 4 {
			return Err(ErrNum) // durée de 4 à 5 ans
		}
		coef = 1.5
	case life <= 6:
		coef = 2
	default:
		coef = 2.5
	}
	rate *= coef
	round := func(f float64) float64 { return math.Copysign(math.Floor(math.Abs(f)+0.5), f) }
	dep := round(first * rate * c)
	c -= dep
	rest := c - s
	for n := 0.0; n < per; n++ {
		dep = round(rate * c)
		rest -= dep
		if rest < 0 {
			if per-n <= 1 {
				return number(round(c * 0.5))
			}
			return Num(0)
		}
		c -= dep
	}
	return number(dep)
}

// --- Taux ------------------------------------------------------------------------------

// fnEffect : TAUX.EFFECTIF(taux_nominal; nb_périodes).
func fnEffect(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	r, n := x[0], math.Trunc(x[1])
	return numOrErr(r <= 0 || n < 1, func() float64 { return math.Pow(1+r/n, n) - 1 })
}

// fnNominal : TAUX.NOMINAL(taux_effectif; nb_périodes).
func fnNominal(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	r, n := x[0], math.Trunc(x[1])
	return numOrErr(r <= 0 || n < 1, func() float64 { return n * (math.Pow(1+r, 1/n) - 1) })
}

// --- Coupons ---------------------------------------------------------------------------

// bond : règlement, échéance, fréquence et base d'un titre à coupons.
type bond struct {
	settle, mat time.Time
	freq        int
	basis       int
}

// bondArgs lit règlement, échéance, puis (aux rangs fi et bi) la fréquence
// (1, 2 ou 4) et la base facultative (0 à 4).
func bondArgs(a []Value, fi, bi int) (bond, Value) {
	var b bond
	s, e := toSerial(a[0])
	if e.IsErr() {
		return b, e
	}
	m, e := toSerial(a[1])
	if e.IsErr() {
		return b, e
	}
	f, e := ToNumber(a[fi])
	if e.IsErr() {
		return b, e
	}
	basis := 0.0
	if bi < len(a) && !a[bi].IsEmpty() {
		if basis, e = ToNumber(a[bi]); e.IsErr() {
			return b, e
		}
	}
	b.freq, b.basis = int(math.Trunc(f)), int(math.Trunc(basis))
	s, m = math.Floor(s), math.Floor(m)
	if s >= m || b.freq != 1 && b.freq != 2 && b.freq != 4 || b.basis < 0 || b.basis > 4 {
		return b, Err(ErrNum)
	}
	b.settle, b.mat = SerialTime(s), SerialTime(m)
	return b, Value{}
}

func endOfMonth(t time.Time) bool { return t.AddDate(0, 0, 1).Day() == 1 }

// couponAt : la date de coupon k périodes avant l'échéance (fin de mois si
// l'échéance en est une).
func (b bond) couponAt(k int) time.Time {
	d := addMonths(b.mat, -k*12/b.freq)
	if endOfMonth(b.mat) {
		d = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)
	}
	return d
}

// coupons : dates de coupon qui encadrent le règlement (précédente ≤
// règlement < suivante) et nombre de coupons restant à payer.
func (b bond) coupons() (pcd, ncd time.Time, n int) {
	k := 0
	for b.couponAt(k + 1).After(b.settle) {
		k++
	}
	return b.couponAt(k + 1), b.couponAt(k), k + 1
}

// days30 : jours entre deux dates en 30/360 (américain, règles NASD, ou
// européen).
func days30(t1, t2 time.Time, european bool) float64 {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	if european {
		d1, d2 = min(d1, 30), min(d2, 30)
	} else {
		lastFeb := func(t time.Time) bool { return t.Month() == 2 && endOfMonth(t) }
		if lastFeb(t1) && lastFeb(t2) {
			d2 = 30
		}
		if lastFeb(t1) {
			d1 = 30
		}
		if d2 == 31 && d1 >= 30 {
			d2 = 30
		}
		if d1 == 31 {
			d1 = 30
		}
	}
	return float64((y2-y1)*360 + (int(m2)-int(m1))*30 + d2 - d1)
}

// dayCount : jours entre deux dates selon la base.
func (b bond) dayCount(t1, t2 time.Time) float64 {
	switch b.basis {
	case 0:
		return days30(t1, t2, false)
	case 4:
		return days30(t1, t2, true)
	}
	return math.Round(t2.Sub(t1).Hours() / 24)
}

// daysBS, days, daysNC : jours du coupon précédent au règlement, jours de la
// période de coupon, jours du règlement au coupon suivant.
func (b bond) daysBS() float64 {
	pcd, _, _ := b.coupons()
	return b.dayCount(pcd, b.settle)
}

func (b bond) days() float64 {
	switch b.basis {
	case 1:
		pcd, ncd, _ := b.coupons()
		return math.Round(ncd.Sub(pcd).Hours() / 24)
	case 3:
		return 365 / float64(b.freq)
	}
	return 360 / float64(b.freq)
}

func (b bond) daysNC() float64 {
	if b.basis != 0 && b.basis != 4 {
		_, ncd, _ := b.coupons()
		return math.Round(ncd.Sub(b.settle).Hours() / 24)
	}
	return b.days() - b.daysBS()
}

// fnCoupon : NB.COUPONS, DATE.COUPON.PREC, DATE.COUPON.SUIV, NB.JOURS.COUPONS,
// NB.JOURS.COUPON.PREC et NB.JOURS.COUPON.SUIV(règlement; échéance;
// fréquence; [base]).
func fnCoupon(what string) func([]Value) Value {
	return func(a []Value) Value {
		b, e := bondArgs(a, 2, 3)
		if e.IsErr() {
			return e
		}
		pcd, ncd, n := b.coupons()
		switch what {
		case "NUM":
			return Num(float64(n))
		case "PCD":
			return dateNumber(TimeSerial(pcd))
		case "NCD":
			return dateNumber(TimeSerial(ncd))
		case "DAYS":
			return number(b.days())
		case "DAYBS":
			return number(b.daysBS())
		}
		return number(b.daysNC())
	}
}

// price : prix d'un titre pour 100 de nominal au rendement y.
func (b bond) price(rate, y, redemption float64) float64 {
	_, _, n := b.coupons()
	f := float64(b.freq)
	e, dsc, ac := b.days(), b.daysNC(), b.daysBS()
	coupon := 100 * rate / f
	if n == 1 {
		return (coupon+redemption)/(1+dsc/e*y/f) - coupon*ac/e
	}
	p := redemption / math.Pow(1+y/f, float64(n-1)+dsc/e)
	for k := 1; k <= n; k++ {
		p += coupon / math.Pow(1+y/f, float64(k-1)+dsc/e)
	}
	return p - coupon*ac/e
}

// fnPrice : PRIX.TITRE(règlement; échéance; taux; rendement; valeur_échéance;
// fréquence; [base]).
func fnPrice(a []Value) Value {
	b, e := bondArgs(a, 5, 6)
	if e.IsErr() {
		return e
	}
	x, e := distArgs(a[2:5], 3)
	if e.IsErr() {
		return e
	}
	rate, y, red := x[0], x[1], x[2]
	return numOrErr(rate < 0 || y < 0 || red <= 0, func() float64 { return b.price(rate, y, red) })
}

// fnYield : RENDEMENT.TITRE(règlement; échéance; taux; prix; valeur_échéance;
// fréquence; [base]) : formule directe avec un seul coupon restant, sinon
// méthode de Newton sur PRIX.TITRE.
func fnYield(a []Value) Value {
	b, e := bondArgs(a, 5, 6)
	if e.IsErr() {
		return e
	}
	x, e := distArgs(a[2:5], 3)
	if e.IsErr() {
		return e
	}
	rate, pr, red := x[0], x[1], x[2]
	if rate < 0 || pr <= 0 || red <= 0 {
		return Err(ErrNum)
	}
	if _, _, n := b.coupons(); n == 1 {
		f := float64(b.freq)
		e, dsr, ac := b.days(), b.daysNC(), b.daysBS()
		num := red/100 + rate/f - (pr/100 + ac/e*rate/f)
		return number(num / (pr/100 + ac/e*rate/f) * f * e / dsr)
	}
	return solve(0.05, func(y float64) float64 { return b.price(rate, y, red) - pr })
}

// fnAccrint : INTERET.ACC(émission; premier_coupon; règlement; taux; valeur_nominale;
// fréquence; [base]; [méthode_calcul]) : valeur_nominale × taux × part
// d'année (selon la base) depuis l'émission, ou depuis le premier coupon si
// méthode_calcul est FAUX et que le règlement le suit.
func fnAccrint(a []Value) Value {
	issue, e := toSerial(a[0])
	if e.IsErr() {
		return e
	}
	firstInt, e := toSerial(a[1])
	if e.IsErr() {
		return e
	}
	settle, e := toSerial(a[2])
	if e.IsErr() {
		return e
	}
	x, e := distArgs(a[3:6], 3)
	if e.IsErr() {
		return e
	}
	basis := 0.0
	if len(a) > 6 && !a[6].IsEmpty() {
		if basis, e = ToNumber(a[6]); e.IsErr() {
			return e
		}
	}
	fromIssue := true
	if len(a) > 7 && !a[7].IsEmpty() {
		if fromIssue, e = Truth(a[7]); e.IsErr() {
			return e
		}
	}
	rate, par, freq := x[0], x[1], math.Trunc(x[2])
	basis = math.Trunc(basis)
	issue, firstInt, settle = math.Floor(issue), math.Floor(firstInt), math.Floor(settle)
	if rate <= 0 || par <= 0 || freq != 1 && freq != 2 && freq != 4 || basis < 0 || basis > 4 || issue >= settle {
		return Err(ErrNum)
	}
	start := issue
	if !fromIssue && settle > firstInt {
		start = firstInt
	}
	yf := fnYearFrac([]Value{DateSerial(start), DateSerial(settle), Num(basis)})
	if yf.IsErr() {
		return yf
	}
	return number(par * rate * yf.Num)
}

// --- Flux datés ---------------------------------------------------------------------

// datedFlows : VAN.PAIEMENTS(taux; valeurs; dates) et TRI.PAIEMENTS(valeurs;
// dates; [estimation]) lisent les paires (valeur, date) ligne à ligne. Une
// ligne vide est sautée ; une valeur sans date (ou l'inverse) donne
// #NOMBRE!, une date antérieure à la première aussi.
type datedFlows struct {
	irr   bool
	param Value // taux de VAN.PAIEMENTS, estimation de TRI.PAIEMENTS
	vals  []float64
	days  []float64
	err   Value
	numRule
}

func (s *datedFlows) add(cols []Value) {
	if s.err.IsErr() {
		return
	}
	v, d := s.coerce(cols[0]), cols[1]
	switch {
	case v.IsErr():
		s.err = v
		return
	case d.IsErr():
		s.err = d
		return
	case blank(v) && blank(d):
		return
	case blank(v) || blank(d):
		s.err = Err(ErrNum)
		return
	}
	if v.Kind != Number && v.Kind != Date {
		s.err = Err(ErrValue)
		return
	}
	day, e := toSerial(d)
	if e.IsErr() {
		s.err = e
		return
	}
	s.vals, s.days = append(s.vals, v.Num), append(s.days, math.Floor(day))
}

func (s *datedFlows) xnpv(r float64) float64 {
	t := 0.0
	for i, v := range s.vals {
		t += v / math.Pow(1+r, (s.days[i]-s.days[0])/365)
	}
	return t
}

func (s *datedFlows) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	if len(s.vals) == 0 {
		return Err(ErrNum)
	}
	for _, d := range s.days {
		if d < s.days[0] {
			return Err(ErrNum)
		}
	}
	if !s.irr {
		r, e := ToNumber(s.param)
		if e.IsErr() {
			return e
		}
		if r <= -1 {
			return Err(ErrNum)
		}
		return number(s.xnpv(r))
	}
	r := 0.1
	if !s.param.IsEmpty() {
		g, e := ToNumber(s.param)
		if e.IsErr() {
			return e
		}
		r = g
	}
	pos, neg := false, false
	for _, v := range s.vals {
		pos = pos || v > 0
		neg = neg || v < 0
	}
	if !pos || !neg {
		return Err(ErrNum)
	}
	return solve(r, s.xnpv)
}
