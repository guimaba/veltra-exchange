import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../state.dart';
import 'wallet.dart';
import 'dashboard.dart';
import 'send.dart';
import 'monitor.dart';
import 'dlq.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen();

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int _index = 0;

  static const _tabs = <_TabSpec>[
    _TabSpec('Carteira', Icons.account_balance_wallet_outlined, WalletScreen()),
    _TabSpec('Dashboard', Icons.timeline_outlined, DashboardScreen()),
    _TabSpec('Enviar', Icons.send_outlined, SendScreen()),
    _TabSpec('Monitor', Icons.list_alt_outlined, MonitorScreen()),
    _TabSpec('DLQ', Icons.report_outlined, DlqScreen()),
  ];

  @override
  Widget build(BuildContext context) {
    final tab = _tabs[_index];
    return Scaffold(
      appBar: AppBar(
        title: Row(
          children: [
            const Icon(Icons.hub_outlined),
            const SizedBox(width: 8),
            const Text('Blockchain + RabbitMQ'),
            const SizedBox(width: 16),
            Expanded(child: _StatusBar()),
          ],
        ),
      ),
      body: SafeArea(child: tab.body),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
        destinations: [
          for (final t in _tabs)
            NavigationDestination(icon: Icon(t.icon), label: t.label),
        ],
      ),
    );
  }
}

class _TabSpec {
  final String label;
  final IconData icon;
  final Widget body;
  const _TabSpec(this.label, this.icon, this.body);
}

class _StatusBar extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    return Wrap(
      spacing: 12,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        _Chip(
          label: state.wsConnected ? 'WS conectado' : 'WS offline',
          color: state.wsConnected ? Colors.greenAccent : Colors.redAccent,
        ),
        _Chip(
          label: state.leader == -1
              ? 'Sem lider'
              : 'Lider: No ${state.leader}',
          color: state.leader == -1 ? Colors.orangeAccent : Colors.lightBlueAccent,
        ),
      ],
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  final Color color;
  const _Chip({required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withOpacity(0.15),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: color.withOpacity(0.5)),
      ),
      child: Text(label, style: TextStyle(color: color, fontSize: 12)),
    );
  }
}
