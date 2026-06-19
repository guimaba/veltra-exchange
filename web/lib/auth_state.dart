import 'package:flutter/material.dart';
import 'api.dart';

class AuthUser {
  final int id;
  final String username;
  final String email;
  final bool isAdmin;
  final String token;

  const AuthUser({
    required this.id,
    required this.username,
    required this.email,
    required this.isAdmin,
    required this.token,
  });

  factory AuthUser.fromJson(Map<String, dynamic> j, String token) => AuthUser(
        id: (j['user_id'] ?? j['id'] ?? 0) as int,
        username: j['username'] as String,
        email: (j['email'] ?? '') as String,
        isAdmin: (j['is_admin'] ?? false) as bool,
        token: token,
      );
}

class AuthState extends ChangeNotifier {
  final ApiClient api;

  AuthState({required this.api});

  AuthUser? _user;
  bool _loading = false;
  String? _error;

  AuthUser? get user => _user;
  bool get isLoggedIn => _user != null;
  bool get loading => _loading;
  String? get error => _error;
  String get account => _user?.username ?? '';

  Future<bool> login(String username, String password) async {
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final body = await api.login(username, password);
      _user = AuthUser.fromJson(body, body['token'] as String);
      api.setToken(_user!.token);
      _loading = false;
      notifyListeners();
      return true;
    } on ApiException catch (e) {
      _error = e.message;
      _loading = false;
      notifyListeners();
      return false;
    } catch (e) {
      _error = e.toString();
      _loading = false;
      notifyListeners();
      return false;
    }
  }

  Future<bool> register(String username, String email, String password) async {
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final body = await api.register(username, email, password);
      _user = AuthUser.fromJson(body, body['token'] as String);
      api.setToken(_user!.token);
      _loading = false;
      notifyListeners();
      return true;
    } on ApiException catch (e) {
      _error = e.message;
      _loading = false;
      notifyListeners();
      return false;
    } catch (e) {
      _error = e.toString();
      _loading = false;
      notifyListeners();
      return false;
    }
  }

  void logout() {
    if (_user != null) {
      api.logout(_user!.token).catchError((_) {});
    }
    _user = null;
    _error = null;
    api.setToken(null);
    notifyListeners();
  }

  void clearError() {
    _error = null;
    notifyListeners();
  }
}
