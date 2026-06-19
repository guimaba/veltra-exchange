import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../auth_state.dart';
import '../state.dart';
import '../theme.dart';
import 'admin.dart';
import 'market.dart';
import 'trade.dart';
import 'wallet.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen();

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int _index = 0;

  List<_TabSpec> _tabs(bool isAdmin) => [
        const _TabSpec('Trading', Icons.candlestick_chart_outlined, TradeScreen()),
        const _TabSpec('Mercado', Icons.bar_chart_outlined, MarketScreen()),
        const _TabSpec('Carteira', Icons.account_balance_wallet_outlined, WalletScreen()),
        if (isAdmin)
          const _TabSpec('Admin', Icons.admin_panel_settings_outlined, AdminScreen()),
      ];

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthState>();
    final isAdmin = auth.user?.isAdmin ?? false;
    final tabs = _tabs(isAdmin);
    final safeIndex = _index.clamp(0, tabs.length - 1);

    return Scaffold(
      appBar: _VeltraAppBar(),
      body: SafeArea(child: tabs[safeIndex].body),
      bottomNavigationBar: _VeltraNavBar(
        tabs: tabs,
        selectedIndex: safeIndex,
        onTap: (i) => setState(() => _index = i),
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

// ─── App bar ──────────────────────────────────────────────────────────────────

class _VeltraAppBar extends StatelessWidget implements PreferredSizeWidget {
  @override
  Size get preferredSize => const Size.fromHeight(60);

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();

    return Container(
      height: 60,
      decoration: BoxDecoration(
        color: kSurface,
        border: const Border(bottom: BorderSide(color: kBorder)),
        boxShadow: [
          BoxShadow(color: kBrand.withOpacity(0.04), blurRadius: 12, offset: const Offset(0, 2)),
        ],
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Row(children: [
          // Logo
          ShaderMask(
            shaderCallback: (bounds) => const LinearGradient(
              colors: [kBrand2, kBrand],
            ).createShader(bounds),
            child: const Row(mainAxisSize: MainAxisSize.min, children: [
              Icon(Icons.hub_outlined, color: Colors.white, size: 22),
              SizedBox(width: 8),
              Text('VELTRA',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 17,
                      fontWeight: FontWeight.w900,
                      letterSpacing: 3)),
            ]),
          ),
          const SizedBox(width: 16),

          // Status chips
          _WsChip(connected: state.wsConnected),
          if (state.leader != -1) ...[
            const SizedBox(width: 8),
            _LeaderChip(node: state.leader),
          ],
          const Spacer(),

          // User menu
          const _UserMenu(),
        ]),
      ),
    );
  }
}

class _WsChip extends StatelessWidget {
  final bool connected;
  const _WsChip({required this.connected});

  @override
  Widget build(BuildContext context) {
    final c = connected ? kBuy : kSell;
    return Row(mainAxisSize: MainAxisSize.min, children: [
      Container(
        width: 7,
        height: 7,
        decoration: BoxDecoration(
          color: c,
          shape: BoxShape.circle,
          boxShadow: [BoxShadow(color: c.withOpacity(0.7), blurRadius: 6)],
        ),
      ),
      const SizedBox(width: 5),
      Text(connected ? 'live' : 'offline',
          style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w600)),
    ]);
  }
}

class _LeaderChip extends StatelessWidget {
  final int node;
  const _LeaderChip({required this.node});

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
        decoration: BoxDecoration(
          color: kBrand.withOpacity(0.08),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: kBrand.withOpacity(0.25)),
        ),
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          const Icon(Icons.bolt, size: 11, color: kBrand),
          const SizedBox(width: 3),
          Text('Nó $node', style: const TextStyle(fontSize: 11, color: kBrand)),
        ]),
      );
}

// ─── Navigation bar ───────────────────────────────────────────────────────────

class _VeltraNavBar extends StatelessWidget {
  final List<_TabSpec> tabs;
  final int selectedIndex;
  final void Function(int) onTap;
  const _VeltraNavBar({required this.tabs, required this.selectedIndex, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 64,
      decoration: const BoxDecoration(
        color: kSurface,
        border: Border(top: BorderSide(color: kBorder)),
      ),
      child: Row(
        children: List.generate(tabs.length, (i) {
          final t = tabs[i];
          final selected = i == selectedIndex;
          final c = selected ? kBrand : kTxtMuted;
          return Expanded(
            child: InkWell(
              onTap: () => onTap(i),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  if (selected)
                    Container(
                      width: 28,
                      height: 2,
                      margin: const EdgeInsets.only(bottom: 4),
                      decoration: BoxDecoration(
                        color: kBrand,
                        borderRadius: BorderRadius.circular(1),
                        boxShadow: [
                          BoxShadow(color: kBrand.withOpacity(0.6), blurRadius: 8),
                        ],
                      ),
                    )
                  else
                    const SizedBox(height: 6),
                  Icon(t.icon, size: 20, color: c),
                  const SizedBox(height: 3),
                  Text(t.label,
                      style: TextStyle(
                          fontSize: 10,
                          color: c,
                          fontWeight: selected ? FontWeight.w700 : FontWeight.normal)),
                ],
              ),
            ),
          );
        }),
      ),
    );
  }
}

// ─── User menu ────────────────────────────────────────────────────────────────

class _UserMenu extends StatelessWidget {
  const _UserMenu();

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthState>();
    final user = auth.user;
    if (user == null) return const SizedBox.shrink();

    final initials = user.username.isNotEmpty ? user.username[0].toUpperCase() : '?';

    return PopupMenuButton<String>(
      tooltip: user.username,
      offset: const Offset(0, 50),
      onSelected: (v) { if (v == 'logout') auth.logout(); },
      itemBuilder: (_) => [
        PopupMenuItem(
          enabled: false,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Text(user.username,
                  style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 14, color: kTxt)),
              if (user.isAdmin) ...[
                const SizedBox(width: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                  decoration: BoxDecoration(
                    color: kBrand.withOpacity(0.15),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Text('Admin',
                      style: TextStyle(fontSize: 10, color: kBrand, fontWeight: FontWeight.w700)),
                ),
              ],
            ]),
            if (user.email.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(user.email,
                    style: const TextStyle(fontSize: 11, color: kTxtSub)),
              ),
          ]),
        ),
        const PopupMenuDivider(),
        const PopupMenuItem(
          value: 'logout',
          child: Row(children: [
            Icon(Icons.logout, size: 16, color: kSell),
            SizedBox(width: 10),
            Text('Sair', style: TextStyle(fontSize: 13, color: kSell)),
          ]),
        ),
      ],
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Container(
          width: 34,
          height: 34,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(10),
            gradient: const LinearGradient(
              colors: [kBrand2, kBrand],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            boxShadow: [
              BoxShadow(color: kBrand.withOpacity(0.3), blurRadius: 8),
            ],
          ),
          child: Center(
            child: Text(initials,
                style: const TextStyle(
                    color: Colors.white, fontWeight: FontWeight.w800, fontSize: 14)),
          ),
        ),
        const SizedBox(width: 4),
        const Icon(Icons.keyboard_arrow_down, size: 16, color: kTxtSub),
      ]),
    );
  }
}
