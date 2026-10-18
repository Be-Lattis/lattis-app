package instance

import (
	"path/filepath"
	"testing"
	"time"
)

func TestForward(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.json")
	file = func() (string, error) { return path, nil }
	if err := Forward("lattis://open"); err != ErrNoInstance {
		t.Fatalf("sans instance : %v", err)
	}
	got := make(chan string, 1)
	stop, err := Serve(func(l string) { got <- l })
	if err != nil {
		t.Fatal(err)
	}
	const link = "lattis://open?server=https%3A%2F%2Fa.fr&workbook=x"
	if err := Forward(link); err != nil {
		t.Fatal(err)
	}
	select {
	case l := <-got:
		if l != link {
			t.Fatalf("reçu %q", l)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lien non reçu")
	}
	stop()
	if err := Forward(link); err != ErrNoInstance {
		t.Fatalf("après l'arrêt : %v", err)
	}
}
