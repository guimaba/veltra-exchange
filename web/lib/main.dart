import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'api.dart';
import 'state.dart';
import 'theme.dart';
import 'screens/home.dart';

void main() {
  runApp(const BlockchainApp());
}

class BlockchainApp extends StatelessWidget {
  const BlockchainApp();

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider<AppState>(
      create: (_) => AppState(api: ApiClient(), ws: WsClient())..bootstrap(),
      child: MaterialApp(
        title: 'Blockchain + RabbitMQ',
        debugShowCheckedModeBanner: false,
        theme: appTheme(),
        home: const HomeScreen(),
      ),
    );
  }
}
