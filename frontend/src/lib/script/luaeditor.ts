// Extensions CodeMirror de l'éditeur de scripts : linter Lua (lualint.ts) et
// bulle d'aide au survol d'une fonction de la référence (reference.ts).

import { RangeSetBuilder, type Extension } from "@codemirror/state";
import {
  Decoration,
  EditorView,
  ViewPlugin,
  WidgetType,
  hoverTooltip,
  keymap,
  showPanel,
  type DecorationSet,
  type ViewUpdate,
} from "@codemirror/view";
import {
  forEachDiagnostic,
  linter,
  lintGutter,
  lintKeymap,
  nextDiagnostic,
  type Diagnostic,
} from "@codemirror/lint";
import { lintLua } from "./lualint";
import { byName } from "./reference";

const luaLinter = linter(
  (view): Diagnostic[] =>
    lintLua(view.state.doc.toString()).map((d) => ({
      from: d.from,
      to: d.to,
      severity: d.severity,
      message: d.message,
      source: "Lua",
    })),
  { delay: 300 },
);

// Nom sous le pointeur, avec sa bibliothèque : « db.query », « print », « for ».
const word = /[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?/g;

const luaHover = hoverTooltip((view, pos) => {
  const line = view.state.doc.lineAt(pos);
  for (const m of line.text.matchAll(word)) {
    const from = line.from + m.index!;
    const to = from + m[0].length;
    if (pos < from || pos > to) continue;
    const entry = byName.get(m[0]);
    if (!entry) return null;
    return {
      pos: from,
      end: to,
      above: true,
      create() {
        const dom = document.createElement("div");
        dom.className = "cm-lua-help";
        const sig = document.createElement("code");
        sig.textContent = entry.signature;
        const sum = document.createElement("div");
        sum.textContent = entry.summary;
        dom.append(sig, sum);
        return { dom };
      },
    };
  }
  return null;
});

// Diagnostics bien visibles pendant la frappe (comme Error Lens) : la ligne
// teintée et le message écrit à sa fin, en plus du soulignement et de la
// marque dans la marge.
type Sev = "error" | "warning" | "info" | "hint";
const rank: Record<Sev, number> = { error: 3, warning: 2, info: 1, hint: 0 };

class LensWidget extends WidgetType {
  constructor(
    readonly severity: Sev,
    readonly message: string,
  ) {
    super();
  }
  eq(o: LensWidget) {
    return o.severity === this.severity && o.message === this.message;
  }
  toDOM() {
    const el = document.createElement("span");
    el.className = "cm-lua-lens cm-lua-lens-" + this.severity;
    el.textContent = this.message;
    el.title = this.message;
    return el;
  }
  ignoreEvent() {
    return true;
  }
}

function lensDecorations(view: EditorView): DecorationSet {
  // Par ligne : le diagnostic le plus grave, et combien d'autres.
  const lines = new Map<
    number,
    { line: number; end: number; severity: Sev; message: string; more: number }
  >();
  forEachDiagnostic(view.state, (d, from) => {
    const line = view.state.doc.lineAt(Math.min(from, view.state.doc.length));
    const cur = lines.get(line.number);
    const severity = d.severity as Sev;
    if (!cur)
      lines.set(line.number, {
        line: line.from,
        end: line.to,
        severity,
        message: d.message,
        more: 0,
      });
    else if (rank[severity] > rank[cur.severity])
      lines.set(line.number, {
        ...cur,
        severity,
        message: d.message,
        more: cur.more + 1,
      });
    else cur.more++;
  });
  const b = new RangeSetBuilder<Decoration>();
  for (const n of [...lines.keys()].sort((a, c) => a - c)) {
    const l = lines.get(n)!;
    const text = l.message + (l.more ? ` (+${l.more})` : "");
    b.add(
      l.line,
      l.line,
      Decoration.line({ class: "cm-lua-line-" + l.severity }),
    );
    b.add(
      l.end,
      l.end,
      Decoration.widget({ widget: new LensWidget(l.severity, text), side: 1 }),
    );
  }
  return b.finish();
}

const lens = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = lensDecorations(view);
    }
    update(u: ViewUpdate) {
      // Les diagnostics arrivent par une transaction à part (après le délai).
      if (u.docChanged || u.transactions.some((t) => t.effects.length))
        this.decorations = lensDecorations(u.view);
    }
  },
  { decorations: (v) => v.decorations },
);

