import 'dart:async';

import 'package:flutter/material.dart';

import 'api.dart';

class AssetBalance {
  final String asset;
  final int balance;   // scaled (1e8)
  final int reserved;
  final int available;

  const AssetBalance({
    required this.asset,
    required this.balance,
    required this.reserved,
    required this.available,
  });

  factory AssetBalance.fromJson(Map<String, dynamic> j) => AssetBalance(
        asset: j['asset'] as String,
        balance: (j['balance'] as num).toInt(),
        reserved: (j['reserved'] as num).toInt(),
        available: (j['available'] as num).toInt(),
      );

  double get balanceDecimal => balance / 1e8;
  double get availableDecimal => available / 1e8;
}

/// Estado de saldo vindo do Postgres (fonte de verdade).
/// Atualiza via REST no login, no faucet/depósito e em eventos WS (trade/faucet).
class BalanceState extends ChangeNotifier {
  final ApiClient api;
  final WsClient ws;

  BalanceState({required this.api, required this.ws});

  List<AssetBalance> _balances = [];
  bool _loading = false;
  StreamSubscription? _sub;

  List<AssetBalance> get balances => List.unmodifiable(_balances);

  bool get loading => _loading;

  double balanceOf(String asset) {
    final found = _balances.where((b) => b.asset == asset).firstOrNull;
    return found?.balanceDecimal ?? 0;
  }

  double availableOf(String asset) {
    final found = _balances.where((b) => b.asset == asset).firstOrNull;
    return found?.availableDecimal ?? 0;
  }

  // Total em USD estimado (usa preços do MarketState via callback, se disponível)
  List<AssetBalance> get nonZero =>
      _balances.where((b) => b.balance > 0).toList();

  Future<void> bootstrap() async {
    await refresh();
    _sub = ws.connect().listen(_onWsMessage, onError: (_) {});
  }

  Future<void> refresh() async {
    _loading = true;
    notifyListeners();
    try {
      final data = await api.getMyBalance();
      final raw = (data['balances'] as List<dynamic>?) ?? [];
      _balances = raw
          .whereType<Map<String, dynamic>>()
          .map(AssetBalance.fromJson)
          .toList();
    } catch (_) {}
    _loading = false;
    notifyListeners();
  }

  void _onWsMessage(Map<String, dynamic> msg) {
    final type = msg['type'] as String?;
    // Atualiza saldo após trade ou faucet
    if (type == 'trade.executed' || type == 'faucet.credit' ||
        type == 'order.filled' || type == 'ledger.posted') {
      // Delay pequeno para dar tempo ao ledger de persistir
      Future.delayed(const Duration(milliseconds: 800), refresh);
    }
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}
