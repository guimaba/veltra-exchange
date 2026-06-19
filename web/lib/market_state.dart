import 'dart:async';

import 'package:flutter/material.dart';

import 'api.dart';

class MarketCoin {
  final String symbol;
  final String name;
  final double priceUSD;
  final double priceBRL;
  final double change24h;
  final double volume24hUSD;
  final double marketCapUSD;

  const MarketCoin({
    required this.symbol,
    required this.name,
    required this.priceUSD,
    required this.priceBRL,
    required this.change24h,
    required this.volume24hUSD,
    required this.marketCapUSD,
  });

  factory MarketCoin.fromJson(Map<String, dynamic> j) => MarketCoin(
        symbol: j['symbol'] as String? ?? '',
        name: j['name'] as String? ?? '',
        priceUSD: (j['price_usd'] as num?)?.toDouble() ?? 0,
        priceBRL: (j['price_brl'] as num?)?.toDouble() ?? 0,
        change24h: (j['change_24h'] as num?)?.toDouble() ?? 0,
        volume24hUSD: (j['volume_24h_usd'] as num?)?.toDouble() ?? 0,
        marketCapUSD: (j['market_cap_usd'] as num?)?.toDouble() ?? 0,
      );

  bool get isUp => change24h >= 0;
}

class Candle {
  final int t; // unix seconds
  final double o, h, l, c, v;
  const Candle(
      {required this.t,
      required this.o,
      required this.h,
      required this.l,
      required this.c,
      required this.v});

  factory Candle.fromJson(Map<String, dynamic> j) => Candle(
        t: (j['t'] as num).toInt(),
        o: (j['o'] as num).toDouble(),
        h: (j['h'] as num).toDouble(),
        l: (j['l'] as num).toDouble(),
        c: (j['c'] as num).toDouble(),
        v: (j['v'] as num).toDouble(),
      );
}

class MarketState extends ChangeNotifier {
  final ApiClient api;
  final WsClient ws;

  MarketState({required this.api, required this.ws});

  List<MarketCoin> _coins = [];
  final Map<String, List<Candle>> _candles = {};
  int _updatedAt = 0;
  StreamSubscription? _sub;
  String _search = '';

  List<MarketCoin> get coins {
    final q = _search.toLowerCase();
    if (q.isEmpty) return List.unmodifiable(_coins);
    return _coins
        .where((c) =>
            c.symbol.toLowerCase().contains(q) ||
            c.name.toLowerCase().contains(q))
        .toList();
  }

  List<Candle> candlesFor(String symbol) =>
      List.unmodifiable(_candles[symbol] ?? []);

  int get updatedAt => _updatedAt;
  String get search => _search;

  void setSearch(String v) {
    _search = v;
    notifyListeners();
  }

  Future<void> bootstrap() async {
    // Carrega snapshot inicial via REST
    try {
      final data = await api.getMarket();
      _applyMarketData(data);
    } catch (_) {}

    // Escuta updates via WS
    _sub = ws.connect().listen(_onWsMessage, onError: (_) {});
  }

  void _onWsMessage(Map<String, dynamic> msg) {
    final type = msg['type'] as String?;
    if (type == 'market.update') {
      final data = msg['data'] as Map<String, dynamic>?;
      if (data != null) _applyMarketData(data);
    }
  }

  void _applyMarketData(Map<String, dynamic> data) {
    final rawCoins = data['coins'] as List<dynamic>?;
    if (rawCoins != null) {
      _coins = rawCoins
          .whereType<Map<String, dynamic>>()
          .map(MarketCoin.fromJson)
          .toList();
    }
    _updatedAt = (data['updated_at'] as num?)?.toInt() ?? 0;
    notifyListeners();
  }

  /// Busca candles para um símbolo via REST (chamado ao abrir o chart).
  Future<List<Candle>> loadCandles(String symbol) async {
    try {
      final raw = await api.getCandles(symbol);
      final list = (raw as List<dynamic>)
          .whereType<Map<String, dynamic>>()
          .map(Candle.fromJson)
          .toList();
      _candles[symbol] = list;
      notifyListeners();
      return list;
    } catch (_) {
      return [];
    }
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}
