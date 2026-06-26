import 'package:intl/intl.dart';

/// Formatação numérica padronizada da Veltra Exchange (pt-BR).
///
/// Regra geral pedida: nunca exibir "valores infinitos" com muitos decimais —
/// todo número é limitado a um número sensato de casas (2 para dinheiro,
/// adaptativo e bounded para preço/cripto), com separador de milhar "." e
/// decimal ",".
class Fmt {
  Fmt._();

  static final NumberFormat _money = NumberFormat('#,##0.00', 'pt_BR');
  static final NumberFormat _int = NumberFormat('#,##0', 'pt_BR');
  static final NumberFormat _d4 = NumberFormat('#,##0.####', 'pt_BR');
  static final NumberFormat _d6 = NumberFormat('#,##0.######', 'pt_BR');

  /// Dinheiro: sempre 2 casas, com milhar. Ex.: 94.041,40
  static String money(num v) => _money.format(v);

  /// Reais. Ex.: R$ 1.234,56
  static String brl(num v) => 'R\$ ${_money.format(v)}';

  /// Dólar. Ex.: $ 1.234,56 (mesma regra de 2 casas; micro-preços ver [price]).
  static String usd(num v) => '\$ ${price(v)}';

  /// Preço adaptativo, SEMPRE limitado (nunca exponencial):
  ///   >= 1     → 2 casas        (59.585,97)
  ///   0,01–1   → 4 casas        (0,4204)
  ///   < 0,01   → 6 casas        (0,000012)
  static String price(num v) {
    final a = v.abs();
    if (a >= 1 || a == 0) return _money.format(v);
    if (a >= 0.01) return _d4.format(v);
    return _d6.format(v);
  }

  /// Quantidade de ativo: até 4 casas, sem zeros à direita inúteis, com milhar.
  /// Ex.: 0,1 · 1.234,5 · 12
  static String qty(num v) => _d4.format(v);

  /// Quantidade com mais precisão (cripto fracionária): até 6 casas.
  static String qty6(num v) => _d6.format(v);

  /// Percentual com sinal e 2 casas. Ex.: +2,34% · -1,10%
  static String pct(num v) => '${v >= 0 ? '+' : ''}${_money.format(v)}%';

  /// Compacto para volume / market cap. Ex.: 1,2 mi · 3,4 bi
  static String compact(num v) {
    final a = v.abs();
    if (a >= 1e12) return '${_d4.format(v / 1e12)} tri';
    if (a >= 1e9) return '${_d4.format(v / 1e9)} bi';
    if (a >= 1e6) return '${_d4.format(v / 1e6)} mi';
    if (a >= 1e3) return '${_d4.format(v / 1e3)} mil';
    return _int.format(v);
  }

  /// Compacto em dólar. Ex.: $ 1,2 bi
  static String compactUsd(num v) => '\$ ${compact(v)}';

  /// Inteiro com milhar. Ex.: 1.234
  static String integer(num v) => _int.format(v);

  // ---- Rótulos "amigáveis" (suaviza o jargão de simulação) ----

  /// Remove o sufixo "-sim" para exibição. Ex.: "USDT-sim" → "USDT".
  static String asset(String a) => a.replaceAll('-sim', '');

  /// Par sem "-sim". Ex.: "VLT/USDT-sim" → "VLT/USDT".
  static String pair(String p) => p.replaceAll('-sim', '');
}

/// Moedas fiat suportadas no depósito/saque (espelha o backend).
/// `rate` = unidades da moeda por 1 USD (USDT ≡ USD).
class FiatCurrency {
  final String code;
  final String symbol;
  final double rate;
  const FiatCurrency(this.code, this.symbol, this.rate);
}

const kFiatCurrencies = <FiatCurrency>[
  FiatCurrency('BRL', 'R\$', 5.20),
  FiatCurrency('USD', '\$', 1.00),
  FiatCurrency('EUR', '€', 0.92),
];

FiatCurrency fiatByCode(String code) =>
    kFiatCurrencies.firstWhere((c) => c.code == code,
        orElse: () => kFiatCurrencies.first);
