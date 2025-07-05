package gen

import "testing"

func TestModelStringFQN(t *testing.T) {
   m := Model{Provider: "prov", Name: "nm"}
   want := "prov/nm"
   if s := m.String(); s != want {
       t.Errorf("String() = %q, want %q", s, want)
   }
   if f := m.FQN(); f != want {
       t.Errorf("FQN() = %q, want %q", f, want)
   }
}

func TestToModel(t *testing.T) {
   m, err := ToModel("p/n")
   if err != nil {
       t.Fatalf("ToModel unexpected error: %v", err)
   }
   if m.Provider != "p" || m.Name != "n" {
       t.Errorf("ToModel = %+v, want Provider=\"p\", Name=\"n\"", m)
   }
   if _, err := ToModel("invalid"); err == nil {
       t.Error("ToModel expected error for invalid input, got nil")
   }
}