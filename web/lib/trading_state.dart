import 'dart:async';
import 'package:flutter/foundation.dart';
import 'api.dart';

/// Escala monetaria do backend (pkg/money: int64 com 8 casas decimais).
/// Em Dart Web os inteiros sao doubles JS (seguros ate 2^53), o que cobre com
/// folga os valores da simulacao. A conversao para double e SO para exibicao.
const int moneyScale = 100000000; // 1e8

/// Formata um valor escalado como decimal legivel, aparando zeros a direita.
String fmtAmount(num scaled, {int minDecimals = 2, int maxDecimals = 8}) {
  final v = scaled / moneyScale;
  var s = v.toStringAsFixed(maxDecimals);
  // Apara zeros mantendo pelo menos minDecimals casas.
  while (s.contains('.') &&
      s.endsWith('0') &&
      s.split('.').last.length > minDecimals) {
    s = s.substring(0, s.length - 1);
  }
  if (s.endsWith('.')) s = s.substring(0, s.length - 1);
  return s;
}

/// Um nivel L2 do order book.
class BookLevel {
  final int price;
  final int quantity;
  const BookLevel(this.price, this.quantity);

  factory BookLevel.fromJson(Map<String, dynamic> j) => BookLevel(
        (j['price'] as num).toInt(),
        (j['quantity'] as num).toInt(),
      );
}

/// Trade na fita.
class TradeTick {
  final String tradeId;
  final String pair;
  final int price;
  final int quantity;
  final String takerSide; // buy | sell
  final int timestampMs;

  const TradeTick({
    required this.tradeId,
    required this.pair,
    required this.price,
    required this.quantity,
    required this.takerSide,
    required this.timestampMs,
  });

  factory TradeTick.fromJson(Map<String, dynamic> j) => TradeTick(
        tradeId: j['trade_id'] as String? ?? '',
        pair: j['pair'] as String? ?? '',
        price: (j['price'] as num? ?? 0).toInt(),
        quantity: (j['quantity'] as num? ?? 0).toInt(),
        takerSide: j['taker_side'] as String? ?? '',
        timestampMs: (j['timestamp_ms'] as num? ?? 0).toInt(),
      );
}

/// Estado corrente de uma ordem.
class OrderInfo {
  final String orderId;
  final String clientOrderId;
  final String account;
  final String pair;
  final String side;
  final String type;
  final int price;
  final int quantity;
  int filled;
  String status;
  final int sequence;

  OrderInfo({
    required this.orderId,
    required this.clientOrderId,
    required this.account,
    required this.pair,
    required this.side,
    required this.type,
    required this.price,
    required this.quantity,
    required this.filled,
    required this.status,
    required this.sequence,
  });

  factory OrderInfo.fromJson(Map<String, dynamic> j) => OrderInfo(
        orderId: j['order_id'] as String? ?? '',
        clientOrderId: j['client_order_id'] as String? ?? '',
        account: j['account'] as String? ?? '',
        pair: j['pair'] as String? ?? '',
        side: j['side'] as String? ?? '',
        type: j['type'] as String? ?? '',
        price: (j['price'] as num? ?? 0).toInt(),
        quantity: (j['quantity'] as num? ?? 0).toInt(),
        filled: (j['filled'] as num? ?? 0).toInt(),
        status: j['status'] as String? ?? '',
        sequence: (j['sequence'] as num? ?? 0).toInt(),
      );

  bool get isOpen => status == 'new' || status == 'partially_filled';
  int get remaining => quantity - filled;
}

/// TradingState e o repositorio reativo da Veltra Exchange na UI. Consome o
/// mesmo WebSocket do gateway (stream broadcast compartilhado com o AppState)
/// e expoe book, fita de trades, ordens e saldos como projecoes locais.
class TradingState extends ChangeNotifier {
  final ApiClient api;
  final WsClient ws;

  TradingState({required this.api, required this.ws});

  static const String defaultPair = 'VLT/USDT-sim';

  String _pair = defaultPair;
  String _account = 'alice';

  List<BookLevel> _bids = const [];
  List<BookLevel> _asks = const [];
  int _bookSequence = 0;
  final List<TradeTick> _trades = [];
  final Map<String, OrderInfo> _orders = {};
  final Map<String, Map<String, num>> _balances = {};
  int _lastPrice = 0;
  int _prevPrice = 0;
  String? _lastError;
  String? _lastNotice;

