import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';

/// Resolve a URL base do gateway. Quando rodando no proprio dominio
/// (mesma origem em que o Flutter Web foi servido), usa caminhos relativos.
String _baseUrl() {
  final host = Uri.base.host;
  final port = Uri.base.port;
  return 'http://$host:$port';
}

String _wsUrl() {
  final host = Uri.base.host;
  final port = Uri.base.port;
  return 'ws://$host:$port/ws';
}

class ApiClient {
  final http.Client _http = http.Client();
  String? _token;

  void setToken(String? token) {
    _token = token;
  }

  Map<String, String> get _authHeaders => {
        'Content-Type': 'application/json',
        if (_token != null) 'Authorization': 'Bearer $_token',
      };

  Future<Map<String, dynamic>> getState() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/state'));
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<double> getBalance(String account) async {
    final res = await _http.get(
      Uri.parse('${_baseUrl()}/api/accounts/$account/balance'),
    );
    _check(res);
    final body = jsonDecode(res.body) as Map<String, dynamic>;
    return (body['balance'] as num).toDouble();
  }

  Future<String> postCredit(String account, double amount) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/accounts/credit'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'account': account, 'amount': amount}),
    );
    _check(res);
    final body = jsonDecode(res.body) as Map<String, dynamic>;
    return body['tx_id'] as String;
  }

  Future<String> postTransaction(
    String sender,
    String receiver,
    double amount,
  ) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/transactions'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'sender': sender,
        'receiver': receiver,
        'amount': amount,
      }),
    );
    _check(res);
    final body = jsonDecode(res.body) as Map<String, dynamic>;
    return body['tx_id'] as String;
  }

  // ===== Balance + Admin =====

  Future<Map<String, dynamic>> getMyBalance() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/balance'),
        headers: _authHeaders);
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> deposit({
    required String amount,
    required String method,
    String currency = 'BRL',
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/deposit'),
      headers: _authHeaders,
      body: jsonEncode({'amount': amount, 'method': method, 'currency': currency}),
    );
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  /// Saque simulado: debita o ativo e devolve o valor equivalente na moeda fiat.
  Future<Map<String, dynamic>> withdraw({
    required String asset,
    required String amount,
    String currency = 'BRL',
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/withdraw'),
      headers: _authHeaders,
      body: jsonEncode({'asset': asset, 'amount': amount, 'currency': currency}),
    );
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getAdminUsers() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/admin/users'),
        headers: _authHeaders);
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> getAdminStats() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/admin/stats'),
        headers: _authHeaders);
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  /// Admin: ajusta o saldo de um usuário (amount com sinal; "-100" debita).
  Future<void> adminCredit({
    required String username,
    required String asset,
    required String amount,
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/admin/credit'),
      headers: _authHeaders,
      body: jsonEncode({'username': username, 'asset': asset, 'amount': amount}),
    );
    _check(res);
  }

  /// Admin: promove/rebaixa um usuário.
  Future<void> adminPromote({
    required String username,
    required bool isAdmin,
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/admin/promote'),
      headers: _authHeaders,
      body: jsonEncode({'username': username, 'is_admin': isAdmin}),
    );
    _check(res);
  }

  // ===== Market data =====

  Future<Map<String, dynamic>> getMarket() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/market'));
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<dynamic> getCandles(String symbol) async {
    final res = await _http.get(
        Uri.parse('${_baseUrl()}/api/market/${symbol.toUpperCase()}/candles'));
    _check(res);
    return jsonDecode(res.body);
  }

  // ===== Auth =====

  Future<Map<String, dynamic>> login(String username, String password) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/auth/login'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'username': username, 'password': password}),
    );
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> register(
      String username, String email, String password) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode(
          {'username': username, 'email': email, 'password': password}),
    );
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<void> logout(String token) async {
    await _http.post(
      Uri.parse('${_baseUrl()}/api/auth/logout'),
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer $token',
      },
    );
  }

  // ===== Veltra Exchange =====

  Future<Map<String, dynamic>> getVeltraState() async {
    final res = await _http.get(Uri.parse('${_baseUrl()}/api/veltra/state'));
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  /// Envia uma ordem. Preco e quantidade viajam como STRING decimal — o
  /// backend converte para inteiro escalado; o cliente nunca faz conta de
  /// dinheiro em float.
  Future<Map<String, dynamic>> placeOrder({
    required String account,
    required String pair,
    required String side,
    required String type,
    required String quantity,
    String price = '',
    String timeInForce = '',
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/orders'),
      headers: _authHeaders,
      body: jsonEncode({
        'account': account,
        'pair': pair,
        'side': side,
        'type': type,
        'price': price,
        'quantity': quantity,
        'time_in_force': timeInForce,
      }),
    );
    _check(res);
    return jsonDecode(res.body) as Map<String, dynamic>;
  }

  Future<void> cancelOrder({
    required String orderId,
    required String account,
    required String pair,
  }) async {
    final req = http.Request(
      'DELETE',
      Uri.parse('${_baseUrl()}/api/orders/$orderId'),
    )
      ..headers.addAll(_authHeaders)
      ..body = jsonEncode({'account': account, 'pair': pair});
    final streamed = await _http.send(req);
    final res = await http.Response.fromStream(streamed);
    _check(res);
  }

  Future<void> faucet({
    required String account,
    required String asset,
    required String amount,
  }) async {
    final res = await _http.post(
      Uri.parse('${_baseUrl()}/api/faucet'),
      headers: _authHeaders,
      body: jsonEncode({'account': account, 'asset': asset, 'amount': amount}),
    );
    _check(res);
  }

  void _check(http.Response res) {
    if (res.statusCode >= 400) {
      String msg = res.body;
      try {
        final body = jsonDecode(res.body) as Map<String, dynamic>;
        if (body['error'] is String) msg = body['error'] as String;
      } catch (_) {}
      throw ApiException(res.statusCode, msg);
    }
  }
}

class ApiException implements Exception {
  final int status;
  final String message;
  ApiException(this.status, this.message);
  @override
  String toString() => 'API $status: $message';
}

/// Wrapper sobre WebSocketChannel para tipar mensagens recebidas.
class WsClient {
  WebSocketChannel? _channel;
  Stream<Map<String, dynamic>>? _stream;

  Stream<Map<String, dynamic>> connect() {
    if (_stream != null) return _stream!;
    _channel = WebSocketChannel.connect(Uri.parse(_wsUrl()));
    _stream = _channel!.stream
        .map((raw) => raw is String ? raw : raw.toString())
        .map((s) => jsonDecode(s) as Map<String, dynamic>)
        .asBroadcastStream();
    return _stream!;
  }

  void close() {
    _channel?.sink.close();
    _channel = null;
    _stream = null;
  }
}
