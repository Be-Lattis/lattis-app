package formula

import "math"

// Lois de probabilité (LOI.NORMALE.N, LOI.STUDENT.N, LOI.KHIDEUX.N…), avec
// les conventions et les erreurs du tableur : un paramètre hors de son
// domaine donne #NOMBRE!, les degrés de liberté et les nombres de succès
// sont tronqués. Les fonctions de répartition reposent sur les fonctions
// gamma et bêta incomplètes régularisées (séries et fractions continues de
// Lentz, précision de l'ordre de 1e-14), les inverses sur une dichotomie
// menée jusqu'à la précision de la machine.

// --- Fonctions spéciales -------------------------------------------------------

// normCDF : fonction de répartition de la loi normale centrée réduite.
func normCDF(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }

// normPDF : densité de la loi normale centrée réduite.
func normPDF(z float64) float64 { return math.Exp(-z*z/2) / math.Sqrt(2*math.Pi) }

// normInv : quantile de la loi normale centrée réduite (0 < p < 1).
func normInv(p float64) float64 { return -math.Sqrt2 * math.Erfcinv(2*p) }

func lgamma(x float64) float64 {
	v, _ := math.Lgamma(x)
	return v
}

// gammaP : fonction gamma incomplète régularisée P(a, x).
func gammaP(a, x float64) float64 {
	switch {
	case x <= 0:
		return 0
	case math.IsInf(x, 1):
		return 1
	case x < a+1:
		return gammaSeries(a, x)
	}
	return 1 - gammaFrac(a, x)
}

// gammaQ : 1 − P(a, x), calculée sans perte de précision dans la queue.
func gammaQ(a, x float64) float64 {
	switch {
	case x <= 0:
		return 1
	case x < a+1:
		return 1 - gammaSeries(a, x)
	}
	return gammaFrac(a, x)
}

func gammaSeries(a, x float64) float64 {
	sum, del, ap := 1/a, 1/a, a
	for range 10000 {
		ap++
		del *= x / ap
		sum += del
		if math.Abs(del) < math.Abs(sum)*1e-16 {
			break
		}
	}
	return sum * math.Exp(-x+a*math.Log(x)-lgamma(a))
}

func gammaFrac(a, x float64) float64 {
	const tiny = 1e-300
	b := x + 1 - a
	c := 1 / tiny
	d := 1 / b
	h := d
	for i := 1; i < 10000; i++ {
		an := -float64(i) * (float64(i) - a)
		b += 2
		d = an*d + b
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = b + an/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-16 {
			break
		}
	}
	return math.Exp(-x+a*math.Log(x)-lgamma(a)) * h
}

// betaI : fonction bêta incomplète régularisée I_x(a, b).
func betaI(x, a, b float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	}
	bt := math.Exp(lgamma(a+b) - lgamma(a) - lgamma(b) + a*math.Log(x) + b*math.Log1p(-x))
	if x < (a+1)/(a+b+2) {
		return bt * betaFrac(x, a, b) / a
	}
	return 1 - bt*betaFrac(1-x, b, a)/b
}

func betaFrac(x, a, b float64) float64 {
	const tiny = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c, d := 1.0, 1-qab*x/qap
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m < 10000; m++ {
		fm := float64(m)
		m2 := 2 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1 + aa/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < 1e-16 {
			break
		}
	}
	return h
}