  StreamSubscription? _sub;

  // ----- getters -----
  String get pair => _pair;
  String get account => _account;
  List<BookLevel> get bids => _bids;
  List<BookLevel> get asks => _asks;
  List<TradeTick> get trades => List.unmodifiable(_trades);
  int get lastPrice => _lastPrice;

  /// 1 = subiu, -1 = caiu, 0 = neutro (cor do last price).
  int get priceDirection =>
      _lastPrice == _prevPrice ? 0 : (_lastPrice > _prevPrice ? 1 : -1);

  int? get bestBid => _bids.isEmpty ? null : _bids.first.price;
  int? get bestAsk => _asks.isEmpty ? null : _asks.first.price;
  int? get spread => (bestBid != null && bestAsk != null) ? bestAsk! - bestBid! : null;

  String? get lastError => _lastError;
  String? get lastNotice => _lastNotice;

  String get baseAsset => _pair.split('/').first;
  String get quoteAsset => _pair.split('/').last;

  /// Ordens abertas da conta selecionada (mais recentes primeiro).
  List<OrderInfo> get openOrders {
    final list = _orders.values
        .where((o) => o.account == _account && o.isOpen)
        .toList()
      ..sort((a, b) => b.sequence.compareTo(a.sequence));
    return list;
  }

  /// Historico (finalizadas) da conta selecionada.
  List<OrderInfo> get orderHistory {
    final list = _orders.values
        .where((o) => o.account == _account && !o.isOpen)
        .toList()
      ..sort((a, b) => b.sequence.compareTo(a.sequence));
    return list;
  }

  num balanceOf(String asset) => _balances[_account]?[asset] ?? 0;

  Map<String, Map<String, num>> get allBalances =>
      Map.unmodifiable(_balances);

  // ----- bootstrap / ws -----

  Future<void> bootstrap() async {
    try {
      _applySnapshot(await api.getVeltraState());
    } catch (e) {
      _lastError = 'Falha ao buscar estado da exchange: $e';
    }
    _sub?.cancel();
    _sub = ws.connect().listen(_onWsMessage, onError: (_) {});
    notifyListeners();
  }

  void setAccount(String account) {
    if (account.isEmpty || account == _account) return;
    _account = account;
    notifyListeners();
  }

  void _onWsMessage(Map<String, dynamic> msg) {
    final type = msg['type'] as String?;
    final data = (msg['data'] is Map<String, dynamic>)
        ? msg['data'] as Map<String, dynamic>
        : <String, dynamic>{};

    switch (type) {
      case 'veltra_snapshot':
        _applySnapshot(data);
        break;
      case 'book.updated':
        _applyBook(data);
        break;
      case 'trade.executed':
        _applyTrade(data);
        break;
      case 'order.accepted':
        final o = OrderInfo.fromJson(data)..status = 'new';
        _orders[o.orderId] = o;
        break;
      case 'order.filled':
        final id = data['order_id'] as String? ?? '';
        final o = _orders[id];
        if (o != null) {
          o.filled = (data['cumulative_filled'] as num? ?? o.filled).toInt();
          o.status = data['status'] as String? ?? o.status;
        }
        break;
      case 'order.canceled':
        final id = data['order_id'] as String? ?? '';
        _orders[id]?.status = 'canceled';
        break;
      case 'order.rejected':
        final reason = data['reason'] as String? ?? 'desconhecido';
        final acc = data['account'] as String? ?? '';
        if (acc == _account) {
          _lastError = 'Ordem rejeitada: $reason';
        }
        break;
      case 'faucet.credit':
        final acc = data['account'] as String? ?? '';
        final asset = data['asset'] as String? ?? '';
        final amount = (data['amount'] as num? ?? 0);
        if (acc.isNotEmpty && asset.isNotEmpty) {
          _balances.putIfAbsent(acc, () => {});
          _balances[acc]![asset] = (_balances[acc]![asset] ?? 0) + amount;
        }
        break;
      default:
        return; // evento de outro dominio (blockchain) — ignora
    }
    notifyListeners();
  }

