# Lua 脚本

**脚本**选项卡使用 Lua（5.1 版）实现工作簿自动化：读写表格、操作工作表、向用户提问、响应控件。

## 脚本选项卡 {#l-onglet-script}

- **左侧是脚本库**：保存在工作簿中的脚本（它们随 `.lattis` 文件一起保存和传递）。右键：重命名、复制、删除。
- **中间是编辑器**，在您键入时检查脚本：语法错误显示为红色，未知名称和参数个数错误显示为黄色。
- **右侧是参考**：即下文列出的函数，并有按钮可插入其示例。
- **底部是输出**：显示 `print` 输出的内容，以及运行结果摘要。

`Ctrl+Enter` 运行脚本，`Ctrl+S` 将脚本保存到工作簿中（之后别忘了保存工作簿）。

## 读取和写入 {#lire-et-ecrire}

查询使用 SQL 编写，使用表格和列的显示名称；包含空格、重音字符或中文的名称需放在双引号中：

```
for r in db.rows('SELECT Customer, sum("Net amount") AS total FROM Orders GROUP BY Customer') do
  print(r.Customer, r.total)
end
```

`db.insert`、`db.update` 和 `db.delete` 的写入效果与在网格中键入相同。脚本写入的所有内容构成**一个撤消步骤**：`Ctrl+Z` 可一次全部撤消。

## 安全和限制 {#securite-et-limites}

脚本在沙盒中运行：无法访问文件、程序或环境变量。HTTP 请求（`http.get` 等）只能访问您已允许的网站。脚本在运行 30 秒后、内存超过 512 MB 时或通过停止按钮被终止。`db.query` 最多返回 100,000 行（`db.rows` 没有限制）。

随工作簿收到的脚本（按钮、控件、事件）在首次运行前会征求您的同意：**允许**或**拒绝**，并提供**查看代码**。脚本可以读取和修改整个工作簿：只有在信任向您提供此工作簿的人时才允许运行。在本机上编写的脚本会自动获得允许。

打开工作簿时，Lattis 会删除工作簿中并非由它自己创建的 SQL 触发器（`TRIGGER`）（Lattis 从不在文件中保存触发器），并在状态栏中提示。

## 按钮和控件 {#boutons-et-controles}

脚本中的函数可以附加到工作表上的按钮或控件：请参阅[按钮和控件](controles)。

## 工作簿事件 {#evenements-du-classeur}

脚本可以响应您在工作簿中的操作：`events.on(event, function, script)` 让已保存脚本中的某个函数订阅事件，订阅随文件一起保存。

```
function entry(e)
  ui.notify(e.address .. ": " .. tostring(e.old) .. " → " .. tostring(e.value))
end
events.on("change", "entry", "My script")
```

| 事件 | 触发时机 |
|---|---|
| `open` | 工作簿刚刚打开 |
| `before_save` / `save` | 保存之前 / 之后（返回 `false` 可取消保存） |
| `before_close` | 工作簿或窗口关闭之前（返回 `false` 可取消关闭） |
| `sheet` | 显示了另一个选项卡 |
| `selection` | 所选内容已更改（停止移动后） |
| `change` | 单元格已被修改：输入、粘贴、清除、移动、表格行 |
| `sheet_add` / `sheet_delete` | 添加 / 删除了工作表 |
| `recalc` | 公式已重新计算（F9） |

函数会收到一个表：`sheet`、`row`、`col`、`address`（“B3”），对于单个输入的单元格还有 `value` 和 `old` 等。只有您的操作会触发这些事件，脚本永远不会：向工作表写入内容的监听器不会再次触发自身。监听器运行时不会阻塞网格，其所做的修改可一次撤消（`Ctrl+Z`），错误只报告一次。`events.enable(false)` 可在本次会话中关闭这些监听器。

## 函数参考 {#reference-des-fonctions}