// invert cherche x dans [lo ; hi] tel que cdf(x) = p, cdf croissante. Une
// borne supérieure infinie est repoussée jusqu'à encadrer p.
func invert(cdf func(float64) float64, p, lo, hi float64) float64 {
	if math.IsInf(hi, 1) {
		hi = math.Max(1, lo+1)
		for cdf(hi) < p && hi < 1e300 {
			hi *= 2
		}
	}
	for range 2000 {
		mid := lo + (hi-lo)/2
		if mid <= lo || mid >= hi {
			break
		}
		if cdf(mid) < p {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + (hi-lo)/2
}

// Lois du khi-deux, de Student et de Fisher (degrés de liberté réels).
func chiCDF(x, k float64) float64 { return gammaP(k/2, x/2) }
func chiPDF(x, k float64) float64 {
	if x < 0 {
		return 0
	}
	if x == 0 {
		switch {
		case k < 2:
			return math.Inf(1)
		case k == 2:
			return 0.5
		}
		return 0
	}
	return math.Exp((k/2-1)*math.Log(x) - x/2 - k/2*math.Ln2 - lgamma(k/2))
}

// tTail : P(T > t) pour t ≥ 0.
func tTail(t, v float64) float64 { return 0.5 * betaI(v/(v+t*t), v/2, 0.5) }

func tCDF(t, v float64) float64 {
	if t >= 0 {
		return 1 - tTail(t, v)
	}
	return tTail(-t, v)
}

func tPDF(t, v float64) float64 {
	return math.Exp(lgamma((v+1)/2)-lgamma(v/2)-(v+1)/2*math.Log1p(t*t/v)) / math.Sqrt(v*math.Pi)
}

func tInv(p, v float64) float64 {
	if p == 0.5 {
		return 0
	}
	if p < 0.5 {
		return -tInv(1-p, v)
	}
	// Quantile de la queue : P(T > t) = 1 − p, décroissante en t.
	q := 1 - p
	return invert(func(t float64) float64 { return -tTail(t, v) }, -q, 0, math.Inf(1))
}

func fCDF(x, d1, d2 float64) float64 {
	if x <= 0 {
		return 0
	}
	return betaI(d1*x/(d1*x+d2), d1/2, d2/2)
}

// fTail : P(F > x), sans perte de précision dans la queue.
func fTail(x, d1, d2 float64) float64 {
	if x <= 0 {
		return 1
	}
	return betaI(d2/(d2+d1*x), d2/2, d1/2)
}

func fPDF(x, d1, d2 float64) float64 {
	if x < 0 {
		return 0
	}
	if x == 0 {
		switch {
		case d1 < 2:
			return math.Inf(1)
		case d1 == 2:
			return 1
		}
		return 0
	}
	l := d1/2*math.Log(d1) + d2/2*math.Log(d2) + (d1/2-1)*math.Log(x) - (d1+d2)/2*math.Log(d2+d1*x)
	return math.Exp(l - lgamma(d1/2) - lgamma(d2/2) + lgamma((d1+d2)/2))
}

// lnChoose : logarithme du coefficient binomial C(n, k).
func lnChoose(n, k float64) float64 { return lgamma(n+1) - lgamma(k+1) - lgamma(n-k+1) }

func binomPMF(k, n, p float64) float64 {
	switch {
	case p == 0:
		if k == 0 {
			return 1
		}
		return 0
	case p == 1:
		if k == n {
			return 1
		}
		return 0
	}
	return math.Exp(lnChoose(n, k) + k*math.Log(p) + (n-k)*math.Log1p(-p))
}

func binomCDF(k, n, p float64) float64 {
	if k >= n {
		return 1
	}
	t := 0.0
	for i := 0.0; i <= k; i++ {
		t += binomPMF(i, n, p)
	}
	return math.Min(t, 1)
}

// --- Arguments -------------------------------------------------------------------

// distArgs lit les n premiers arguments, numériques (absents : 0).
func distArgs(a []Value, n int) ([]float64, Value) {
	out := make([]float64, n)
	for i := range out {
		if i >= len(a) {
			continue
		}
		f, e := ToNumber(a[i])
		if e.IsErr() {
			return nil, e
		}
		out[i] = f
	}
	return out, Value{}
}

// cumulative lit l'argument « cumulative » (VRAI : fonction de répartition).
func cumulative(a []Value, i int) (bool, Value) {
	if i >= len(a) {
		return false, Err(ErrValue)
	}
	return Truth(a[i])
}

// numOrErr : #NOMBRE! si cond, sinon le nombre f.
func numOrErr(bad bool, f func() float64) Value {
	if bad {
		return Err(ErrNum)
	}
	return number(f())
}

// --- Loi normale ------------------------------------------------------------------

// fnNormDist : LOI.NORMALE.N(x; espérance; écart_type; cumulative).
func fnNormDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[2] <= 0, func() float64 {
		z := (x[0] - x[1]) / x[2]
		if cum {
			return normCDF(z)
		}
		return normPDF(z) / x[2]
	})
}

