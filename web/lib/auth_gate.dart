import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'auth_state.dart';
import 'balance_state.dart';
import 'screens/home.dart';
import 'screens/login.dart';
import 'screens/register.dart';
import 'trading_state.dart';

class AuthGate extends StatefulWidget {
  const AuthGate({super.key});
  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool _showRegister = false;
  String? _lastAccount;

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthState>();

    if (auth.isLoggedIn) {
      final username = auth.user!.username;
      if (username != _lastAccount) {
        _lastAccount = username;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          context.read<TradingState>().setAccount(username);
          // Carrega saldos reais do Postgres após login
          context.read<BalanceState>().bootstrap();
        });
      }
      return const HomeScreen();
    }

    if (_showRegister) {
      return RegisterScreen(
        onGoLogin: () {
          context.read<AuthState>().clearError();
          setState(() => _showRegister = false);
        },
      );
    }

    return LoginScreen(
      onGoRegister: () {
        context.read<AuthState>().clearError();
        setState(() => _showRegister = true);
      },
    );
  }
}