  void _applySnapshot(Map<String, dynamic> snap) {
    final books = snap['books'];
    if (books is Map && books[_pair] is Map) {
      _applyBook((books[_pair] as Map).cast<String, dynamic>());
    }
    final trades = snap['trades'];
    if (trades is Map && trades[_pair] is List) {
      _trades
        ..clear()
        ..addAll((trades[_pair] as List)
            .cast<Map<String, dynamic>>()
            .map(TradeTick.fromJson));
    }
    final orders = snap['orders'];
    if (orders is List) {
      for (final j in orders.cast<Map<String, dynamic>>()) {
        final o = OrderInfo.fromJson(j);
        _orders[o.orderId] = o;
      }
    }
    final balances = snap['balances'];
    if (balances is Map) {
      _balances.clear();
      balances.forEach((acc, assets) {
        if (assets is Map) {
          _balances[acc.toString()] = assets
              .map((k, v) => MapEntry(k.toString(), (v as num? ?? 0)));
        }
      });
    }
    final lastPx = snap['last_price'];
    if (lastPx is Map && lastPx[_pair] is num) {
      _lastPrice = (lastPx[_pair] as num).toInt();
      _prevPrice = _lastPrice;
    }
  }

  void _applyBook(Map<String, dynamic> data) {
    if ((data['pair'] as String? ?? '') != _pair) return;
    final seq = (data['sequence'] as num? ?? 0).toInt();
    if (seq < _bookSequence) return; // update atrasado
    _bookSequence = seq;
    _bids = ((data['bids'] as List?) ?? const [])
        .cast<Map<String, dynamic>>()
        .map(BookLevel.fromJson)
        .toList();
    _asks = ((data['asks'] as List?) ?? const [])
        .cast<Map<String, dynamic>>()
        .map(BookLevel.fromJson)
        .toList();
  }

  void _applyTrade(Map<String, dynamic> data) {
    final t = TradeTick.fromJson(data);
    if (t.pair != _pair) return;
    _prevPrice = _lastPrice == 0 ? t.price : _lastPrice;
    _lastPrice = t.price;
    _trades.insert(0, t);
    if (_trades.length > 100) _trades.removeLast();

    // Atualiza a projecao local de saldos das duas pontas.
    final notionalScaled =
        (t.price / moneyScale) * t.quantity; // exibicao apenas
    final buyer = data[t.takerSide == 'buy' ? 'taker_account' : 'maker_account']
            as String? ??
        '';
    final seller =
        data[t.takerSide == 'buy' ? 'maker_account' : 'taker_account']
                as String? ??
            '';
    void move(String acc, String asset, num delta) {
      if (acc.isEmpty) return;
      _balances.putIfAbsent(acc, () => {});
      _balances[acc]![asset] = (_balances[acc]![asset] ?? 0) + delta;
    }

    move(buyer, baseAsset, t.quantity);
    move(buyer, quoteAsset, -notionalScaled);
    move(seller, baseAsset, -t.quantity);
    move(seller, quoteAsset, notionalScaled);
  }

  // ----- acoes -----

  Future<bool> placeOrder({
    required String side,
    required String type,
    required String quantity,
    String pair = '',
    String price = '',
    String timeInForce = '',
  }) async {
    try {
      await api.placeOrder(
        account: _account,
        pair: pair.isEmpty ? _pair : pair,
        side: side,
        type: type,
        quantity: quantity,
        price: price,
        timeInForce: timeInForce,
      );
      _lastNotice = 'Ordem enviada';
      notifyListeners();
      return true;
    } catch (e) {
      _lastError = e.toString();
      notifyListeners();
      return false;
    }
  }

  Future<void> cancelOrder(OrderInfo o) async {
    try {
      await api.cancelOrder(orderId: o.orderId, account: _account, pair: _pair);
    } catch (e) {
      _lastError = e.toString();
      notifyListeners();
    }
  }

  Future<bool> requestFaucet(String asset, String amount) async {
    try {
      await api.faucet(account: _account, asset: asset, amount: amount);
      _lastNotice = 'Crédito solicitado';
      notifyListeners();
      return true;
    } catch (e) {
      _lastError = e.toString();
      notifyListeners();
      return false;
    }
  }

  void clearMessages() {
    _lastError = null;
    _lastNotice = null;
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }
}
