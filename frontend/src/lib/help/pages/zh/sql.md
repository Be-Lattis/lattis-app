# SQL 查询

工作簿中的每个表格都是一个 SQLite 表。**SQL** 选项卡（位于功能区选项卡右侧）使用您在屏幕上看到的名称查询这些表格。

## 编写查询 {#ecrire-une-requete}

```
SELECT Customer, SUM("Net amount") AS Total
FROM Orders
WHERE Paid = 0
GROUP BY Customer
ORDER BY Total DESC
```

- 包含空格、重音字符、大写字母或中文的名称需放在双引号中：`"Net amount"`。
- `Ctrl+Enter` 运行所选内容，未选择时运行光标所在的语句（语句之间以 `;` 或空行分隔）。
- `Ctrl+Space` 提示表格名和列名；`Alt+Up` / `Alt+Down` 浏览历史记录。
- 结果最多显示 1,000 行。

[数据透视表](tcd)可以像表一样查询。`Cell_A1` 表示当前显示的工作表中的一个自由单元格。

## 左侧窗格 {#le-volet-de-gauche}

包含三个列表：工作簿的**对象**（表格、数据透视表及其列；单击即可转到工作表上的相应表格）、**收藏夹**（保存在工作簿中的查询）和**历史记录**。

登录到 Lattis 服务器后，对象列表还会在单独的分组中显示您有权读取的 **Lattis 数据湖数据集**：即 `donnees.name` 视图（双击：插入示例查询）。引用这些视图的查询在服务器上运行，只读，且仅限于您的受众范围内的行；查询使用 PostgreSQL 的 SQL 编写，不能引用工作簿中的表格（若要将两者结合，请将数据集插入工作簿：插入 › Lattis 数据湖）。

## 将结果发送到工作表 {#envoyer-le-resultat-sur-une-feuille}

查询结果可以复制到新表格中，也可以由新表格**引用**：此时表格与查询保持链接，可通过右键 › 表格 › 刷新查询（或 `F5`）重新读取。表格保留结果的标题；技术列（`_pos`）永远不会出现在其中。

### 在链接结果旁边输入 {#ecrire-a-cote-d-un-resultat-lie}

由查询加载的列（蓝色标题，或在视图 › 主题中所选主题的外部数据源颜色）不能编辑。若要添加您自己的信息，请在标题紧右侧键入一个名称：新的**输入列**（绿色标题）可以手动填写或用公式填写，工作簿中的公式可以像引用其他列一样引用它（`Query1[Comment]`）。

每次刷新时，输入列中的值通过**键**跟随各自的行：如果结果中有 `id` 列（如使用 `SELECT *` 时），则使用该列；否则使用一个没有重复值和空单元格的列，Lattis 会在您输入第一个值时询问使用哪一列。如果没有单独一列合适，Lattis 会建议列对：例如对于 `SELECT *, 2021 AS Year FROM Tracking2021 UNION SELECT *, 2022 FROM Tracking2022`，相同的 `id` 值会在不同年份中重复出现，此时由“id + Year”来标识各行。也可以使用行序号，但不推荐：如果查询改变了行的顺序，值将不再跟随正确的行。

- 从结果中消失的行：其值会被**暂存**，当其键再次出现时恢复。右键 › 表格 › 清除暂存的值可将其删除。
- 结果中出现重复的键：该键的值会被暂存，并显示警告。
- 重命名或删除输入列的方式与普通表格相同；键可通过右键 › 表格 › 输入列的键…进行更改。
- 所有操作（包括刷新）都可以用 `Ctrl+Z` 撤消。

由编辑器中的查询加载的表格（[导入数据](connexions)）也同样如此。

## 修改数据 {#modifier-les-donnees}

`INSERT`、`UPDATE` 和 `DELETE` 查询像手动输入一样修改表格，可以用 `Ctrl+Z` 撤消。查询运行期间，**停止**按钮可以中断查询；读取操作在 30 秒后自动停止，写入操作在 2 分钟后自动停止（写入期间会阻止输入）。被停止的写入操作不会做任何修改。

## 在公式中使用查询 {#une-requete-dans-une-formule}

[QUERY](fonction:QUERY) 函数以只读方式运行 `SELECT`，并将结果溢出到工作表上：

```
=QUERY("SELECT Customer, SUM(Amount) FROM Sales GROUP BY Customer")
```

## 在脚本中 {#dans-les-scripts}

`db.query`、`db.rows` 和 `db.execute` 使用同样的 SQL：请参阅 [Lua 脚本](lua)。
