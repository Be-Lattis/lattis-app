package formula

// Traductions chinoises de docs_added.go.
func docsAddedZH() map[string]docText {
	x := ta("x", "要计算的值。")
	xPos := ta("x", "要计算的值，须为正数。")
	cum := ta("累积", "{=TRUE}：累积概率（分布函数）；{=FALSE}：概率密度。")
	mean := ta("平均值", "分布的平均值。")
	sd := ta("标准差", "分布的标准差（正数）。")
	prob := ta("概率", "概率，介于 0 与 1 之间。")
	df := ta("自由度", "自由度（截尾取整）。")
	df1 := ta("自由度1", "分子的自由度。")
	df2 := ta("自由度2", "分母的自由度。")
	alpha := ta("alpha", "形状参数。")
	beta := ta("beta", "尺度参数。")
	knownY := ta("已知y值", "要解释的值所在的列（表[列]）或区域。")
	knownX := ta("已知x值", "同一张表中的解释变量列，或形状相同的区域。")
	konst := ta("常量", "{=TRUE} 或省略：带截距的直线；{=FALSE}：过原点。")
	agg := ta("数值1", "整列（表[列]）、区域（Cell[A1:B3]）、动态数组或常量、基于表中各列的表达式、单元格或值。")
	s1 := ta("数组1", "第一列（表[列]）或区域。")
	s2 := ta("数组2", "同一张表中的第二列，或形状相同的区域。")
	settle := ta("结算日", "证券的购买日期。")
	mat := ta("到期日", "证券的到期日期。")
	freq := ta("频率", "每年付息次数：1、2 或 4。")
	basis := ta("基准", "日计数基准：0 或省略 30/360，1 实际/实际，2 实际/360，3 实际/365，4 欧洲 30/360。")
	cost := ta("成本", "资产的购置成本。")
	salvage := ta("残值", "折旧结束时资产的价值。")
	life := ta("年限", "折旧期数。")
	period := ta("期间", "期次。")
	db := ta("数据库", "整张表，用表名表示（销售）。")
	field := ta("字段", "带引号的列名（\"金额\"）或列号。")
	crit := ta("条件", "单元格区域（Cell[E1:F3]）：第一行为列标题，之后每行一个条件。")
	amorDate := ta("购买日期", "资产的购买日期。")
	amorFirst := ta("首期结束日", "第一期的结束日期。")
	amorPer := ta("期间", "期次（0：第一期，按比例）。")
	amorRate := ta("折旧率", "折旧率。")
	amorBasis := ta("基准", "日计数基准：0 或省略 30/360，1 实际/实际，3 实际/365，4 欧洲 30/360。")
	coupon := ta("利率", "年票面利率。")
	redemption := ta("赎回价", "每 100 面值的赎回价值。")
	dbText := func(desc, ex string) docText { return tdx(desc, ex, db, field, crit) }
	return map[string]docText{
		"NORM.DIST":       tdx("返回给定平均值和标准差的正态分布。", `=NORM.DIST(42, 40, 1.5, TRUE)`, x, mean, sd, cum),
		"NORM.INV":        tdx("返回累积概率为给定值时正态分布的值。", `=NORM.INV(0.95, 40, 1.5)`, prob, mean, sd),
		"NORM.S.DIST":     tdx("返回标准正态分布（平均值 0，标准差 1）。", `=NORM.S.DIST(1.96, TRUE)`, ta("z", "要计算的值。"), cum),
		"NORM.S.INV":      tdx("返回累积概率为给定值时标准正态分布的值。", `=NORM.S.INV(0.975)`, prob),
		"NORMSDIST":       tdx("返回标准正态累积分布（旧形式）。", `=NORMSDIST(1.96)`, ta("z", "要计算的值。")),
		"LOGNORM.DIST":    tdx("返回对数正态分布（ln(x) 服从正态分布）。", `=LOGNORM.DIST(4, 3.5, 1.2, TRUE)`, xPos, ta("平均值", "ln(x) 的平均值。"), ta("标准差", "ln(x) 的标准差。"), cum),
		"LOGNORM.INV":     tdx("返回累积概率为给定值时对数正态分布的值。", `=LOGNORM.INV(0.04, 3.5, 1.2)`, prob, ta("平均值", "ln(x) 的平均值。"), ta("标准差", "ln(x) 的标准差。")),
		"T.DIST":          tdx("返回学生 t 分布（左尾）。", `=T.DIST(1.5, 10, TRUE)`, x, df, cum),
		"T.DIST.2T":       tdx("返回学生 t 分布的双尾概率。", `=T.DIST.2T(1.96, 60)`, xPos, df),
		"T.DIST.RT":       tdx("返回学生 t 分布的右尾概率。", `=T.DIST.RT(1.96, 60)`, x, df),
		"T.INV":           tdx("返回左尾概率为给定值时 t 分布的值。", `=T.INV(0.975, 10)`, prob, df),
		"T.INV.2T":        tdx("返回双尾概率为给定值时 t 分布的值。", `=T.INV.2T(0.05, 10)`, prob, df),
		"CHISQ.DIST":      tdx("返回卡方分布（左尾）。", `=CHISQ.DIST(3, 2, TRUE)`, xPos, df, cum),
		"CHISQ.DIST.RT":   tdx("返回卡方分布的右尾概率。", `=CHISQ.DIST.RT(18.3, 10)`, xPos, df),
		"CHISQ.INV":       tdx("返回左尾概率为给定值时卡方分布的值。", `=CHISQ.INV(0.95, 10)`, prob, df),
		"CHISQ.INV.RT":    tdx("返回右尾概率为给定值时卡方分布的值。", `=CHISQ.INV.RT(0.05, 10)`, prob, df),
		"F.DIST":          tdx("返回 F 分布（左尾）。", `=F.DIST(15, 6, 4, TRUE)`, xPos, df1, df2, cum),
		"F.DIST.RT":       tdx("返回 F 分布的右尾概率。", `=F.DIST.RT(15, 6, 4)`, xPos, df1, df2),
		"F.INV":           tdx("返回左尾概率为给定值时 F 分布的值。", `=F.INV(0.95, 6, 4)`, prob, df1, df2),
		"F.INV.RT":        tdx("返回右尾概率为给定值时 F 分布的值。", `=F.INV.RT(0.05, 6, 4)`, prob, df1, df2),
		"BINOM.DIST":      tdx("返回在若干次试验中取得若干次成功的概率（二项分布）。", `=BINOM.DIST(6, 10, 0.5, FALSE)`, ta("成功次数", "成功的次数。"), ta("试验次数", "独立试验的次数。"), ta("成功概率", "每次试验成功的概率。"), cum),
		"BINOM.INV":       tdx("返回累积概率达到阈值的最小成功次数。", `=BINOM.INV(6, 0.5, 0.75)`, ta("试验次数", "试验的次数。"), ta("成功概率", "成功的概率。"), ta("alpha", "累积概率的阈值。")),
		"NEGBINOM.DIST":   tdx("返回在取得若干次成功之前出现若干次失败的概率（负二项分布）。", `=NEGBINOM.DIST(10, 5, 0.25, TRUE)`, ta("失败次数", "失败的次数。"), ta("成功次数", "目标成功次数。"), ta("成功概率", "成功的概率。"), cum),
		"HYPGEOM.DIST":    tdx("返回无放回抽样时样本中成功次数的概率（超几何分布）。", `=HYPGEOM.DIST(1, 4, 8, 20, TRUE)`, ta("样本成功数", "样本中的成功次数。"), ta("样本量", "样本的大小。"), ta("总体成功数", "总体中的成功次数。"), ta("总体量", "总体的大小。"), cum),
		"POISSON.DIST":    tdx("返回泊松分布：发生若干次事件的概率。", `=POISSON.DIST(2, 5, TRUE)`, ta("x", "事件次数。"), ta("平均值", "事件的期望次数。"), cum),
		"EXPON.DIST":      tdx("返回指数分布（两次事件之间的时间）。", `=EXPON.DIST(0.2, 10, TRUE)`, xPos, ta("lambda", "分布的参数（正数）。"), cum),
		"GAMMA.DIST":      tdx("返回伽玛分布。", `=GAMMA.DIST(10, 9, 2, TRUE)`, xPos, alpha, beta, cum),
		"GAMMA.INV":       tdx("返回累积概率为给定值时伽玛分布的值。", `=GAMMA.INV(0.07, 9, 2)`, prob, alpha, beta),
		"GAMMALN":         tdx("返回伽玛函数的自然对数。", `=GAMMALN(4)`, ta("x", "正数。")),
		"GAMMA":           tdx("返回伽玛函数的值。", `=GAMMA(2.5)`, ta("数值", "数字（不能为 0 或负整数）。")),
		"BETA.DIST":       tdx("返回 0 与 1 之间或两个界限之间的贝塔分布。", `=BETA.DIST(2, 8, 10, TRUE, 1, 3)`, ta("x", "要计算的值，介于 A 与 B 之间。"), ta("alpha", "第一个形状参数。"), ta("beta", "第二个形状参数。"), cum, ta("A", "下限（省略时为 0）。"), ta("B", "上限（省略时为 1）。")),
		"BETA.INV":        tdx("返回累积概率为给定值时贝塔分布的值。", `=BETA.INV(0.69, 8, 10, 1, 3)`, prob, ta("alpha", "第一个形状参数。"), ta("beta", "第二个形状参数。"), ta("A", "下限（省略时为 0）。"), ta("B", "上限（省略时为 1）。")),
		"WEIBULL.DIST":    tdx("返回韦伯分布（寿命、故障）。", `=WEIBULL.DIST(105, 20, 100, TRUE)`, xPos, alpha, beta, cum),
		"CONFIDENCE.NORM": tdx("返回平均值置信区间的半宽（正态分布）。", `=CONFIDENCE.NORM(0.05, 2.5, 50)`, ta("alpha", "风险：0.05 表示 95% 的区间。"), ta("标准差", "总体的标准差。"), ta("样本量", "样本的大小。")),
		"CONFIDENCE.T":    tdx("返回平均值置信区间的半宽（学生 t 分布）。", `=CONFIDENCE.T(0.05, 1, 50)`, ta("alpha", "风险：0.05 表示 95% 的区间。"), ta("标准差", "样本的标准差。"), ta("样本量", "样本的大小。")),
		"STANDARDIZE":     tdx("返回标准化值：(x − 平均值) / 标准差。", `=STANDARDIZE([成绩], 12, 3)`, ta("x", "要标准化的值。"), mean, sd),
		"PHI":             tdx("返回标准正态分布的密度。", `=PHI(0.75)`, x),
		"GAUSS":           tdx("返回标准正态值落在 0 与 z 之间的概率。", `=GAUSS(2)`, ta("z", "要计算的值。")),
		"FISHER":          tdx("返回相关系数的费希尔变换。", `=FISHER(0.75)`, ta("x", "严格介于 −1 与 1 之间的值。")),
		"FISHERINV":       tdx("返回费希尔变换的反函数值。", `=FISHERINV(0.97)`, ta("y", "变换后的值。")),

		"KURT":            tdx("返回一组数据的峰度：至少 4 个值。", `=KURT(销售[金额])`, agg),
		"SKEW":            tdx("返回一组数据（样本）的偏度：至少 3 个值。", `=SKEW(销售[金额])`, agg),
		"SKEW.P":          tdx("返回总体的偏度：至少 3 个值。", `=SKEW.P(销售[金额])`, agg),
		"AVEDEV":          tdx("返回各值与平均值绝对偏差的平均值。", `=AVEDEV(销售[金额])`, agg),
		"DEVSQ":           tdx("返回各值与平均值偏差的平方和。", `=DEVSQ(销售[金额])`, agg),
		"GEOMEAN":         tdx("返回正数的几何平均值。", `=GEOMEAN(利率[系数])`, agg),
		"HARMEAN":         tdx("返回正数的调和平均值。", `=HARMEAN(行程[速度])`, agg),
		"COVARIANCE.S":    tdx("返回两列的样本协方差。", `=COVARIANCE.S(销售[单价], 销售[数量])`, s1, s2),
		"COVARIANCE.P":    tdx("返回两列的总体协方差。", `=COVARIANCE.P(销售[单价], 销售[数量])`, s1, s2),
		"RSQ":             tdx("返回线性回归的决定系数 R²。", `=RSQ(销售[金额], 销售[数量])`, knownY, knownX),
		"STEYX":           tdx("返回线性回归预测值的标准误差。", `=STEYX(销售[金额], 销售[数量])`, knownY, knownX),
		"FORECAST.LINEAR": tdx("用线性回归预测一个值。", `=FORECAST.LINEAR(30, 销售[金额], 销售[数量])`, ta("x", "要预测的位置。"), knownY, knownX),
		"TREND":           tdx("返回回归直线在新 x 处的值。", `=TREND(销售[金额], 销售[数量], [数量])`, knownY, knownX, ta("新x值", "要预测的位置，通常是本行的列（[数量]）。"), konst),
		"GROWTH":          tdx("返回拟合指数曲线在新 x 处的值。", `=GROWTH(销售[金额], 销售[数量], 10)`, knownY, knownX, ta("新x值", "要预测的位置。"), ta("常量", "{=TRUE} 或省略：正常计算；{=FALSE}：曲线在 0 处等于 1。")),
		"LINEST":          tdx("返回回归直线的斜率和截距及其统计量；结果会溢出。", `=LINEST(销售[金额], 销售[数量])`, knownY, knownX, konst, ta("统计", "{=TRUE}：5 行统计量；{=FALSE} 或省略：一行（须写在公式中）。")),
		"LOGEST":          tdx("返回拟合指数曲线的系数及其统计量；结果会溢出。", `=LOGEST(销售[金额], 销售[数量])`, knownY, knownX, ta("常量", "{=TRUE} 或省略：正常计算；{=FALSE}：系数 b 等于 1。"), ta("统计", "{=TRUE}：5 行统计量；{=FALSE} 或省略：一行（须写在公式中）。")),
		"T.TEST":          tdx("返回两组数据学生 t 检验的概率。", `=T.TEST(试验[之前], 试验[之后], 2, 1)`, s1, s2, ta("尾数", "1：单尾检验；2：双尾。"), ta("类型", "1：成对数据；2：方差相等；3：方差不等。")),
		"F.TEST":          tdx("返回两组数据方差相同的双尾概率。", `=F.TEST(试验[之前], 试验[之后])`, s1, s2),
		"Z.TEST":          tdx("返回 z 检验的单尾概率（平均值大于 x）。", `=Z.TEST(试验[测量值], 4)`, ta("数组", "数值列（表[列]）或区域。"), ta("x", "要检验的值。"), ta("sigma", "总体标准差（省略时用样本标准差）。")),
		"CHISQ.TEST":      tdx("返回卡方独立性检验的概率。", `=CHISQ.TEST(Cell[B2:C4], Cell[E2:F4])`, ta("实际区域", "观测值：列或区域。"), ta("期望区域", "期望值，形状相同。")),
		"RANK.AVG":        tdx("返回某值在一列中的排名；并列的值取平均排名。", `=RANK.AVG([得分], 选手[得分])`, ta("数值", "要排名的值。"), ta("引用", "进行排名的列（表[列]）。"), ta("排序", "0 或省略：降序；1：升序。")),
		"AGGREGATE": tdx("计算聚合值，可选择忽略错误值或被筛选隐藏的行。", `=AGGREGATE(9, 6, 销售[金额])`,
			ta("函数编号", "1 {fn:AVERAGE}、2 {fn:COUNT}、3 {fn:COUNTA}、4 {fn:MAX}、5 {fn:MIN}、6 {fn:PRODUCT}、7 {fn:STDEV.S}、8 {fn:STDEV.P}、9 {fn:SUM}、10 {fn:VAR.S}、11 {fn:VAR.P}、12 {fn:MEDIAN}、13 {fn:MODE.SNGL}、14 {fn:LARGE}、15 {fn:SMALL}、16 {fn:PERCENTILE.INC}、17 {fn:QUARTILE.INC}、18 {fn:PERCENTILE.EXC}、19 {fn:QUARTILE.EXC}。"),
			ta("选项", "0 到 7：6 忽略错误值，5 忽略隐藏行，7 两者都忽略（0：都不忽略）。"),
			ta("引用1", "列（表[列]）、区域或表达式；14 到 19 时后接 k。")),

		"XNPV":       tdx("返回带日期现金流的净现值。", `=XNPV(8%, 现金流[金额], 现金流[日期])`, ta("贴现率", "年贴现率。"), ta("值", "现金流所在的列（表[列]）。"), ta("日期", "同一张表中现金流日期所在的列；第一个日期为起点。")),
		"XIRR":       tdx("返回带日期现金流的内部收益率。", `=XIRR(现金流[金额], 现金流[日期])`, ta("值", "现金流所在的列，至少有一个正值和一个负值。"), ta("日期", "同一张表中现金流日期所在的列。"), ta("预估值", "计算的起始利率（省略时为 10%）。")),
		"EFFECT":     tdx("返回每年复利多次的名义利率对应的实际年利率。", `=EFFECT(5%, 12)`, ta("名义利率", "名义年利率。"), ta("每年期数", "每年的复利期数。")),
		"NOMINAL":    tdx("返回实际利率对应的名义年利率。", `=NOMINAL(5.12%, 12)`, ta("实际利率", "实际年利率。"), ta("每年期数", "每年的复利期数。")),
		"SLN":        tdx("返回资产在一个期间内的直线折旧额。", `=SLN(30000, 7500, 10)`, cost, salvage, life),
		"SYD":        tdx("返回资产在一个期间内按年数总和法计算的折旧额。", `=SYD(30000, 7500, 10, 1)`, cost, salvage, life, ta("期次", "期次，从 1 到年限。")),
		"DB":         tdx("返回资产在一个期间内按固定余额递减法计算的折旧额。", `=DB(1000000, 100000, 6, 1, 7)`, cost, salvage, life, period, ta("月份", "第一年的月数（省略时为 12）。")),
		"DDB":        tdx("返回资产在一个期间内按双倍（或其他倍数）余额递减法计算的折旧额。", `=DDB(2400, 300, 10, 1)`, cost, salvage, life, period, ta("倍数", "余额递减速率（省略时为 2）。")),
		"VDB":        tdx("返回资产在两个期间之间的余额递减折旧额，并可转为直线法。", `=VDB(2400, 300, 10, 0, 1)`, cost, salvage, life, ta("起始期", "期间的开始（0：购买时）。"), ta("结束期", "期间的结束。"), ta("倍数", "余额递减速率（省略时为 2）。"), ta("不转换", "{=TRUE}：始终不转为直线法。")),
		"AMORLINC":   tdx("按法国会计制度返回一个期间的直线折旧额（第一期按比例）。", `=AMORLINC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"AMORDEGRC":  tdx("按法国会计制度返回一个期间的递减折旧额。", `=AMORDEGRC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"PRICE":      tdx("返回付息证券每 100 面值的价格。", `=PRICE(DATE(2026,2,15), DATE(2035,11,15), 5.75%, 6.5%, 100, 2, 0)`, settle, mat, coupon, ta("收益率", "证券的年收益率。"), redemption, freq, basis),
		"YIELD":      tdx("根据价格返回付息证券的收益率。", `=YIELD(DATE(2026,2,15), DATE(2034,11,15), 5.75%, 95.04, 100, 2, 0)`, settle, mat, coupon, ta("价格", "每 100 面值的价格。"), redemption, freq, basis),
		"ACCRINT":    tdx("返回付息证券的应计利息。", `=ACCRINT(DATE(2026,3,1), DATE(2026,8,31), DATE(2026,5,1), 10%, 1000, 2, 0)`, ta("发行日", "证券的发行日期。"), ta("首次付息日", "第一次付息的日期。"), settle, coupon, ta("面值", "证券的面值。"), freq, basis, ta("计算方法", "{=TRUE} 或省略：从发行日起；{=FALSE}：从首次付息日起。")),
		"COUPNUM":    tdx("返回结算日与到期日之间尚需支付的付息次数。", `=COUPNUM(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPPCD":    tdx("返回结算日之前的上一个付息日。", `=COUPPCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPNCD":    tdx("返回结算日之后的下一个付息日。", `=COUPNCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYS":   tdx("返回包含结算日的付息期的天数。", `=COUPDAYS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYBS":  tdx("返回从上一个付息日到结算日的天数。", `=COUPDAYBS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYSNC": tdx("返回从结算日到下一个付息日的天数。", `=COUPDAYSNC(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),

		"DSUM":     dbText("对满足条件的行，求某列数值之和。", `=DSUM(销售, "金额", Cell[E1:F3])`),
		"DAVERAGE": dbText("对满足条件的行，返回某列的平均值。", `=DAVERAGE(销售, "金额", Cell[E1:F3])`),
		"DCOUNT":   dbText("对满足条件的行，统计某列中数字的个数。", `=DCOUNT(销售, "金额", Cell[E1:F3])`),
		"DCOUNTA":  dbText("对满足条件的行，统计某列中非空值的个数。", `=DCOUNTA(销售, "客户", Cell[E1:F3])`),
		"DMAX":     dbText("对满足条件的行，返回某列的最大值。", `=DMAX(销售, "金额", Cell[E1:F3])`),
		"DMIN":     dbText("对满足条件的行，返回某列的最小值。", `=DMIN(销售, "金额", Cell[E1:F3])`),
		"DPRODUCT": dbText("对满足条件的行，求某列数值之积。", `=DPRODUCT(利率, "系数", Cell[E1:E2])`),
		"DSTDEV":   dbText("对满足条件的行，返回某列的样本标准差。", `=DSTDEV(销售, "金额", Cell[E1:F3])`),
		"DSTDEVP":  dbText("对满足条件的行，返回某列的总体标准差。", `=DSTDEVP(销售, "金额", Cell[E1:F3])`),
		"DVAR":     dbText("对满足条件的行，返回某列的样本方差。", `=DVAR(销售, "金额", Cell[E1:F3])`),
		"DVARP":    dbText("对满足条件的行，返回某列的总体方差。", `=DVARP(销售, "金额", Cell[E1:F3])`),
		"DGET":     dbText("返回唯一满足条件的行中某列的值。", `=DGET(客户, "城市", Cell[E1:E2])`),

		"OFFSET": tdx("返回按指定行数和列数偏移、大小可指定的引用。", `=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1], 1))`,
			ta("引用", "起始单元格（Cell[A1]）、区域、列（表[列]）或行（表[列][2]）。"),
			ta("行数", "向下偏移（负数：向上）。"), ta("列数", "向右偏移（负数：向左）。"),
			ta("高度", "结果的行数（省略时与引用相同）。"),
			ta("宽度", "结果的列数（省略时与引用相同）。")),
		"INDIRECT": tdx("返回文本中写明的引用：\"B3\"、\"Sheet2!B3\"、\"Cell[A1:A9]\"、\"销售[单价]\"。", `=INDIRECT("Sheet2!B"&Cell[A1])`,
			ta("引用文本", "引用的文本；B3 这样的地址表示自由单元格。"),
			ta("a1", "{=TRUE} 或省略；不支持 {=FALSE}（R1C1 样式）。")),
		"CELL": tdx("返回单元格的信息：\"address\"、\"row\"、\"col\"、\"contents\" 或 \"type\"（b 空白，l 文本，v 值）。", `=CELL("address", Cell[B3])`,
			ta("信息类型", "所需信息，带引号。"), ta("引用", "单元格、区域（取其第一个单元格）或表引用。")),
		"TRANSPOSE": tdx("交换单元格区域或常量的行和列；结果会溢出。", `=TRANSPOSE(Cell[A1:C2])`,
			ta("数组", "单元格区域（Cell[A1:C2]）或常量（{1;2;3}）。")),
	}
}

func init() {
	for _, l := range []struct {
		set  *docSet
		cat  string
		docs map[string]docText
	}{
		{&docsEN, "Statistics", docsAddedEN()},
		{&docsDE, "Statistik", docsAddedDE()},
		{&docsES, "Estadística", docsAddedES()},
		{&docsZH, "统计", docsAddedZH()},
	} {
		l.set.cats[catStats] = l.cat
		for k, v := range l.docs {
			l.set.funcs[k] = v
		}
	}
}
