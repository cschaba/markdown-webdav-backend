package main

import "testing"

func TestVaultSpecs(t *testing.T) {
	var v vaultSpecs
	for _, arg := range []string{"notes=/srv/my notes", "./testdata/vault,nogit", "work=./w,nogit"} {
		if err := v.Set(arg); err != nil {
			t.Fatalf("Set(%q): %v", arg, err)
		}
	}
	want := vaultSpecs{
		{name: "notes", dir: "/srv/my notes", git: true},
		{name: "vault", dir: "./testdata/vault", git: false},
		{name: "work", dir: "./w", git: false},
	}
	if len(v) != len(want) {
		t.Fatalf("got %v", v)
	}
	for i := range want {
		if v[i] != want[i] {
			t.Errorf("spec %d = %+v, want %+v", i, v[i], want[i])
		}
	}
	for _, bad := range []string{"notes=/elsewhere", "dav=./x", "a b=./x", "-=./x", "x=", "/srv/my notes"} {
		if err := v.Set(bad); err == nil {
			t.Errorf("Set(%q) was accepted", bad)
		}
	}
}