// fnNormInv : LOI.NORMALE.INVERSE.N(probabilité; espérance; écart_type).
func fnNormInv(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0 || x[0] >= 1 || x[2] <= 0, func() float64 { return x[1] + x[2]*normInv(x[0]) })
}

// fnNormSDist : LOI.NORMALE.STANDARD.N(z; cumulative).
func fnNormSDist(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 1)
	if e.IsErr() {
		return e
	}
	if cum {
		return number(normCDF(x[0]))
	}
	return number(normPDF(x[0]))
}

// fnNormSDistLegacy : LOI.NORMALE.STANDARD(z), toujours cumulative.
func fnNormSDistLegacy(a []Value) Value {
	return fnNormSDist([]Value{a[0], Boolean(true)})
}

func fnNormSInv(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0 || x[0] >= 1, func() float64 { return normInv(x[0]) })
}

// fnLogNormDist : LOI.LOGNORMALE.N(x; espérance; écart_type; cumulative).
func fnLogNormDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0 || x[2] <= 0, func() float64 {
		z := (math.Log(x[0]) - x[1]) / x[2]
		if cum {
			return normCDF(z)
		}
		return normPDF(z) / (x[2] * x[0])
	})
}

func fnLogNormInv(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0 || x[0] >= 1 || x[2] <= 0, func() float64 { return math.Exp(x[1] + x[2]*normInv(x[0])) })
}

// fnStandardize : CENTREE.REDUITE(x; espérance; écart_type).
func fnStandardize(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[2] <= 0, func() float64 { return (x[0] - x[1]) / x[2] })
}

// fnConfidence : INTERVALLE.CONFIANCE.NORMAL et .STUDENT(alpha; écart_type;
// taille), la demi-largeur de l'intervalle de confiance d'une moyenne.
func fnConfidence(student bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := distArgs(a, 3)
		if e.IsErr() {
			return e
		}
		alpha, sd, n := x[0], x[1], math.Trunc(x[2])
		if alpha <= 0 || alpha >= 1 || sd <= 0 || n < 1 {
			return Err(ErrNum)
		}
		if !student {
			return number(-normInv(alpha/2) * sd / math.Sqrt(n))
		}
		if n == 1 {
			return Err(ErrDiv0)
		}
		return number(tInv(1-alpha/2, n-1) * sd / math.Sqrt(n))
	}
}

// --- Student ------------------------------------------------------------------------

// fnTDist : LOI.STUDENT.N(x; degrés_liberté; cumulative).
func fnTDist(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 2)
	if e.IsErr() {
		return e
	}
	v := math.Trunc(x[1])
	return numOrErr(v < 1, func() float64 {
		if cum {
			return tCDF(x[0], v)
		}
		return tPDF(x[0], v)
	})
}

// fnTDistTail : LOI.STUDENT.BILATERALE (deux queues, x ≥ 0) et
// LOI.STUDENT.DROITE (queue droite).
func fnTDistTail(two bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := distArgs(a, 2)
		if e.IsErr() {
			return e
		}
		t, v := x[0], math.Trunc(x[1])
		if v < 1 || two && t < 0 {
			return Err(ErrNum)
		}
		if two {
			return number(math.Min(1, 2*tTail(t, v)))
		}
		return number(1 - tCDF(t, v))
	}
}

// fnTInv : LOI.STUDENT.INVERSE.N (queue gauche) et
// LOI.STUDENT.INVERSE.BILATERALE (deux queues).
func fnTInv(two bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := distArgs(a, 2)
		if e.IsErr() {
			return e
		}
		p, v := x[0], math.Trunc(x[1])
		if v < 1 || p <= 0 || p > 1 || !two && p == 1 {
			return Err(ErrNum)
		}
		if two {
			return number(tInv(1-p/2, v))
		}
		return number(tInv(p, v))
	}
}

