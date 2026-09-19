package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// vaultSpec is one -vault argument: [name=]directory[,nogit].
type vaultSpec struct {
	name string
	dir  string
	// git is false for a vault whose files something else versions, such as
	// the test vault inside this project's own repository. The server must
	// not create a repository there, nor commit into one it does not own.
	git bool
}

// A vault name is part of two URLs, "/<name>/" and "/dav/<name>/".
var vaultName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type vaultSpecs []vaultSpec

func (v *vaultSpecs) String() string { return fmt.Sprint(*v) }

func (v *vaultSpecs) Set(arg string) error {
	spec := vaultSpec{git: true}
	arg = strings.TrimSpace(arg)
	if rest, ok := strings.CutSuffix(arg, ",nogit"); ok {
		arg, spec.git = rest, false
	}
	if name, dir, ok := strings.Cut(arg, "="); ok {
		spec.name, spec.dir = name, dir
	} else {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return err
		}
		spec.name, spec.dir = filepath.Base(abs), arg
	}
	switch {
	case spec.dir == "":
		return fmt.Errorf("vault %q has no directory", arg)
	case !vaultName.MatchString(spec.name):
		return fmt.Errorf("vault name %q: use letters, digits, - and _ (give one with name=directory)", spec.name)
	case spec.name == "dav":
		return fmt.Errorf("vault name %q is reserved for the WebDAV endpoint", spec.name)
	}
	for _, other := range *v {
		if other.name == spec.name {
			return fmt.Errorf("two vaults are named %q", spec.name)
		}
	}
	*v = append(*v, spec)
	return nil
}
