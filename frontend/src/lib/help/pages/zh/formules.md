# 公式

公式以 `=` 开头。函数使用英文名称书写（`SUM`、`IF`、`COUNTIF` 等）。参数之间用 `,` 分隔，小数使用小数点：`=ROUND([Price]*1.2, 2)`。

键入时，Lattis 会提示函数、表格和列，显示当前函数的签名（待输入的参数以粗体显示），并为引用着色。在此按 `F1` 会打开光标所在函数的帮助页面；公式 › 插入函数可打开完整列表。

## 引用 {#references}

| 写法 | 表示 |
| --- | --- |
| `[Column]` | 在表格中：该列在公式所在行的值 |
| `[Column][2]` | 同一表格中该列第 2 行的值 |
| `Table[Column]` | 整列（用于聚合函数中，或在表格之外）；单独位于自由单元格中时（`=Sales[Price]`），其值会溢出到下方 |
| `Table[Column][2]` | 按行序号指定的单个值（1 = 第一个数据行） |
| `Table[Column][1:3]` | 某列的第 1 至 3 行 |
| `Cell[A1]` | 工作表中的自由单元格 A1 |
| `Sheet2!Cell[A1]` | 另一个工作表中的自由单元格 A1（如果名称包含空格，则写作 `'My sheet'!Cell[A1]`） |
| `Cell[C14:C16]` | 一个自由单元格区域 |

行序号遵循显示顺序：对表格排序会改变所指的行，与 Excel 相同。若要按键查找某一行，请使用 [REFERENCE](fonction:REFERENCE)。

在公式模式下，单击单元格会插入其引用；拖动则插入一个区域。从自由单元格出发，单击表格中的单元格会插入该单个值（`Report[TEST][2]`，按其行序号）；单击标题或拖过整列会插入整列（`Report[TEST]`）。

## Excel 样式的地址 {#adresses-a-la-excel}

出于习惯键入的地址（`=C14*2`、`=SUM(C14:C16)`、`$C$14`、`Sheet2!B3`）会在确认时自动转换：自由单元格转换为 `Cell[C14]`，表格中的单元格转换为 `Table[Column][k]`，`C14:C16` 转换为区域。

[VLOOKUP](fonction:VLOOKUP)、[HLOOKUP](fonction:HLOOKUP) 和 [XLOOKUP](fonction:XLOOKUP) 也可用于自由单元格（`=VLOOKUP(A2, D2:F20, 2, FALSE)`），支持精确或近似匹配：它们会转换为 [INDEX](fonction:INDEX) 与 [MATCH](fonction:MATCH) 或 [XMATCH](fonction:XMATCH)。

## 计算列 {#colonnes-calculees}

在表格列中键入的公式会应用于该列的所有行：该列成为**计算列**。其中的 `[Column]` 表示同一行的值。聚合函数接受基于表格各列的表达式，并逐行计算：`=SUM(Sales[Price]*Sales[Qty])`。

### 转换为值 {#convertir-en-valeurs}

右键单击计算列 › **转换为值**（或公式 › 转换为值）：公式被删除，该列成为普通列，保留其当前值，类型取决于这些值（整数、小数、日期、布尔值，否则为文本；`#DIV/0!` 之类的错误仍会标示）。列名保持不变：引用它的公式会继续读取它，但它不再跟随其公式原先读取的列。选中多个计算列时会一起转换；在自由单元格上，该命令会将公式替换为其值（溢出的结果保留其公式）。按 Ctrl+Z 可恢复公式。在受保护的结构下，以及目前在协作工作簿的共享表格中，不能使用此命令。

## 溢出的结果 {#resultats-qui-se-deversent}

某些函数会返回多个值（[FILTER](fonction:FILTER)、[SORT](fonction:SORT)、[UNIQUE](fonction:UNIQUE)、[SEQUENCE](fonction:SEQUENCE)、[TEXTSPLIT](fonction:TEXTSPLIT)、[QUERY](fonction:QUERY) 等）：结果会溢出到相邻的单元格中，与 Excel 的动态数组相同。单独位于自由单元格中的整列（`=Sales[Price]`）也会以同样方式按表格的行顺序溢出。如果位置已被占用，公式会显示 `#SPILL!`。在聚合函数中，这些函数提供其值：`=COUNTA(TEXTSPLIT(Cell[A1], ","))` 计算文本被拆分的段数；而遍历列的函数（COUNTIF、LARGE、MATCH 等）不读取这些值（`#VALUE!`）。UNIQUE、SORT、TAKE 和 DROP 可读取一列（`=UNIQUE(NestTracking[Nest ID])`）、整个表格、单元格区域（`Cell[A1:A20]`）或另一个动态数组；FILTER 和 SORTBY 可读取表格或列。