// --- Khi-deux -----------------------------------------------------------------------

func chiDf(f float64) (float64, bool) {
	k := math.Trunc(f)
	return k, k >= 1 && k <= 1e10
}

// fnChiDist : LOI.KHIDEUX.N(x; degrés_liberté; cumulative).
func fnChiDist(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 2)
	if e.IsErr() {
		return e
	}
	k, ok := chiDf(x[1])
	return numOrErr(!ok || x[0] < 0, func() float64 {
		if cum {
			return chiCDF(x[0], k)
		}
		return chiPDF(x[0], k)
	})
}

// fnChiDistRT : LOI.KHIDEUX.DROITE(x; degrés_liberté).
func fnChiDistRT(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	k, ok := chiDf(x[1])
	return numOrErr(!ok || x[0] < 0, func() float64 { return gammaQ(k/2, x[0]/2) })
}

// fnChiInv : LOI.KHIDEUX.INVERSE (queue gauche) et .INVERSE.DROITE.
func fnChiInv(right bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := distArgs(a, 2)
		if e.IsErr() {
			return e
		}
		p := x[0]
		k, ok := chiDf(x[1])
		if !ok || p < 0 || p > 1 || !right && p == 1 || right && p == 0 {
			return Err(ErrNum)
		}
		if right {
			return number(invert(func(t float64) float64 { return -gammaQ(k/2, t/2) }, -p, 0, math.Inf(1)))
		}
		return number(invert(func(t float64) float64 { return chiCDF(t, k) }, p, 0, math.Inf(1)))
	}
}

// --- Fisher-Snedecor ------------------------------------------------------------------

func fDfs(a, b float64) (float64, float64, bool) {
	d1, d2 := math.Trunc(a), math.Trunc(b)
	return d1, d2, d1 >= 1 && d2 >= 1 && d1 < 1e10 && d2 < 1e10
}

// fnFDist : LOI.F.N(x; degrés_liberté1; degrés_liberté2; cumulative).
func fnFDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	d1, d2, ok := fDfs(x[1], x[2])
	return numOrErr(!ok || x[0] < 0, func() float64 {
		if cum {
			return fCDF(x[0], d1, d2)
		}
		return fPDF(x[0], d1, d2)
	})
}

func fnFDistRT(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	d1, d2, ok := fDfs(x[1], x[2])
	return numOrErr(!ok || x[0] < 0, func() float64 { return fTail(x[0], d1, d2) })
}

// fnFInv : INVERSE.LOI.F.N (queue gauche) et INVERSE.LOI.F.DROITE.
func fnFInv(right bool) func([]Value) Value {
	return func(a []Value) Value {
		x, e := distArgs(a, 3)
		if e.IsErr() {
			return e
		}
		p := x[0]
		d1, d2, ok := fDfs(x[1], x[2])
		if !ok || p < 0 || p > 1 || !right && p == 1 || right && p == 0 {
			return Err(ErrNum)
		}
		if right {
			return number(invert(func(t float64) float64 { return -fTail(t, d1, d2) }, -p, 0, math.Inf(1)))
		}
		return number(invert(func(t float64) float64 { return fCDF(t, d1, d2) }, p, 0, math.Inf(1)))
	}
}

// --- Lois discrètes -----------------------------------------------------------------

// fnBinomDist : LOI.BINOMIALE.N(succès; tirages; probabilité; cumulative).
func fnBinomDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	k, n, p := math.Trunc(x[0]), math.Trunc(x[1]), x[2]
	return numOrErr(k < 0 || k > n || p < 0 || p > 1, func() float64 {
		if cum {
			return binomCDF(k, n, p)
		}
		return binomPMF(k, n, p)
	})
}

