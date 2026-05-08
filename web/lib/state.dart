import 'dart:async';
import 'package:flutter/foundation.dart';
import 'api.dart';

/// Modelo simples de bloco para a UI.
class BlockSummary {
  final int index;
  final String hash;
  final String prevHash;
  final int nonce;
  final List<Map<String, dynamic>> transactions;
  final int minerNodeId;
  final DateTime receivedAt;

  BlockSummary({
    required this.index,
    required this.hash,
    required this.prevHash,
    required this.nonce,
    required this.transactions,
    required this.minerNodeId,
    required this.receivedAt,
  });

  factory BlockSummary.fromJson(Map<String, dynamic> j) => BlockSummary(
        index: j['index'] as int,
        hash: j['hash'] as String,
        prevHash: j['previous_hash'] as String,
        nonce: j['nonce'] as int? ?? 0,
        transactions: ((j['transactions'] as List?) ?? const [])
            .cast<Map<String, dynamic>>(),
        minerNodeId: j['miner_node_id'] as int? ?? 0,
        receivedAt: DateTime.now(),
      );
}

/// Item de monitoramento bruto (alimenta a tela "Monitor").
class EventLogEntry {
  final DateTime when;
  final String type;
  final Map<String, dynamic> data;

  EventLogEntry({required this.when, required this.type, required this.data});
}

/// AppState e o repositorio reativo da aplicacao. Conecta no WebSocket do
/// gateway e atualiza saldos/blocos/lider conforme eventos chegam.
class AppState extends ChangeNotifier {
  final ApiClient api;
  final WsClient ws;

  AppState({required this.api, required this.ws});

  final Map<String, double> _balances = {};
  final List<BlockSummary> _blocks = [];
  final List<EventLogEntry> _eventLog = [];
  int _leader = -1;
  bool _wsConnected = false;
  String? _lastError;

  StreamSubscription? _sub;

  Map<String, double> get balances => Map.unmodifiable(_balances);
  List<BlockSummary> get blocks => List.unmodifiable(_blocks);
  List<EventLogEntry> get eventLog => List.unmodifiable(_eventLog);
  int get leader => _leader;
  bool get wsConnected => _wsConnected;
  String? get lastError => _lastError;

  Future<void> bootstrap() async {
    try {
      final state = await api.getState();
      _applySnapshot(state);
    } catch (e) {
      _lastError = 'Falha ao buscar estado inicial: $e';
    }
    _connectWs();
    notifyListeners();
  }

  void _connectWs() {
    _sub?.cancel();
    final stream = ws.connect();
    _wsConnected = true;
    _sub = stream.listen(
      _onWsMessage,
      onError: (e) {
        _wsConnected = false;
        _lastError = 'WebSocket erro: $e';
        notifyListeners();
      },
      onDone: () {
        _wsConnected = false;
        notifyListeners();
      },
    );
  }

  void _onWsMessage(Map<String, dynamic> msg) {
    final type = msg['type'] as String?;
    final data = (msg['data'] is Map<String, dynamic>)
        ? msg['data'] as Map<String, dynamic>
        : <String, dynamic>{};

    if (type == 'snapshot') {
      _applySnapshot(data);
    } else if (type != null) {
      _eventLog.insert(0, EventLogEntry(when: DateTime.now(), type: type, data: data));
      if (_eventLog.length > 200) {
        _eventLog.removeLast();
      }
      _applyEvent(type, data);
    }
    notifyListeners();
  }

  void _applySnapshot(Map<String, dynamic> snap) {
    final bals = (snap['balances'] as Map?) ?? const {};
    _balances.clear();
    bals.forEach((k, v) {
      _balances[k.toString()] = (v as num).toDouble();
    });
    final blks = (snap['recent_blocks'] as List?) ?? const [];
    _blocks
      ..clear()
      ..addAll(blks
          .cast<Map<String, dynamic>>()
          .map(BlockSummary.fromJson));
    if (snap['leader'] is num) {
      _leader = (snap['leader'] as num).toInt();
    }
  }

  void _applyEvent(String routingKey, Map<String, dynamic> data) {
    switch (routingKey) {
      case 'credit.added':
        if (data['account'] is String && data['new_balance'] is num) {
          _balances[data['account'] as String] =
              (data['new_balance'] as num).toDouble();
        }
        break;
      case 'transaction.received':
        final after = data['balance_after'];
        if (after is Map) {
          after.forEach((k, v) {
            _balances[k.toString()] = (v as num).toDouble();
          });
        }
        break;
      case 'block.mined':
        try {
          final blk = BlockSummary.fromJson(data);
          if (!_blocks.any((b) => b.hash == blk.hash)) {
            _blocks.add(blk);
            if (_blocks.length > 50) _blocks.removeAt(0);
          }
        } catch (_) {}
        break;
      case 'leader.changed':
        if (data['new_leader'] is num) {
          _leader = (data['new_leader'] as num).toInt();
        }
        break;
    }
  }

  void clearError() {
    _lastError = null;
    notifyListeners();
  }

  @override
  void dispose() {
    _sub?.cancel();
    ws.close();
    super.dispose();
  }
}