`=TEXTSPLIT("Name=Smith;City=Paris", "=", ";")` 先将文本按行拆分（在 `;` 处），再按列拆分（在 `=` 处）；较短的行用 `#N/A` 或最后一个参数补齐。要溢出，文本不能依赖于表格的某一行：应为单元格，或 `Sales[Notes][2]` 这样的值。

**数组常量**写在大括号中：`{1;2;3}`（一列）、`{"a","b"}`（一行）、`{1.2;3.4}`。它像动态数组一样溢出，特别适合用于指定多个分隔符：`=TEXTSPLIT(Cell[A1], {","," "})`。

### 将溢出的结果转换为表格 {#convertir-un-resultat-deverse-en-tableau}

溢出区域带有蓝色边框；其工具提示会显示公式所在的单元格。**右键单击**该区域：

- **转换为链接表格**：用一个真正的表格替换结果。表格与公式保持链接（公式保存在工作簿的一个查询中，数据源为“公式”，可在数据 › 查询中看到），并在其数据源变化时**自动重新计算**：在 `NestTracking` 中输入内容会立即更新由 `=UNIQUE(NestTracking[Nest ID])` 生成的表格，一次 Ctrl+Z 即可同时撤消输入和重新计算。其计算列为只读（蓝色，即主题中外部数据源的颜色）；在其标题右侧键入名称可添加一个**输入列**（绿色，即表格的颜色），结果变化时，输入列中的值会跟随其所在行（对于针对一列的 `UNIQUE`，按该列本身；否则按所要求的键），就像在查询结果旁边一样。按 F5 也可以刷新它。如果向其查询添加步骤（数据 › 查询），则之后只能通过 F5 刷新。
- **转换为表格（值）**：生成一个普通表格，保留当前的值，不带链接。

在这两种情况下，**标题**取自公式正上方的文本（前提是每一列上方都有文本；例如 `=UNIQUE(…)` 上方的标题“Nests”会成为表格的标题，表格名为 Nests）；否则使用公式结果的列名（“Nest ID”，对于 SEQUENCE 为“Value”），放在公式上方的空行中（值不移动），如果没有空行，则放在公式所在的行（值下移一行）。一个撤消步骤即可恢复公式。

## 以文本形式存储的数字 {#nombres-ecrits-en-texte}

在文本列中，需要数字的函数（[SUM](fonction:SUM)、[AVERAGE](fonction:AVERAGE)、[MIN](fonction:MIN)、[MAX](fonction:MAX)、[SUMIF](fonction:SUMIF)、[SUMPRODUCT](fonction:SUMPRODUCT)、统计函数等）会将每个文本读取为它所表示的数字：“12”、“1,234.5”、“$12”、“15%”（0.15）、“(12)”（−12），以及其他地区的写法“1.234,56”或“1'234.5”。**不是数字的文本按 0 计算**：“12”和“abc”的平均值为 6。空单元格仍然被忽略。[COUNT](fonction:COUNT) 只计算是数字的文本：`=COUNTA(Sales[Score]) - COUNT(Sales[Score])` 给出不是数字的文本个数。

Excel 的情况不同，它会忽略这些文本。自由单元格区域（`Cell[A1:A10]`）遵循 Excel 的规则：其中的文本被视为标签而忽略。在直接计算中（`=[Score]*2`），不是数字的文本会得到 `#VALUE!`。

## 中间名称 {#noms-intermediaires}

[LET](fonction:LET) 为值命名，这些值只计算一次，可用于后续的值和最终计算中：`=LET(net, [Price]*[Qty], tax, net*0.2, net+tax)`。名称不能是单元格地址（`A1`、`Cell[A1]`），也不能是函数名。

## 错误值 {#valeurs-d-erreur}

| 错误 | 常见原因 |
| --- | --- |
| `#VALUE!` | 参数类型错误，或混用了两个表格的列 |
| `#DIV/0!` | 除数为零 |
| `#N/A` | 找不到值（XLOOKUP、MATCH、REFERENCE、MUNICIPALITY 等） |
| `#NAME?` | 未知的函数、表格或列 |
| `#REF!` | 单元格或区域已被删除 |
| `#CIRC!` | 循环引用：公式依赖于自身 |
| `#NUM!` | 数字超出范围（负数的平方根等） |