// fnBinomInv : LOI.BINOMIALE.INVERSE(tirages; probabilité; alpha), le plus
// petit nombre de succès dont la probabilité cumulée atteint alpha.
func fnBinomInv(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	n, p, alpha := math.Trunc(x[0]), x[1], x[2]
	if n < 0 || p < 0 || p > 1 || alpha < 0 || alpha > 1 {
		return Err(ErrNum)
	}
	t := 0.0
	for k := 0.0; k < n; k++ {
		t += binomPMF(k, n, p)
		if t >= alpha*(1-1e-15) {
			return Num(k)
		}
	}
	return Num(n)
}

// fnNegBinomDist : LOI.BINOMIALE.NEG.N(échecs; succès; probabilité; cumulative).
func fnNegBinomDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	f, s, p := math.Trunc(x[0]), math.Trunc(x[1]), x[2]
	return numOrErr(f < 0 || s < 1 || p <= 0 || p > 1, func() float64 {
		if cum {
			return betaI(p, s, f+1)
		}
		if p == 1 {
			if f == 0 {
				return 1
			}
			return 0
		}
		return math.Exp(lnChoose(f+s-1, s-1) + s*math.Log(p) + f*math.Log1p(-p))
	})
}

// fnHypGeomDist : LOI.HYPERGEOMETRIQUE.N(succès_échantillon; taille_échantillon;
// succès_population; taille_population; cumulative).
func fnHypGeomDist(a []Value) Value {
	x, e := distArgs(a, 4)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 4)
	if e.IsErr() {
		return e
	}
	k, n, K, N := math.Trunc(x[0]), math.Trunc(x[1]), math.Trunc(x[2]), math.Trunc(x[3])
	if N <= 0 || n <= 0 || n > N || K <= 0 || K > N || k < math.Max(0, n-N+K) || k > math.Min(n, K) {
		return Err(ErrNum)
	}
	pmf := func(k float64) float64 { return math.Exp(lnChoose(K, k) + lnChoose(N-K, n-k) - lnChoose(N, n)) }
	if !cum {
		return number(pmf(k))
	}
	t := 0.0
	for i := math.Max(0, n-N+K); i <= k; i++ {
		t += pmf(i)
	}
	return number(math.Min(t, 1))
}

// fnPoissonDist : LOI.POISSON.N(x; espérance; cumulative).
func fnPoissonDist(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 2)
	if e.IsErr() {
		return e
	}
	k, m := math.Trunc(x[0]), x[1]
	return numOrErr(k < 0 || m < 0, func() float64 {
		if m == 0 {
			return 1 // toute la masse en 0, et k ≥ 0
		}
		if cum {
			return gammaQ(k+1, m)
		}
		return math.Exp(k*math.Log(m) - m - lgamma(k+1))
	})
}

// fnExponDist : LOI.EXPONENTIELLE.N(x; lambda; cumulative).
func fnExponDist(a []Value) Value {
	x, e := distArgs(a, 2)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 2)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] < 0 || x[1] <= 0, func() float64 {
		if cum {
			return -math.Expm1(-x[1] * x[0])
		}
		return x[1] * math.Exp(-x[1]*x[0])
	})
}

// --- Gamma, bêta, Weibull -------------------------------------------------------------

// fnGammaDist : LOI.GAMMA.N(x; alpha; bêta; cumulative).
func fnGammaDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	v, al, b := x[0], x[1], x[2]
	return numOrErr(v < 0 || al <= 0 || b <= 0, func() float64 {
		if cum {
			return gammaP(al, v/b)
		}
		if v == 0 {
			switch {
			case al < 1:
				return math.Inf(1)
			case al == 1:
				return 1 / b
			}
			return 0
		}
		return math.Exp((al-1)*math.Log(v) - v/b - al*math.Log(b) - lgamma(al))
	})
}

// fnGammaInv : LOI.GAMMA.INVERSE.N(probabilité; alpha; bêta).
func fnGammaInv(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	p, al, b := x[0], x[1], x[2]
	if p < 0 || p >= 1 || al <= 0 || b <= 0 {
		return Err(ErrNum)
	}
	if p == 0 {
		return Num(0)
	}
	return number(b * invert(func(t float64) float64 { return gammaP(al, t) }, p, 0, math.Inf(1)))
}

