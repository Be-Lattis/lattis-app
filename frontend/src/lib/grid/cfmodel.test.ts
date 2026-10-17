import { describe, expect, it } from "vitest";
import {
  buildManaged,
  diffRules,
  formatLocs,
  locIn,
  parseLocs,
  rects,
  type RuleTable,
  type SheetRules,
} from "./cfmodel";
import type { Rule } from "./rules";

const tables: RuleTable[] = [
  {
    id: 7,
    name: "Ventes",
    row: 0,
    col: 1,
    rows: 10,
    columns: ["Ville", "Montant"],
    rules: [],
  },
];

function sheet(): SheetRules {
  return {
    tables: structuredClone(tables),
    rows: new Map(),
    cols: new Map(),
    cells: new Map(),
  };
}

describe("plages des règles", () => {
  it("regroupe des cellules en rectangles", () => {
    expect(
      rects([
        [0, 0],
        [0, 1],
        [1, 0],
        [1, 1],
        [3, 0],
      ]),
    ).toEqual([
      { r0: 0, c0: 0, r1: 1, c1: 1 },
      { r0: 3, c0: 0, r1: 3, c1: 0 },
    ]);
  });
  it("lit et écrit « S’applique à »", () => {
    const locs = parseLocs("=Ventes[Montant];$E:$F;$20:$21;$H$1:$I$2", tables);
    expect(Array.isArray(locs)).toBe(true);
    if (typeof locs === "string") return;
    expect(locs.length).toBe(1 + 2 + 2 + 4);
    expect(formatLocs(locs, tables)).toBe(
      "=Ventes[Montant];$E:$F;$20:$21;$H$1:$I$2",
    );
    // Une plage qui traverse un tableau : sa colonne, et les cellules autour.
    const mix = parseLocs("C10:C12", tables);
    expect(mix).toEqual([
      { k: "tbl", t: 7, c: 1 },
      { k: "cell", r: 11, c: 2 },
    ]);
    expect(typeof parseLocs("Inconnu[x]", tables)).toBe("string");
    expect(typeof parseLocs("A1:Z100000", tables)).toBe("string");
  });
  it("emplacements dans la sélection", () => {
    expect(locIn({ k: "tbl", t: 7, c: 1 }, 3, 2, 3, 2, tables)).toBe(true);
    expect(locIn({ k: "tbl", t: 7, c: 1 }, 3, 3, 3, 3, tables)).toBe(false);
    expect(locIn({ k: "row", i: 4 }, 3, 0, 5, 0, tables)).toBe(true);
  });
});

describe("gestionnaire des règles", () => {
  it("regroupe une règle posée à plusieurs endroits et garde la priorité", () => {
    const sr = sheet();
    const a: Rule = { id: "a", op: ">", value: "1", style: {} };
    const b: Rule = { id: "b", op: "dup", style: {} };
    sr.tables[0].rules = [undefined, [{ ...a, f: "=1" }, b]];
    sr.cells.set("20,5", [a]);
    sr.cols.set(9, [b]);
    // Anciennes règles sans identifiant : regroupées par contenu.
    sr.rows.set(2, [{ op: "empty", style: {} }]);
    sr.rows.set(3, [{ op: "empty", style: {} }]);
    const list = buildManaged(sr);
    expect(list.map((m) => m.rule.id ?? m.rule.op)).toEqual([
      "a",
      "b",
      "empty",
    ]);
    expect(list[0].locs.length).toBe(2);
    expect(list[0].rule.f).toBeUndefined();
    expect(list[2].locs.length).toBe(2);
    // Rien ne change : aucun changement à envoyer.
    const none = diffRules(sr, list);
    expect(none).toEqual({ columns: [], lines: [], cells: [] });
  });
  it("réordonne, supprime, change la plage", () => {
    const sr = sheet();
    const a: Rule = { id: "a", op: ">", value: "1", style: {} };
    const b: Rule = { id: "b", op: "<", value: "0", style: {} };
    sr.tables[0].rules = [undefined, [a, b]];
    const [ma, mb] = buildManaged(sr);
    // b passe devant a.
    let ch = diffRules(sr, [mb, ma]);
    expect(ch.columns).toEqual([{ table: "Ventes", col: 1, rules: [b, a] }]);
    // a supprimée, b étendue aux lignes 5 à 7 de la feuille.
    ch = diffRules(sr, [
      {
        ...mb,
        locs: [
          ...mb.locs,
          { k: "row", i: 5 },
          { k: "row", i: 6 },
          { k: "row", i: 7 },
        ],
      },
    ]);
    expect(ch.columns).toEqual([{ table: "Ventes", col: 1, rules: [b] }]);
    expect(ch.lines).toEqual([{ rows: true, i0: 5, i1: 7, rules: [b] }]);
    // Tout effacer.
    ch = diffRules(sr, []);
    expect(ch.columns).toEqual([{ table: "Ventes", col: 1, rules: [] }]);
  });
  it("cellules libres : un rectangle par liste", () => {
    const sr = sheet();
    const a: Rule = { id: "a", op: "top", value: "3", style: {} };
    const ch = diffRules(sr, [
      {
        key: "id:a",
        rule: a,
        locs: [
          { k: "cell", r: 0, c: 5 },
          { k: "cell", r: 1, c: 5 },
          { k: "cell", r: 2, c: 5 },
        ],
      },
    ]);
    expect(ch.cells).toEqual([{ r0: 0, c0: 5, r1: 2, c1: 5, rules: [a] }]);
  });
});