## 统计和财务 {#statistiques-et-finances}

统计函数可读取整列、区域或基于表格各列的表达式；其中的布尔值和空值被忽略，文本按上文所述方式读取：[STDEV.S](fonction:STDEV.S)、[VAR](fonction:VAR)、[MODE](fonction:MODE)、[PERCENTILE](fonction:PERCENTILE)、[QUARTILE](fonction:QUARTILE)、[CORREL](fonction:CORREL)、[SLOPE](fonction:SLOPE) 等：`=QUARTILE(Sales[Amount], 3)`。

财务函数遵循 Excel 的约定：支出的金额为负数，利率为每期利率。一笔 200,000 元、期限 20 年、年利率 4% 的贷款，其月还款额写作 `=PMT(4%/12, 20*12, 200000)`（−1,211.96）。另请参阅 [PV](fonction:PV)、[FV](fonction:FV)、[NPER](fonction:NPER)、[RATE](fonction:RATE)、[IPMT](fonction:IPMT) 和 [PPMT](fonction:PPMT)，用于制作分期还款表。[NPV](fonction:NPV) 和 [IRR](fonction:IRR) **按表格的显示顺序**读取某列中的现金流：`=IRR(CashFlows[Amount])`。

还提供概率分布（[NORM.DIST](fonction:NORM.DIST)、[T.DIST](fonction:T.DIST)…）、检验（[T.TEST](fonction:T.TEST)）、回归（[FORECAST.LINEAR](fonction:FORECAST.LINEAR)、[LINEST](fonction:LINEST)）、折旧（[SLN](fonction:SLN)、[AMORDEGRC](fonction:AMORDEGRC)）和证券（[PRICE](fonction:PRICE)）函数。[DSUM](fonction:DSUM) 等数据库函数从单元格区域读取条件：`=DSUM(销售, "金额", Cell[E1:F3])`。[OFFSET](fonction:OFFSET) 和 [INDIRECT](fonction:INDIRECT) 计算引用：`=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1]))`。

## GPS 点所在的市镇 {#commune-d-un-point-gps}

[MUNICIPALITY](fonction:MUNICIPALITY) 返回包含某个点的法国市镇（commune）：`=MUNICIPALITY([Latitude], [Longitude])`，或其 INSEE 代码（`"code"`）、邮政编码（`"cp"`）、省（`"departement"`）或大区（`"region"`）。计算无需联网，基于 Lattis 内置的市镇边界（法国本土和海外领地），即使对 100,000 行的列也很快。位于海上或法国境外的点返回 `#N/A`。由于边界经过简化，距离边界几十米以内的点可能会落入相邻的市镇。

*包含 IGN 和 INSEE 的数据（ADMIN EXPRESS 市镇边界，2026 年版，由 Etalab 发布）以及经由 geo.api.gouv.fr 获取的 La Poste 邮政编码，依据 Licence Ouverte / Etalab 2.0 许可。*

## 公式之间的链接 {#liens-entre-formules}

**公式 › 显示链接**用于追踪所选单元格的链接：蓝色箭头从其公式读取的内容指向它们，红色箭头从它们指向读取它们的公式。整列（`Customers[City]`）以其标题表示；具体某一行（`Customers[City][1]`、`[Balance][-1]`、由 `REFERENCE` 找到的行）以其单元格表示，行区域（`Sales[Amount][2:5]`）则带有边框。如果该行被隐藏（筛选、选择选项卡、隐藏的行）或找不到，箭头会指向该列，并在状态栏中说明原因。指向其他工作表的链接会在状态栏中计数。在网格中单击（或按 `Esc`）可清除箭头。

## 模拟分析 {#analyse-de-scenarios}

**数据 › 模拟分析**：**单变量求解**查找使公式得到所需结果的单元格值；**方案管理器**保存命名的值组，显示它们并在新工作表上生成摘要；**模拟运算表**为一组值计算公式（单变量或双变量，结果为固定值：再次运行即可重新计算）；**规划求解**在约束下求最大值、最小值或目标值（线性问题为精确解，否则为近似解）。点击**保留**之前不会更改任何内容；`Ctrl+Z` 可撤销。在文件 › 设置 › 功能中开启。

## 重新计算 {#recalcul}

公式在其数据变化时会自动重新计算。`F9` 重新计算所有内容；`TODAY`、`NOW` 和 `RAND` 只在打开时和按 `F9` 时才会变化。

## 所有函数 {#toutes-les-fonctions}
