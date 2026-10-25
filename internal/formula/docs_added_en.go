package formula

// Traductions anglaises de docs_added.go.
func docsAddedEN() map[string]docText {
	x := ta("x", "Value to evaluate.")
	xPos := ta("x", "Value to evaluate, positive.")
	cum := ta("cumulative", "{=TRUE}: cumulative probability (distribution function); {=FALSE}: density.")
	mean := ta("mean", "Mean of the distribution.")
	sd := ta("standard_dev", "Standard deviation of the distribution (positive).")
	prob := ta("probability", "Probability, between 0 and 1.")
	df := ta("deg_freedom", "Number of degrees of freedom (truncated).")
	df1 := ta("deg_freedom1", "Degrees of freedom of the numerator.")
	df2 := ta("deg_freedom2", "Degrees of freedom of the denominator.")
	alpha := ta("alpha", "Shape parameter.")
	beta := ta("beta", "Scale parameter.")
	knownY := ta("known_ys", "Column of values to explain (Table[Column]) or range.")
	knownX := ta("known_xs", "Column of explanatory values, from the same table, or range of the same shape.")
	konst := ta("const", "{=TRUE} or omitted: line with an intercept; {=FALSE}: through 0.")
	agg := ta("number1", "Whole column (Table[Column]), range (Cell[A1:B3]), dynamic array or constant, expression on the columns of a table, cell or value.")
	s1 := ta("array1", "First column (Table[Column]) or range.")
	s2 := ta("array2", "Second column, from the same table, or range of the same shape.")
	settle := ta("settlement", "Purchase date of the security.")
	mat := ta("maturity", "Maturity date of the security.")
	freq := ta("frequency", "Coupons per year: 1, 2 or 4.")
	basis := ta("basis", "Day count: 0 or omitted 30/360, 1 actual/actual, 2 actual/360, 3 actual/365, 4 European 30/360.")
	cost := ta("cost", "Purchase cost of the asset.")
	salvage := ta("salvage", "Value of the asset at the end of the depreciation.")
	life := ta("life", "Depreciation life, in periods.")
	period := ta("period", "Period number.")
	db := ta("database", "Whole table, by its name (Sales).")
	field := ta("field", "Column name in quotes (\"Amount\") or its number.")
	crit := ta("criteria", "Cell range (Cell[E1:F3]): column headers, then one condition per row.")
	amorDate := ta("date_purchased", "Purchase date of the asset.")
	amorFirst := ta("first_period", "End date of the first period.")
	amorPer := ta("period", "Period number (0: the first one, prorated).")
	amorRate := ta("rate", "Depreciation rate.")
	amorBasis := ta("basis", "Day count: 0 or omitted 30/360, 1 actual/actual, 3 actual/365, 4 European 30/360.")
	coupon := ta("rate", "Annual coupon rate.")
	redemption := ta("redemption", "Redemption value per 100 of face value.")
	dbText := func(desc, ex string) docText { return tdx(desc, ex, db, field, crit) }
	return map[string]docText{
		"NORM.DIST":       tdx("Returns the normal distribution for a mean and a standard deviation.", `=NORM.DIST(42, 40, 1.5, TRUE)`, x, mean, sd, cum),
		"NORM.INV":        tdx("Returns the value of the normal distribution with the given cumulative probability.", `=NORM.INV(0.95, 40, 1.5)`, prob, mean, sd),
		"NORM.S.DIST":     tdx("Returns the standard normal distribution (mean 0, standard deviation 1).", `=NORM.S.DIST(1.96, TRUE)`, ta("z", "Value to evaluate."), cum),
		"NORM.S.INV":      tdx("Returns the value of the standard normal distribution with the given cumulative probability.", `=NORM.S.INV(0.975)`, prob),
		"NORMSDIST":       tdx("Returns the cumulative standard normal distribution (older form).", `=NORMSDIST(1.96)`, ta("z", "Value to evaluate.")),
		"LOGNORM.DIST":    tdx("Returns the lognormal distribution (ln(x) is normally distributed).", `=LOGNORM.DIST(4, 3.5, 1.2, TRUE)`, xPos, ta("mean", "Mean of ln(x)."), ta("standard_dev", "Standard deviation of ln(x)."), cum),
		"LOGNORM.INV":     tdx("Returns the value of the lognormal distribution with the given cumulative probability.", `=LOGNORM.INV(0.04, 3.5, 1.2)`, prob, ta("mean", "Mean of ln(x)."), ta("standard_dev", "Standard deviation of ln(x).")),
		"T.DIST":          tdx("Returns the Student's t-distribution (left tail).", `=T.DIST(1.5, 10, TRUE)`, x, df, cum),
		"T.DIST.2T":       tdx("Returns the two-tailed probability of the Student's t-distribution.", `=T.DIST.2T(1.96, 60)`, xPos, df),
		"T.DIST.RT":       tdx("Returns the right-tail probability of the Student's t-distribution.", `=T.DIST.RT(1.96, 60)`, x, df),
		"T.INV":           tdx("Returns the value of the Student's t-distribution with the given left-tail probability.", `=T.INV(0.975, 10)`, prob, df),
		"T.INV.2T":        tdx("Returns the value of the Student's t-distribution with the given two-tailed probability.", `=T.INV.2T(0.05, 10)`, prob, df),
		"CHISQ.DIST":      tdx("Returns the chi-squared distribution (left tail).", `=CHISQ.DIST(3, 2, TRUE)`, xPos, df, cum),
		"CHISQ.DIST.RT":   tdx("Returns the right-tail probability of the chi-squared distribution.", `=CHISQ.DIST.RT(18.3, 10)`, xPos, df),
		"CHISQ.INV":       tdx("Returns the value of the chi-squared distribution with the given left-tail probability.", `=CHISQ.INV(0.95, 10)`, prob, df),
		"CHISQ.INV.RT":    tdx("Returns the value of the chi-squared distribution with the given right-tail probability.", `=CHISQ.INV.RT(0.05, 10)`, prob, df),
		"F.DIST":          tdx("Returns the F distribution (left tail).", `=F.DIST(15, 6, 4, TRUE)`, xPos, df1, df2, cum),
		"F.DIST.RT":       tdx("Returns the right-tail probability of the F distribution.", `=F.DIST.RT(15, 6, 4)`, xPos, df1, df2),
		"F.INV":           tdx("Returns the value of the F distribution with the given left-tail probability.", `=F.INV(0.95, 6, 4)`, prob, df1, df2),
		"F.INV.RT":        tdx("Returns the value of the F distribution with the given right-tail probability.", `=F.INV.RT(0.05, 6, 4)`, prob, df1, df2),
		"BINOM.DIST":      tdx("Returns the probability of a number of successes in a number of trials (binomial distribution).", `=BINOM.DIST(6, 10, 0.5, FALSE)`, ta("number_s", "Number of successes."), ta("trials", "Number of independent trials."), ta("probability_s", "Probability of success on each trial."), cum),
		"BINOM.INV":       tdx("Returns the smallest number of successes whose cumulative probability reaches a threshold.", `=BINOM.INV(6, 0.5, 0.75)`, ta("trials", "Number of trials."), ta("probability_s", "Probability of a success."), ta("alpha", "Cumulative probability threshold.")),
		"NEGBINOM.DIST":   tdx("Returns the probability of a number of failures before a number of successes (negative binomial distribution).", `=NEGBINOM.DIST(10, 5, 0.25, TRUE)`, ta("number_f", "Number of failures."), ta("number_s", "Target number of successes."), ta("probability_s", "Probability of a success."), cum),
		"HYPGEOM.DIST":    tdx("Returns the probability of a number of successes in a sample drawn without replacement (hypergeometric distribution).", `=HYPGEOM.DIST(1, 4, 8, 20, TRUE)`, ta("sample_s", "Number of successes in the sample."), ta("number_sample", "Size of the sample."), ta("population_s", "Number of successes in the population."), ta("number_pop", "Size of the population."), cum),
		"POISSON.DIST":    tdx("Returns the Poisson distribution: probability of a number of events.", `=POISSON.DIST(2, 5, TRUE)`, ta("x", "Number of events."), ta("mean", "Expected number of events."), cum),
		"EXPON.DIST":      tdx("Returns the exponential distribution (time between two events).", `=EXPON.DIST(0.2, 10, TRUE)`, xPos, ta("lambda", "Parameter of the distribution (positive)."), cum),
		"GAMMA.DIST":      tdx("Returns the gamma distribution.", `=GAMMA.DIST(10, 9, 2, TRUE)`, xPos, alpha, beta, cum),
		"GAMMA.INV":       tdx("Returns the value of the gamma distribution with the given cumulative probability.", `=GAMMA.INV(0.07, 9, 2)`, prob, alpha, beta),
		"GAMMALN":         tdx("Returns the natural logarithm of the gamma function.", `=GAMMALN(4)`, ta("x", "Positive number.")),
		"GAMMA":           tdx("Returns the value of the gamma function.", `=GAMMA(2.5)`, ta("number", "Number (neither 0 nor a negative integer).")),
		"BETA.DIST":       tdx("Returns the beta distribution, between 0 and 1 or between two bounds.", `=BETA.DIST(2, 8, 10, TRUE, 1, 3)`, ta("x", "Value to evaluate, between A and B."), ta("alpha", "First shape parameter."), ta("beta", "Second shape parameter."), cum, ta("A", "Lower bound (0 if omitted)."), ta("B", "Upper bound (1 if omitted).")),
		"BETA.INV":        tdx("Returns the value of the beta distribution with the given cumulative probability.", `=BETA.INV(0.69, 8, 10, 1, 3)`, prob, ta("alpha", "First shape parameter."), ta("beta", "Second shape parameter."), ta("A", "Lower bound (0 if omitted)."), ta("B", "Upper bound (1 if omitted).")),
		"WEIBULL.DIST":    tdx("Returns the Weibull distribution (lifetimes, failures).", `=WEIBULL.DIST(105, 20, 100, TRUE)`, xPos, alpha, beta, cum),
		"CONFIDENCE.NORM": tdx("Returns the half-width of the confidence interval of a mean (normal distribution).", `=CONFIDENCE.NORM(0.05, 2.5, 50)`, ta("alpha", "Risk: 0.05 for a 95% interval."), ta("standard_dev", "Standard deviation of the population."), ta("size", "Sample size.")),
		"CONFIDENCE.T":    tdx("Returns the half-width of the confidence interval of a mean (Student's t-distribution).", `=CONFIDENCE.T(0.05, 1, 50)`, ta("alpha", "Risk: 0.05 for a 95% interval."), ta("standard_dev", "Standard deviation of the sample."), ta("size", "Sample size.")),
		"STANDARDIZE":     tdx("Returns the standardized value: (x − mean) / standard deviation.", `=STANDARDIZE([Grade], 12, 3)`, ta("x", "Value to standardize."), mean, sd),
		"PHI":             tdx("Returns the density of the standard normal distribution.", `=PHI(0.75)`, x),
		"GAUSS":           tdx("Returns the probability that a standard normal value falls between 0 and z.", `=GAUSS(2)`, ta("z", "Value to evaluate.")),
		"FISHER":          tdx("Returns the Fisher transformation of a correlation coefficient.", `=FISHER(0.75)`, ta("x", "Value strictly between −1 and 1.")),
		"FISHERINV":       tdx("Returns the inverse of the Fisher transformation.", `=FISHERINV(0.97)`, ta("y", "Transformed value.")),

		"KURT":            tdx("Returns the kurtosis of a series: at least 4 values.", `=KURT(Sales[Amount])`, agg),
		"SKEW":            tdx("Returns the skewness of a series (sample): at least 3 values.", `=SKEW(Sales[Amount])`, agg),
		"SKEW.P":          tdx("Returns the skewness of a population: at least 3 values.", `=SKEW.P(Sales[Amount])`, agg),
		"AVEDEV":          tdx("Returns the average of the absolute deviations from the mean.", `=AVEDEV(Sales[Amount])`, agg),
		"DEVSQ":           tdx("Returns the sum of the squared deviations from the mean.", `=DEVSQ(Sales[Amount])`, agg),
		"GEOMEAN":         tdx("Returns the geometric mean of positive numbers.", `=GEOMEAN(Rates[Factor])`, agg),
		"HARMEAN":         tdx("Returns the harmonic mean of positive numbers.", `=HARMEAN(Trips[Speed])`, agg),
		"COVARIANCE.S":    tdx("Returns the sample covariance of two columns.", `=COVARIANCE.S(Sales[Price], Sales[Qty])`, s1, s2),
		"COVARIANCE.P":    tdx("Returns the population covariance of two columns.", `=COVARIANCE.P(Sales[Price], Sales[Qty])`, s1, s2),
		"RSQ":             tdx("Returns the coefficient of determination R² of the linear regression.", `=RSQ(Sales[Amount], Sales[Qty])`, knownY, knownX),
		"STEYX":           tdx("Returns the standard error of the values predicted by the linear regression.", `=STEYX(Sales[Amount], Sales[Qty])`, knownY, knownX),
		"FORECAST.LINEAR": tdx("Predicts a value by linear regression.", `=FORECAST.LINEAR(30, Sales[Amount], Sales[Qty])`, ta("x", "Value to predict for."), knownY, knownX),
		"TREND":           tdx("Returns the value of the regression line for a new x.", `=TREND(Sales[Amount], Sales[Qty], [Qty])`, knownY, knownX, ta("new_x", "Value to predict for, often a column of the row ([Qty])."), konst),
		"GROWTH":          tdx("Returns the value of the fitted exponential curve for a new x.", `=GROWTH(Sales[Amount], Sales[Qty], 10)`, knownY, knownX, ta("new_x", "Value to predict for."), ta("const", "{=TRUE} or omitted: normal calculation; {=FALSE}: curve equal to 1 at 0.")),
		"LINEST":          tdx("Returns the slope and intercept of the regression line, and its statistics; the result spills.", `=LINEST(Sales[Amount], Sales[Qty])`, knownY, knownX, konst, ta("stats", "{=TRUE}: 5 rows of statistics; {=FALSE} or omitted: one row (written in the formula).")),
		"LOGEST":          tdx("Returns the coefficients of the fitted exponential curve, and its statistics; the result spills.", `=LOGEST(Sales[Amount], Sales[Qty])`, knownY, knownX, ta("const", "{=TRUE} or omitted: normal calculation; {=FALSE}: coefficient b equal to 1."), ta("stats", "{=TRUE}: 5 rows of statistics; {=FALSE} or omitted: one row (written in the formula).")),
		"T.TEST":          tdx("Returns the probability of Student's t-test on two series.", `=T.TEST(Tests[Before], Tests[After], 2, 1)`, s1, s2, ta("tails", "1: one-tailed test; 2: two-tailed."), ta("type", "1: paired series; 2: equal variances; 3: unequal variances.")),
		"F.TEST":          tdx("Returns the two-tailed probability that two series have the same variance.", `=F.TEST(Tests[Before], Tests[After])`, s1, s2),
		"Z.TEST":          tdx("Returns the one-tailed probability of the z-test (mean greater than x).", `=Z.TEST(Tests[Measure], 4)`, ta("array", "Column of values (Table[Column]) or range."), ta("x", "Value to test."), ta("sigma", "Standard deviation of the population (that of the sample if omitted).")),
		"CHISQ.TEST":      tdx("Returns the probability of the chi-squared test of independence.", `=CHISQ.TEST(Cell[B2:C4], Cell[E2:F4])`, ta("actual_range", "Observed values: column or range."), ta("expected_range", "Expected values, of the same shape.")),
		"RANK.AVG":        tdx("Returns the rank of a value in a column; ties get their average rank.", `=RANK.AVG([Score], Players[Score])`, ta("number", "Value to rank."), ta("ref", "Column in which to rank the value (Table[Column])."), ta("order", "0 or omitted: descending; 1: ascending.")),
		"AGGREGATE": tdx("Calculates an aggregate, optionally ignoring errors or the rows hidden by the filter.", `=AGGREGATE(9, 6, Sales[Amount])`,
			ta("function_num", "1 {fn:AVERAGE}, 2 {fn:COUNT}, 3 {fn:COUNTA}, 4 {fn:MAX}, 5 {fn:MIN}, 6 {fn:PRODUCT}, 7 {fn:STDEV.S}, 8 {fn:STDEV.P}, 9 {fn:SUM}, 10 {fn:VAR.S}, 11 {fn:VAR.P}, 12 {fn:MEDIAN}, 13 {fn:MODE.SNGL}, 14 {fn:LARGE}, 15 {fn:SMALL}, 16 {fn:PERCENTILE.INC}, 17 {fn:QUARTILE.INC}, 18 {fn:PERCENTILE.EXC}, 19 {fn:QUARTILE.EXC}."),
			ta("options", "0 to 7: 6 ignores errors, 5 hidden rows, 7 both (0: nothing)."),
			ta("ref1", "Column (Table[Column]), range or expression; for 14 to 19, followed by k.")),

		"XNPV":       tdx("Returns the net present value of dated cash flows.", `=XNPV(8%, Flows[Amount], Flows[Date])`, ta("rate", "Annual discount rate."), ta("values", "Column of cash flows (Table[Column])."), ta("dates", "Column of the dates of the flows, from the same table; the first one is the origin.")),
		"XIRR":       tdx("Returns the internal rate of return of dated cash flows.", `=XIRR(Flows[Amount], Flows[Date])`, ta("values", "Column of cash flows, with at least one positive and one negative flow."), ta("dates", "Column of the dates of the flows, from the same table."), ta("guess", "Starting rate for the calculation (10% if omitted).")),
		"EFFECT":     tdx("Returns the effective annual rate of a nominal rate compounded several times a year.", `=EFFECT(5%, 12)`, ta("nominal_rate", "Nominal annual rate."), ta("npery", "Number of compounding periods per year.")),
		"NOMINAL":    tdx("Returns the nominal annual rate of an effective rate.", `=NOMINAL(5.12%, 12)`, ta("effect_rate", "Effective annual rate."), ta("npery", "Number of compounding periods per year.")),
		"SLN":        tdx("Returns the straight-line depreciation of an asset for one period.", `=SLN(30000, 7500, 10)`, cost, salvage, life),
		"SYD":        tdx("Returns the sum-of-years' digits depreciation of an asset for one period.", `=SYD(30000, 7500, 10, 1)`, cost, salvage, life, ta("per", "Period number, from 1 to life.")),
		"DB":         tdx("Returns the fixed-declining balance depreciation of an asset for one period.", `=DB(1000000, 100000, 6, 1, 7)`, cost, salvage, life, period, ta("month", "Months in the first year (12 if omitted).")),
		"DDB":        tdx("Returns the double-declining balance (or other factor) depreciation of an asset for one period.", `=DDB(2400, 300, 10, 1)`, cost, salvage, life, period, ta("factor", "Rate at which the balance declines (2 if omitted).")),
		"VDB":        tdx("Returns the declining balance depreciation of an asset between two periods, switching to straight-line.", `=VDB(2400, 300, 10, 0, 1)`, cost, salvage, life, ta("start_period", "Start of the period (0: purchase)."), ta("end_period", "End of the period."), ta("factor", "Rate at which the balance declines (2 if omitted)."), ta("no_switch", "{=TRUE}: never switches to straight-line.")),
		"AMORLINC":   tdx("Returns the straight-line depreciation of a period under the French accounting system (first period prorated).", `=AMORLINC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"AMORDEGRC":  tdx("Returns the declining depreciation of a period under the French accounting system.", `=AMORDEGRC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"PRICE":      tdx("Returns the price of a coupon security per 100 of face value.", `=PRICE(DATE(2026,2,15), DATE(2035,11,15), 5.75%, 6.5%, 100, 2, 0)`, settle, mat, coupon, ta("yld", "Annual yield of the security."), redemption, freq, basis),
		"YIELD":      tdx("Returns the yield of a coupon security from its price.", `=YIELD(DATE(2026,2,15), DATE(2034,11,15), 5.75%, 95.04, 100, 2, 0)`, settle, mat, coupon, ta("pr", "Price per 100 of face value."), redemption, freq, basis),
		"ACCRINT":    tdx("Returns the accrued interest of a coupon security.", `=ACCRINT(DATE(2026,3,1), DATE(2026,8,31), DATE(2026,5,1), 10%, 1000, 2, 0)`, ta("issue", "Issue date of the security."), ta("first_interest", "Date of the first coupon."), settle, coupon, ta("par", "Face value of the security."), freq, basis, ta("calc_method", "{=TRUE} or omitted: from the issue; {=FALSE}: from the first coupon.")),
		"COUPNUM":    tdx("Returns the number of coupons still to be paid between settlement and maturity.", `=COUPNUM(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPPCD":    tdx("Returns the date of the coupon before settlement.", `=COUPPCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPNCD":    tdx("Returns the date of the coupon after settlement.", `=COUPNCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYS":   tdx("Returns the number of days in the coupon period that contains settlement.", `=COUPDAYS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYBS":  tdx("Returns the number of days from the previous coupon to settlement.", `=COUPDAYBS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYSNC": tdx("Returns the number of days from settlement to the next coupon.", `=COUPDAYSNC(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),

		"DSUM":     dbText("Adds the values of a column for the rows that meet the criteria.", `=DSUM(Sales, "Amount", Cell[E1:F3])`),
		"DAVERAGE": dbText("Returns the average of a column for the rows that meet the criteria.", `=DAVERAGE(Sales, "Amount", Cell[E1:F3])`),
		"DCOUNT":   dbText("Counts the numbers of a column for the rows that meet the criteria.", `=DCOUNT(Sales, "Amount", Cell[E1:F3])`),
		"DCOUNTA":  dbText("Counts the non-blank values of a column for the rows that meet the criteria.", `=DCOUNTA(Sales, "Customer", Cell[E1:F3])`),
		"DMAX":     dbText("Returns the largest value of a column for the rows that meet the criteria.", `=DMAX(Sales, "Amount", Cell[E1:F3])`),
		"DMIN":     dbText("Returns the smallest value of a column for the rows that meet the criteria.", `=DMIN(Sales, "Amount", Cell[E1:F3])`),
		"DPRODUCT": dbText("Multiplies the values of a column for the rows that meet the criteria.", `=DPRODUCT(Rates, "Factor", Cell[E1:E2])`),
		"DSTDEV":   dbText("Returns the sample standard deviation of a column for the rows that meet the criteria.", `=DSTDEV(Sales, "Amount", Cell[E1:F3])`),
		"DSTDEVP":  dbText("Returns the population standard deviation of a column for the rows that meet the criteria.", `=DSTDEVP(Sales, "Amount", Cell[E1:F3])`),
		"DVAR":     dbText("Returns the sample variance of a column for the rows that meet the criteria.", `=DVAR(Sales, "Amount", Cell[E1:F3])`),
		"DVARP":    dbText("Returns the population variance of a column for the rows that meet the criteria.", `=DVARP(Sales, "Amount", Cell[E1:F3])`),
		"DGET":     dbText("Returns the value of a column for the only row that meets the criteria.", `=DGET(Customers, "City", Cell[E1:E2])`),

		"OFFSET": tdx("Returns a reference shifted by a number of rows and columns, of the size you want.", `=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1], 1))`,
			ta("reference", "Starting cell (Cell[A1]), range, column (Table[Column]) or row (Table[Column][2])."),
			ta("rows", "Shift down (negative: up)."), ta("cols", "Shift to the right (negative: to the left)."),
			ta("height", "Number of rows of the result (that of the reference if omitted)."),
			ta("width", "Number of columns of the result (that of the reference if omitted).")),
		"INDIRECT": tdx("Returns the reference written in a text: \"B3\", \"Sheet2!B3\", \"Cell[A1:A9]\", \"Sales[Price]\".", `=INDIRECT("Sheet2!B"&Cell[A1])`,
			ta("ref_text", "Text of the reference; an address such as B3 refers to a free cell."),
			ta("a1", "{=TRUE} or omitted; {=FALSE} (R1C1 style) is not supported.")),
		"CELL": tdx("Returns information about a cell: \"address\", \"row\", \"col\", \"contents\" or \"type\" (b blank, l text, v value).", `=CELL("address", Cell[B3])`,
			ta("info_type", "Information wanted, in quotes."), ta("reference", "Cell, range (its first cell) or table reference.")),
		"TRANSPOSE": tdx("Swaps the rows and columns of a cell range or a constant; the result spills.", `=TRANSPOSE(Cell[A1:C2])`,
			ta("array", "Cell range (Cell[A1:C2]) or constant ({1;2;3}).")),
	}
}
