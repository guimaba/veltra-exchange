import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'api.dart';
import 'auth_gate.dart';
import 'auth_state.dart';
import 'balance_state.dart';
import 'market_state.dart';
import 'state.dart';
import 'theme.dart';
import 'trading_state.dart';

void main() {
  runApp(const VeltraApp());
}

class VeltraApp extends StatelessWidget {
  const VeltraApp();

  @override
  Widget build(BuildContext context) {
    final api = ApiClient();
    final ws = WsClient();

    return MultiProvider(
      providers: [
        // ApiClient exposto para uso direto em widgets (admin, deposit, etc.)
        Provider<ApiClient>(create: (_) => api),
        ChangeNotifierProvider<AuthState>(
          create: (_) => AuthState(api: api),
        ),
        ChangeNotifierProvider<AppState>(
          create: (_) => AppState(api: api, ws: ws)..bootstrap(),
        ),
        ChangeNotifierProvider<TradingState>(
          create: (_) => TradingState(api: api, ws: ws)..bootstrap(),
        ),
        ChangeNotifierProvider<MarketState>(
          create: (_) => MarketState(api: api, ws: ws)..bootstrap(),
        ),
        ChangeNotifierProvider<BalanceState>(
          create: (_) => BalanceState(api: api, ws: ws),
        ),
      ],
      child: MaterialApp(
        title: 'Veltra Exchange',
        debugShowCheckedModeBanner: false,
        theme: appTheme(),
        home: const AuthGate(),
      ),
    );
  }
}