// Bilan sous l'éditeur : « 1 erreur · 2 avertissements » ; un clic va au suivant.
function summary(view: EditorView): { text: string; level: string } {
  let errors = 0;
  let warnings = 0;
  forEachDiagnostic(view.state, (d) =>
    d.severity === "error" ? errors++ : warnings++,
  );
  if (!errors && !warnings)
    return { text: "✓ Aucun problème détecté", level: "ok" };
  const parts = [];
  if (errors) parts.push(`${errors} erreur${errors > 1 ? "s" : ""}`);
  if (warnings)
    parts.push(`${warnings} avertissement${warnings > 1 ? "s" : ""}`);
  return {
    text: parts.join(" · ") + " — cliquer pour aller au suivant (F8)",
    level: errors ? "error" : "warning",
  };
}

const lintSummary = showPanel.of((view) => {
  const dom = document.createElement("button");
  dom.type = "button";
  dom.className = "cm-lua-summary";
  const paint = () => {
    const s = summary(view);
    dom.textContent = s.text;
    dom.dataset.level = s.level;
  };
  dom.onclick = () => {
    nextDiagnostic(view);
    view.focus();
  };
  paint();
  return { dom, bottom: true, update: paint };
});

const theme = EditorView.baseTheme({
  ".cm-lua-line-error": { backgroundColor: "rgba(220, 38, 38, 0.07)" },
  ".cm-lua-line-warning": { backgroundColor: "rgba(217, 119, 6, 0.07)" },
  ".cm-lua-lens": {
    marginLeft: "2.5em",
    fontFamily: "'Segoe UI', system-ui, sans-serif",
    fontSize: "12px",
    fontStyle: "italic",
    whiteSpace: "pre",
  },
  ".cm-lua-lens-error": { color: "#b91c1c" },
  ".cm-lua-lens-warning": { color: "#b45309" },
  ".cm-lua-lens-info, .cm-lua-lens-hint": { color: "#6b7280" },
  ".cm-lintRange-error": {
    backgroundImage: "none",
    textDecoration: "underline wavy #dc2626",
    textDecorationThickness: "1.5px",
    textUnderlineOffset: "3px",
  },
  ".cm-lintRange-warning": {
    backgroundImage: "none",
    textDecoration: "underline wavy #d97706",
    textDecorationThickness: "1.5px",
    textUnderlineOffset: "3px",
  },
  ".cm-lua-summary": {
    display: "block",
    width: "100%",
    textAlign: "left",
    border: "0",
    borderTop: "1px solid #e3e5e8",
    padding: "3px 10px",
    font: "12px 'Segoe UI', system-ui, sans-serif",
    background: "#f7f8f9",
    color: "#3c7a52",
    cursor: "pointer",
  },
  '.cm-lua-summary[data-level="error"]': {
    color: "#b91c1c",
    background: "#fef2f2",
  },
  '.cm-lua-summary[data-level="warning"]': {
    color: "#b45309",
    background: "#fffbeb",
  },
  ".cm-lua-help": {
    padding: "4px 8px",
    maxWidth: "440px",
    font: "12px 'Segoe UI', system-ui, sans-serif",
  },
  ".cm-lua-help code": {
    display: "block",
    marginBottom: "2px",
    fontFamily: "Consolas, 'Cascadia Mono', monospace",
    color: "#14532d",
  },
});

/** Linter, marges d'erreur et aide au survol. */
export function luaTools(): Extension[] {
  return [
    luaLinter,
    lintGutter(),
    lens,
    lintSummary,
    keymap.of(lintKeymap),
    luaHover,
    theme,
  ];
}
