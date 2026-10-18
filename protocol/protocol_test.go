package protocol

import (
	"encoding/json"
	"testing"
)

func TestLinks(t *testing.T) {
	const id = "0192f0c4-7a1e-7b2c-9d3e-4f5a6b7c8d9e"
	if got := WorkbookLink("https://lattis.acme.fr/", id); got != "https://lattis.acme.fr/w/"+id {
		t.Fatalf("WorkbookLink = %s", got)
	}
	l := DesktopLink("https://lattis.acme.fr/base", id)
	if l != "lattis://open?server=https%3A%2F%2Flattis.acme.fr%2Fbase&workbook="+id {
		t.Fatalf("DesktopLink = %s", l)
	}
	iss, got, err := ParseDesktopLink(l)
	if err != nil || iss != "https://lattis.acme.fr/base" || got != id {
		t.Fatalf("ParseDesktopLink = %q, %q, %v", iss, got, err)
	}
	for _, bad := range []string{
		"https://lattis.acme.fr/w/" + id,
		"lattis://delete?server=https%3A%2F%2Fa.fr&workbook=" + id,
		"lattis://open/x?server=https%3A%2F%2Fa.fr&workbook=" + id,
		"lattis://open?server=https%3A%2F%2Fa.fr&workbook=../etc",
		"lattis://open?server=file%3A%2F%2F%2Fetc&workbook=" + id,
		"lattis://open?server=javascript%3Aalert(1)&workbook=" + id,
		"lattis://open?server=https%3A%2F%2Fu%3Ap%40a.fr&workbook=" + id,
		"lattis://open?workbook=" + id,
	} {
		if _, _, err := ParseDesktopLink(bad); err == nil {
			t.Errorf("accepté : %s", bad)
		}
	}
}

func TestDecode(t *testing.T) {
	var d Discovery
	if err := json.Unmarshal([]byte(`{"name":"Acme","edition":"web","postgres":{"database":"lattis","sslmode":"verify-full","rw":{"host":"pg.acme.fr","port":5432},"ro":["r1.acme.fr:5433"]}}`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Edition != "web" || d.Postgres.RW != "pg.acme.fr:5432" || len(d.Postgres.RO) != 1 || d.Postgres.RO[0] != "r1.acme.fr:5433" {
		t.Fatalf("découverte : %+v", d)
	}
	var s LoginSummary
	if err := json.Unmarshal([]byte(`{"server":{"display_name":"Acme","access_mode":"open"},"user":{"email":"a@acme.fr","pg_role":"a@acme.fr"},"server_owner":false,"workbook_creator":false,"can_create_workbooks":false,"workbooks":[]}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.CanCreate() || s.User.PGRole != "a@acme.fr" || s.Server.AccessMode != "open" {
		t.Fatalf("résumé : %+v", s)
	}
	if !(LoginSummary{}).CanCreate() {
		t.Fatal("serveur d'avant le droit : publication refusée")
	}
}
