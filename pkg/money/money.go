// Package money implementa aritmetica de ponto fixo (fixed-point) sobre int64,
// conforme exigido pelo plano tecnico da Veltra Exchange (secao 4.2.2):
//
//	"Aritmetica inteira (fixed-point): precos e quantidades em inteiros
//	 (ticks e a menor unidade do ativo). Nunca ponto flutuante perto de
//	 saldo, 0,1 + 0,2 != 0,3 em float."
//
// Toda quantia e representada internamente como um int64 escalado por 10^Decimals
// (a "menor unidade"). Ex.: com Decimals=8, o valor 1.5 e armazenado como
// 150000000. Isso elimina erros de arredondamento de float64 em saldos, precos
// e quantidades, e torna o matching engine deterministico.
package money

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
)

// Decimals e o numero de casas decimais da menor unidade. 8 casas e o padrao de
// mercado para ativos digitais (estilo satoshi) e cobre VLT e USDT-sim com folga.
const Decimals = 8

// Scale = 10^Decimals. Fator de conversao entre unidade "humana" e menor unidade.
const Scale int64 = 100_000_000 // 10^8

// Amount e uma quantia em menor unidade (int64 escalado por Scale).
// Representa saldos, precos e quantidades. Nunca usar float64 ao lado deste tipo.
type Amount int64

var (
	// ErrOverflow indica estouro de int64 em uma operacao aritmetica.
	ErrOverflow = errors.New("money: overflow aritmetico")
	// ErrInvalidFormat indica string nao parseavel como decimal.
	ErrInvalidFormat = errors.New("money: formato decimal invalido")
	// ErrTooManyDecimals indica mais casas decimais do que Decimals suporta.
	ErrTooManyDecimals = errors.New("money: casas decimais alem da precisao suportada")
	// ErrDivisionByZero indica divisao por zero.
	ErrDivisionByZero = errors.New("money: divisao por zero")
)

// Zero e a quantia nula, conveniencia para comparacoes e inicializacao.
const Zero Amount = 0

// FromInt converte um inteiro de unidades "humanas" (ex.: 5 -> 5.00000000).
func FromInt(units int64) (Amount, error) {
	// units * Scale com checagem de overflow.
	hi := units
	if hi > maxInt64/Scale || hi < minInt64/Scale {
		return 0, ErrOverflow
	}
	return Amount(units * Scale), nil
}

// MustFromInt e como FromInt mas entra em panico em erro. Use apenas em
// constantes de teste / seed onde o valor e conhecido.
func MustFromInt(units int64) Amount {
	a, err := FromInt(units)
	if err != nil {
		panic(err)
	}
	return a
}

// Parse converte uma string decimal ("1.5", "-0.00000001", "1000") em Amount.
// Aceita ate Decimals casas decimais; mais que isso e erro (sem arredondamento
// silencioso, para preservar determinismo e auditabilidade).
func Parse(s string) (Amount, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidFormat
	}

	neg := false
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, ErrInvalidFormat
	}

	intPart := s
	fracPart := ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart = s[:dot]
		fracPart = s[dot+1:]
	}

	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > Decimals {
		return 0, ErrTooManyDecimals
	}

	// Normaliza a parte fracionaria para exatamente Decimals digitos.
	fracPadded := fracPart + strings.Repeat("0", Decimals-len(fracPart))

	if !isDigits(intPart) || (fracPadded != "" && !isDigits(fracPadded)) {
		return 0, ErrInvalidFormat
	}

	intVal, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, ErrOverflow
	}
	fracVal := int64(0)
	if fracPadded != "" {
		fracVal, err = strconv.ParseInt(fracPadded, 10, 64)
		if err != nil {
			return 0, ErrOverflow
		}
	}

	// total = intVal*Scale + fracVal, com checagem de overflow.
	if intVal > maxInt64/Scale || intVal < minInt64/Scale {
		return 0, ErrOverflow
	}
	scaled := intVal * Scale
	total := scaled + fracVal
	if (scaled > 0 && total < scaled) || (scaled < 0 && total > scaled) {
		return 0, ErrOverflow
	}

	if neg {
		total = -total
	}
	return Amount(total), nil
}

// MustParse e como Parse mas entra em panico em erro. Use apenas em
// constantes de teste / seed.
func MustParse(s string) Amount {
	a, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return a
}

// String formata a quantia como decimal com Decimals casas, sem perda de
// precisao. Ex.: Amount(150000000) -> "1.50000000".
func (a Amount) String() string {
	neg := a < 0
	v := int64(a)
	if neg {
		v = -v
	}
	intPart := v / Scale
	fracPart := v % Scale

	frac := strconv.FormatInt(fracPart, 10)
	frac = strings.Repeat("0", Decimals-len(frac)) + frac

	sign := ""
	if neg {
		sign = "-"
	}
	return sign + strconv.FormatInt(intPart, 10) + "." + frac
}

// Float64 retorna uma aproximacao em float64 APENAS para exibicao/telemetria.
// NUNCA use o resultado em logica de saldo, preco ou matching.
func (a Amount) Float64() float64 {
	return float64(a) / float64(Scale)
}

// Add soma duas quantias com checagem de overflow.
func (a Amount) Add(b Amount) (Amount, error) {
	r := a + b
	if (b > 0 && r < a) || (b < 0 && r > a) {
		return 0, ErrOverflow
	}
	return r, nil
}

// Sub subtrai b de a com checagem de overflow.
func (a Amount) Sub(b Amount) (Amount, error) {
	r := a - b
	if (b < 0 && r < a) || (b > 0 && r > a) {
		return 0, ErrOverflow
	}
	return r, nil
}

// Neg retorna -a.
func (a Amount) Neg() Amount {
	return -a
}

// IsZero, IsPositive, IsNegative: predicados de sinal.
func (a Amount) IsZero() bool     { return a == 0 }
func (a Amount) IsPositive() bool { return a > 0 }
func (a Amount) IsNegative() bool { return a < 0 }

// Cmp compara a com b: -1 (a<b), 0 (a==b), 1 (a>b).
func (a Amount) Cmp(b Amount) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Notional calcula preco * quantidade, retornando uma quantia escalada por
// Scale. Ambos os operandos sao escalados por Scale, entao o produto bruto fica
// escalado por Scale^2 e precisa ser dividido por Scale. Usa math/big como
// intermediario para evitar overflow do produto antes do reescalonamento, e
// trunca para a menor unidade (sem arredondar).
//
// Ex.: price=2.00000000 (200000000), qty=1.50000000 (150000000)
//
//	-> notional 3.00000000 (300000000)
func Notional(price, qty Amount) (Amount, error) {
	bp := big.NewInt(int64(price))
	bq := big.NewInt(int64(qty))
	prod := new(big.Int).Mul(bp, bq)
	prod.Quo(prod, big.NewInt(Scale)) // trunca em direcao a zero

	if !prod.IsInt64() {
		return 0, ErrOverflow
	}
	return Amount(prod.Int64()), nil
}

// helpers internos -----------------------------------------------------------

const (
	maxInt64 = int64(^uint64(0) >> 1)
	minInt64 = -maxInt64 - 1
)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
