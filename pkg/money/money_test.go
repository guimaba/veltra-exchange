package money

import "testing"

func TestParseAndString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.5", "1.50000000"},
		{"1", "1.00000000"},
		{"0", "0.00000000"},
		{"0.00000001", "0.00000001"},
		{"-0.00000001", "-0.00000001"},
		{"1000", "1000.00000000"},
		{"-42.42", "-42.42000000"},
		{".5", "0.50000000"},
		{"+3", "3.00000000"},
	}
	for _, c := range cases {
		a, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q) erro inesperado: %v", c.in, err)
		}
		if got := a.String(); got != c.want {
			t.Errorf("Parse(%q).String() = %q, quero %q", c.in, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{"", "abc", "1.2.3", "1.234567890", "--1", "1e9"}
	for _, s := range bad {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) deveria falhar, mas passou", s)
		}
	}
}

func TestNoFloatDrift(t *testing.T) {
	// O classico 0.1 + 0.2 != 0.3 em float NAO pode acontecer aqui.
	a := MustParse("0.1")
	b := MustParse("0.2")
	sum, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	if sum != MustParse("0.3") {
		t.Errorf("0.1 + 0.2 = %s, quero 0.30000000", sum)
	}
}

func TestAddSubOverflow(t *testing.T) {
	max := Amount(maxInt64)
	if _, err := max.Add(1); err != ErrOverflow {
		t.Errorf("esperava overflow ao somar 1 ao max, veio %v", err)
	}
	min := Amount(minInt64)
	if _, err := min.Sub(1); err != ErrOverflow {
		t.Errorf("esperava overflow ao subtrair 1 do min, veio %v", err)
	}
	a, err := MustParse("10").Sub(MustParse("3"))
	if err != nil || a != MustParse("7") {
		t.Errorf("10 - 3 = %s (err=%v), quero 7", a, err)
	}
}

func TestNotional(t *testing.T) {
	cases := []struct {
		price, qty, want string
	}{
		{"2", "1.5", "3.00000000"},
		{"0.5", "0.5", "0.25000000"},
		{"100", "0.01", "1.00000000"},
		{"1.23456789", "2", "2.46913578"},
	}
	for _, c := range cases {
		n, err := Notional(MustParse(c.price), MustParse(c.qty))
		if err != nil {
			t.Fatalf("Notional(%s,%s) erro: %v", c.price, c.qty, err)
		}
		if n.String() != c.want {
			t.Errorf("Notional(%s,%s) = %s, quero %s", c.price, c.qty, n, c.want)
		}
	}
}

func TestNotionalTruncates(t *testing.T) {
	// 0.00000001 * 0.5 = 0.000000005 -> trunca para 0 na menor unidade.
	n, err := Notional(MustParse("0.00000001"), MustParse("0.5"))
	if err != nil {
		t.Fatal(err)
	}
	if !n.IsZero() {
		t.Errorf("esperava truncar para 0, veio %s", n)
	}
}

func TestCmp(t *testing.T) {
	a := MustParse("1.5")
	b := MustParse("2.5")
	if a.Cmp(b) != -1 || b.Cmp(a) != 1 || a.Cmp(a) != 0 {
		t.Errorf("Cmp incorreto")
	}
}