// fnGammaLn : LNGAMMA(x), x > 0.
func fnGammaLn(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0, func() float64 { return lgamma(x[0]) })
}

// fnGamma : GAMMA(x) ; 0 et les entiers négatifs donnent #NOMBRE!.
func fnGamma(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= 0 && x[0] == math.Trunc(x[0]), func() float64 { return math.Gamma(x[0]) })
}

// betaBounds lit les bornes A et B facultatives (0 et 1 par défaut).
func betaBounds(a []Value, i int) (float64, float64, Value) {
	lo, hi := 0.0, 1.0
	if i < len(a) && !a[i].IsEmpty() {
		f, e := ToNumber(a[i])
		if e.IsErr() {
			return 0, 0, e
		}
		lo = f
	}
	if i+1 < len(a) && !a[i+1].IsEmpty() {
		f, e := ToNumber(a[i+1])
		if e.IsErr() {
			return 0, 0, e
		}
		hi = f
	}
	return lo, hi, Value{}
}

// fnBetaDist : LOI.BETA.N(x; alpha; bêta; cumulative; [A]; [B]).
func fnBetaDist(a []Value) Value {
	x, e := distArgs(a[:min(3, len(a))], 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	lo, hi, e := betaBounds(a, 4)
	if e.IsErr() {
		return e
	}
	v, al, b := x[0], x[1], x[2]
	if al <= 0 || b <= 0 || lo >= hi || v < lo || v > hi {
		return Err(ErrNum)
	}
	y := (v - lo) / (hi - lo)
	if cum {
		return number(betaI(y, al, b))
	}
	if y == 0 || y == 1 {
		return Err(ErrNum)
	}
	return number(math.Exp((al-1)*math.Log(y)+(b-1)*math.Log1p(-y)+lgamma(al+b)-lgamma(al)-lgamma(b)) / (hi - lo))
}

// fnBetaInv : BETA.INVERSE.N(probabilité; alpha; bêta; [A]; [B]).
func fnBetaInv(a []Value) Value {
	x, e := distArgs(a[:min(3, len(a))], 3)
	if e.IsErr() {
		return e
	}
	lo, hi, e := betaBounds(a, 3)
	if e.IsErr() {
		return e
	}
	p, al, b := x[0], x[1], x[2]
	if p <= 0 || p > 1 || al <= 0 || b <= 0 || lo >= hi {
		return Err(ErrNum)
	}
	y := 1.0
	if p < 1 {
		y = invert(func(t float64) float64 { return betaI(t, al, b) }, p, 0, 1)
	}
	return number(lo + y*(hi-lo))
}

// fnWeibullDist : LOI.WEIBULL.N(x; alpha; bêta; cumulative).
func fnWeibullDist(a []Value) Value {
	x, e := distArgs(a, 3)
	if e.IsErr() {
		return e
	}
	cum, e := cumulative(a, 3)
	if e.IsErr() {
		return e
	}
	v, al, b := x[0], x[1], x[2]
	return numOrErr(v < 0 || al <= 0 || b <= 0, func() float64 {
		if cum {
			return -math.Expm1(-math.Pow(v/b, al))
		}
		return al / math.Pow(b, al) * math.Pow(v, al-1) * math.Exp(-math.Pow(v/b, al))
	})
}

// --- Divers -------------------------------------------------------------------------

func fnPhi(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return number(normPDF(x[0]))
}

func fnGauss(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return number(normCDF(x[0]) - 0.5)
}

func fnFisher(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return numOrErr(x[0] <= -1 || x[0] >= 1, func() float64 { return math.Atanh(x[0]) })
}

func fnFisherInv(a []Value) Value {
	x, e := distArgs(a, 1)
	if e.IsErr() {
		return e
	}
	return number(math.Tanh(x[0]))
}
