import 'dart:convert';

import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api.dart';
import '../fmt.dart';
import '../state.dart';
import '../theme.dart';
import 'dlq.dart' show openUrl;

class AdminScreen extends StatefulWidget {
  const AdminScreen();
  @override
  State<AdminScreen> createState() => _AdminScreenState();
}

class _AdminScreenState extends State<AdminScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 4, vsync: this);
  }

  @override
  void dispose() {
    _tabs.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Container(
        color: kSurface,
        child: TabBar(
          controller: _tabs,
          isScrollable: true,
          tabs: const [
            Tab(icon: Icon(Icons.bar_chart_outlined, size: 16), text: 'Visão Geral'),
            Tab(icon: Icon(Icons.people_outline, size: 16), text: 'Usuários'),
            Tab(icon: Icon(Icons.water_drop_outlined, size: 16), text: 'Faucet'),
            Tab(icon: Icon(Icons.router_outlined, size: 16), text: 'Sistema'),
          ],
        ),
      ),
      Expanded(
        child: TabBarView(
          controller: _tabs,
          children: const [
            _OverviewTab(),
            _UsersTab(),
            _FaucetTab(),
            _SystemTab(),
          ],
        ),
      ),
    ]);
  }
}

// ─── Overview ─────────────────────────────────────────────────────────────────

class _OverviewTab extends StatefulWidget {
  const _OverviewTab();
  @override
  State<_OverviewTab> createState() => _OverviewTabState();
}

class _OverviewTabState extends State<_OverviewTab> {
  Map<String, dynamic>? _stats;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; });
    try {
      final data = await context.read<ApiClient>().getAdminStats();
      if (mounted) setState(() { _stats = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _loading = false; _stats = {}; });
    }
  }

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    if (_loading) return const Center(child: CircularProgressIndicator(color: kBrand));

    final s = _stats ?? {};
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          ShaderMask(
            shaderCallback: (b) => const LinearGradient(colors: [kBrand2, kBrand]).createShader(b),
            child: const Text('Dashboard',
                style: TextStyle(fontSize: 22, fontWeight: FontWeight.w900, color: Colors.white)),
          ),
          const Spacer(),
          IconButton(icon: const Icon(Icons.refresh, color: kTxtSub, size: 18), onPressed: _load),
        ]),
        const SizedBox(height: 14),

        LayoutBuilder(builder: (ctx, box) => GridView.count(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisCount: box.maxWidth >= 700 ? 4 : 2,
          mainAxisSpacing: 10,
          crossAxisSpacing: 10,
          childAspectRatio: box.maxWidth >= 700 ? 1.7 : 2.2,
          children: [
            _KpiCard(Icons.people_outline, 'Usuários', '${s['total_users'] ?? 0}', kBrand),
            _KpiCard(Icons.swap_horiz, 'Trades', '${s['total_trades'] ?? 0}', kBuy),
            _KpiCard(Icons.water_drop_outlined, 'Depósitos', '${s['total_faucets'] ?? 0}', kBrand2),
            _KpiCard(Icons.show_chart, 'Volume Total',
                _fmtVol((s['total_volume'] as num?)?.toDouble() ?? 0),
                const Color(0xFFF0B90B)),
          ],
        )),
        const SizedBox(height: 14),

        _MetricsBarChart(stats: s),
        const SizedBox(height: 14),
        _ActivityDonut(stats: s),
        const SizedBox(height: 14),

        _AdminCard(
          title: 'Status da rede',
          child: Wrap(spacing: 24, runSpacing: 12, children: [
            _Kv('WS Clients', '${s['ws_clients'] ?? 0}', kBrand),
            _Kv('Líder (Bully)',
                (s['leader'] as num?)?.toInt() == -1
                    ? 'Sem líder'
                    : 'Nó ${s['leader'] ?? '?'}',
                Colors.lightBlueAccent),
            _Kv('WebSocket App', app.wsConnected ? 'Conectado' : 'Offline',
                app.wsConnected ? kBuy : kSell),
            _Kv('Blocos minerados', '${app.blocks.length}', kTxtSub),
            _Kv('Eventos recentes', '${app.eventLog.length}', kTxtSub),
          ]),
        ),
      ]),
    );
  }

  String _fmtVol(double v) {
    if (v >= 1e9) return '${(v / 1e9).toStringAsFixed(2)}B';
    if (v >= 1e6) return '${(v / 1e6).toStringAsFixed(2)}M';
    if (v >= 1e3) return '${(v / 1e3).toStringAsFixed(1)}K';
    return v.toStringAsFixed(2);
  }
}

// ─── Gráfico de barras das métricas ──────────────────────────────────────────

class _MetricsBarChart extends StatelessWidget {
  final Map<String, dynamic> stats;
  const _MetricsBarChart({required this.stats});

  @override
  Widget build(BuildContext context) {
    final data = <(String, double, Color)>[
      ('Usuários', ((stats['total_users'] ?? 0) as num).toDouble(), kBrand),
      ('Trades', ((stats['total_trades'] ?? 0) as num).toDouble(), kBuy),
      ('Depósitos', ((stats['total_faucets'] ?? 0) as num).toDouble(), kBrand2),
      ('WS', ((stats['ws_clients'] ?? 0) as num).toDouble(), const Color(0xFFF0B90B)),
    ];
    final maxV = data.map((e) => e.$2).fold<double>(1, (a, b) => b > a ? b : a);

    return _AdminCard(
      title: 'Métricas da exchange',
      child: SizedBox(
        height: 180,
        child: BarChart(BarChartData(
          alignment: BarChartAlignment.spaceAround,
          maxY: maxV * 1.2,
          gridData: FlGridData(
            show: true,
            drawVerticalLine: false,
            horizontalInterval: (maxV / 4).ceilToDouble().clamp(1, double.infinity),
            getDrawingHorizontalLine: (_) =>
                FlLine(color: kBorder.withOpacity(0.5), strokeWidth: 0.5),
          ),
          borderData: FlBorderData(show: false),
          titlesData: FlTitlesData(
            topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
            rightTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
            leftTitles: AxisTitles(
                sideTitles: SideTitles(showTitles: true, reservedSize: 34, getTitlesWidget: (v, m) {
              if (v == m.max) return const SizedBox.shrink();
              return Text(Fmt.compact(v),
                  style: const TextStyle(fontSize: 9, color: kTxtMuted));
            })),
            bottomTitles: AxisTitles(
                sideTitles: SideTitles(showTitles: true, getTitlesWidget: (v, m) {
              final i = v.toInt();
              if (i < 0 || i >= data.length) return const SizedBox.shrink();
              return Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(data[i].$1,
                      style: const TextStyle(fontSize: 10, color: kTxtSub)));
            })),
          ),
          barGroups: [
            for (int i = 0; i < data.length; i++)
              BarChartGroupData(x: i, barRods: [
                BarChartRodData(
                  toY: data[i].$2,
                  color: data[i].$3,
                  width: 26,
                  borderRadius: const BorderRadius.vertical(top: Radius.circular(4)),
                )
              ]),
          ],
        )),
      ),
    );
  }
}

// ─── Donut de composição de atividade ────────────────────────────────────────

class _ActivityDonut extends StatelessWidget {
  final Map<String, dynamic> stats;
  const _ActivityDonut({required this.stats});

  @override
  Widget build(BuildContext context) {
    final parts = <(String, double, Color)>[
      ('Trades', ((stats['total_trades'] ?? 0) as num).toDouble(), kBuy),
      ('Depósitos', ((stats['total_faucets'] ?? 0) as num).toDouble(), kBrand2),
      ('Usuários', ((stats['total_users'] ?? 0) as num).toDouble(), kBrand),
    ];
    final total = parts.fold<double>(0, (a, b) => a + b.$2);

    return _AdminCard(
      title: 'Composição de atividade',
      child: total <= 0
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: Center(
                  child: Text('Sem atividade ainda',
                      style: TextStyle(color: kTxtMuted, fontSize: 12))),
            )
          : Row(children: [
              SizedBox(
                height: 140,
                width: 140,
                child: PieChart(PieChartData(
                  sectionsSpace: 2,
                  centerSpaceRadius: 38,
                  sections: [
                    for (final p in parts)
                      PieChartSectionData(
                        value: p.$2,
                        color: p.$3,
                        title: total > 0
                            ? '${(p.$2 / total * 100).toStringAsFixed(0)}%'
                            : '',
                        radius: 28,
                        titleStyle: const TextStyle(
                            fontSize: 10,
                            fontWeight: FontWeight.w800,
                            color: Colors.black),
                      ),
                  ],
                )),
              ),
              const SizedBox(width: 20),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  for (final p in parts)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 4),
                      child: Row(children: [
                        Container(
                            width: 10,
                            height: 10,
                            decoration: BoxDecoration(
                                color: p.$3,
                                borderRadius: BorderRadius.circular(2))),
                        const SizedBox(width: 8),
                        Text('${p.$1}: ',
                            style: const TextStyle(fontSize: 12, color: kTxtSub)),
                        Text(Fmt.integer(p.$2),
                            style: const TextStyle(
                                fontSize: 12,
                                color: kTxt,
                                fontWeight: FontWeight.w700)),
                      ]),
                    ),
                ],
              ),
            ]),
    );
  }
}

class _KpiCard extends StatelessWidget {
  final IconData icon;
  final String label, value;
  final Color color;
  const _KpiCard(this.icon, this.label, this.value, this.color);

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: kSurface,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: color.withOpacity(0.25)),
          boxShadow: [BoxShadow(color: color.withOpacity(0.06), blurRadius: 10)],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              padding: const EdgeInsets.all(6),
              decoration: BoxDecoration(
                  color: color.withOpacity(0.12), borderRadius: BorderRadius.circular(8)),
              child: Icon(icon, size: 16, color: color),
            ),
            const SizedBox(height: 8),
            Text(value,
                style: TextStyle(
                    fontSize: 22,
                    fontWeight: FontWeight.w900,
                    color: color,
                    fontFeatures: const [FontFeature.tabularFigures()])),
            Text(label, style: const TextStyle(fontSize: 11, color: kTxtSub)),
          ],
        ),
      );
}

// ─── Usuários ─────────────────────────────────────────────────────────────────

class _UsersTab extends StatefulWidget {
  const _UsersTab();
  @override
  State<_UsersTab> createState() => _UsersTabState();
}

class _UsersTabState extends State<_UsersTab> {
  List<Map<String, dynamic>> _users = [];
  bool _loading = true;
  String _q = '';
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = ''; });
    try {
      final data = await context.read<ApiClient>().getAdminUsers();
      final raw = (data['users'] as List<dynamic>?) ?? [];
      if (mounted) {
        setState(() {
          _users = raw.whereType<Map<String, dynamic>>().toList();
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _loading = false; _error = e.toString(); });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator(color: kBrand));
    if (_error.isNotEmpty) return Center(child: Padding(
      padding: const EdgeInsets.all(24),
      child: Column(mainAxisSize: MainAxisSize.min, children: [
        const Icon(Icons.error_outline, color: kSell, size: 36),
        const SizedBox(height: 12),
        Text('Erro ao carregar usuários', style: const TextStyle(color: kSell, fontWeight: FontWeight.w700)),
        const SizedBox(height: 6),
        Text(_error, style: const TextStyle(color: kTxtSub, fontSize: 12), textAlign: TextAlign.center),
        const SizedBox(height: 16),
        FilledButton(onPressed: _load, child: const Text('Tentar novamente')),
      ]),
    ));

    final filtered = _q.isEmpty
        ? _users
        : _users.where((u) {
            final s = _q.toLowerCase();
            return (u['username'] as String? ?? '').toLowerCase().contains(s) ||
                (u['email'] as String? ?? '').toLowerCase().contains(s);
          }).toList();

    return Column(children: [
      // Search
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 10, 16, 8),
        child: Row(children: [
          Expanded(
            child: TextField(
              style: const TextStyle(color: kTxt, fontSize: 13),
              onChanged: (v) => setState(() => _q = v),
              decoration: InputDecoration(
                hintText: 'Buscar usuário…',
                prefixIcon: const Icon(Icons.search, size: 16, color: kTxtSub),
                isDense: true,
                filled: true,
                fillColor: kSurface2,
                border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBorder)),
                enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBorder)),
                focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBrand)),
              ),
            ),
          ),
          const SizedBox(width: 10),
          IconButton(
              icon: const Icon(Icons.refresh, color: kTxtSub, size: 18),
              onPressed: _load),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
                color: kBrand.withOpacity(0.1),
                borderRadius: BorderRadius.circular(8)),
            child: Text(
              '${filtered.length} usuário${filtered.length != 1 ? 's' : ''}',
              style: const TextStyle(
                  fontSize: 12, color: kBrand, fontWeight: FontWeight.w600),
            ),
          ),
        ]),
      ),
      // Header
      Container(
        color: kSurface2,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        child: Row(children: [
          Expanded(flex: 3, child: _H('Usuário')),
          Expanded(flex: 3, child: _H('E-mail')),
          Expanded(flex: 4, child: _H('Saldos')),
          Expanded(flex: 2, child: _H('Cadastro', right: true)),
          const SizedBox(width: 40),
        ]),
      ),
      Container(height: 1, color: kBorder),

      // List
      Expanded(
        child: filtered.isEmpty
            ? const Center(
                child: Text('Nenhum usuário',
                    style: TextStyle(color: kTxtSub)))
            : ListView.separated(
                padding: EdgeInsets.zero,
                itemCount: filtered.length,
                separatorBuilder: (_, __) =>
                    Container(height: 1, color: kBorder.withOpacity(0.4)),
                itemBuilder: (_, i) => _UserRow(user: filtered[i], onChanged: _load),
              ),
      ),
    ]);
  }
}

class _UserRow extends StatelessWidget {
  final Map<String, dynamic> user;
  final VoidCallback onChanged;
  const _UserRow({required this.user, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    final username = user['username'] as String? ?? '';
    final email = user['email'] as String? ?? '';
    final isAdmin = user['is_admin'] as bool? ?? false;
    final createdAt = _fmtDate(user['created_at'] as String? ?? '');

    List<Map<String, dynamic>> bals = [];
    final rawBal = user['balances'];
    if (rawBal is List) {
      bals = rawBal.whereType<Map<String, dynamic>>().toList();
    } else if (rawBal is String) {
      try {
        bals = (json.decode(rawBal) as List)
            .whereType<Map<String, dynamic>>()
            .toList();
      } catch (_) {}
    }

    return InkWell(
      onTap: () => _showManage(context, username, isAdmin, bals),
      child: Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
        // Avatar + name
        Expanded(
          flex: 3,
          child: Row(children: [
            Container(
              width: 34,
              height: 34,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(10),
                gradient: isAdmin
                    ? const LinearGradient(colors: [kBrand2, kBrand])
                    : LinearGradient(
                        colors: [kSurface2, kSurface2.withOpacity(0.5)]),
              ),
              child: Center(
                child: Text(
                    username.isNotEmpty ? username[0].toUpperCase() : '?',
                    style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w800,
                        color: isAdmin ? Colors.white : kTxtSub)),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Row(children: [
                Flexible(
                  child: Text(username,
                      style: const TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                          color: kTxt),
                      overflow: TextOverflow.ellipsis),
                ),
                if (isAdmin) ...[
                  const SizedBox(width: 5),
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                    decoration: BoxDecoration(
                        color: kBrand.withOpacity(0.15),
                        borderRadius: BorderRadius.circular(6)),
                    child: const Text('Admin',
                        style: TextStyle(
                            fontSize: 9,
                            color: kBrand,
                            fontWeight: FontWeight.w800)),
                  ),
                ],
              ]),
            ),
          ]),
        ),

        // Email
        Expanded(
          flex: 3,
          child: Text(email.isEmpty ? '—' : email,
              style: const TextStyle(fontSize: 11, color: kTxtSub),
              overflow: TextOverflow.ellipsis),
        ),

        // Balances
        Expanded(
          flex: 4,
          child: bals.isEmpty
              ? const Text('—', style: TextStyle(fontSize: 11, color: kTxtMuted))
              : Wrap(
                  spacing: 4,
                  runSpacing: 4,
                  children: bals.take(3).map((b) {
                    final a = b['asset'] as String? ?? '';
                    final v = (b['balance'] as num?)?.toInt() ?? 0;
                    final disp = '${Fmt.qty(v / 1e8)} ${Fmt.asset(a)}';
                    return Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 7, vertical: 2),
                      decoration: BoxDecoration(
                          color: kBorder.withOpacity(0.4),
                          borderRadius: BorderRadius.circular(6)),
                      child: Text(disp,
                          style: const TextStyle(
                              fontSize: 10,
                              color: kTxtSub,
                              fontFeatures: [FontFeature.tabularFigures()])),
                    );
                  }).toList(),
                ),
        ),

        // Date
        Expanded(
          flex: 2,
          child: Text(createdAt,
              textAlign: TextAlign.right,
              style: const TextStyle(fontSize: 11, color: kTxtMuted)),
        ),

        // Faucet button
        SizedBox(
          width: 40,
          child: IconButton(
            icon: const Icon(Icons.water_drop_outlined, size: 16, color: kBrand),
            tooltip: 'Faucet para este usuário',
            padding: EdgeInsets.zero,
            constraints:
                const BoxConstraints(minWidth: 32, minHeight: 32),
            onPressed: () => _showFaucet(context, username),
          ),
        ),
      ]),
    ));
  }

  // Painel de gestão do usuário: saldos completos + ajustar saldo + promover.
  void _showManage(BuildContext context, String username, bool isAdmin,
      List<Map<String, dynamic>> bals) {
    final amtCtrl = TextEditingController(text: '1000');
    String asset = 'USDT-sim';
    final assetOptions = <String>{
      'USDT-sim', 'VLT', 'BTC', 'ETH', 'BNB', 'SOL',
      ...bals.map((b) => b['asset'] as String? ?? '').where((a) => a.isNotEmpty),
    }.toList();
    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(builder: (ctx, ss) {
        bool busy = false;
        return AlertDialog(
          backgroundColor: kSurface2,
          shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14), side: const BorderSide(color: kBorder)),
          title: Row(children: [
            Text('@$username', style: const TextStyle(color: kTxt, fontSize: 16, fontWeight: FontWeight.w800)),
            const SizedBox(width: 8),
            if (isAdmin)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(color: kBrand.withOpacity(0.15), borderRadius: BorderRadius.circular(6)),
                child: const Text('Admin', style: TextStyle(fontSize: 9, color: kBrand, fontWeight: FontWeight.w800)),
              ),
          ]),
          content: SizedBox(width: 380, child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Text('SALDOS', style: TextStyle(fontSize: 10, color: kTxtMuted, letterSpacing: 1.5, fontWeight: FontWeight.w700)),
            const SizedBox(height: 6),
            if (bals.isEmpty)
              const Text('Sem saldo', style: TextStyle(fontSize: 12, color: kTxtSub))
            else
              Wrap(spacing: 6, runSpacing: 6, children: bals.map((b) {
                final a = b['asset'] as String? ?? '';
                final v = (b['balance'] as num?)?.toInt() ?? 0;
                return Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(color: kBorder.withOpacity(0.4), borderRadius: BorderRadius.circular(6)),
                  child: Text('${Fmt.qty(v / 1e8)} ${Fmt.asset(a)}',
                      style: const TextStyle(fontSize: 11, color: kTxt, fontFeatures: [FontFeature.tabularFigures()])),
                );
              }).toList()),
            const Divider(height: 24, color: kBorder),
            const Text('AJUSTAR SALDO', style: TextStyle(fontSize: 10, color: kTxtMuted, letterSpacing: 1.5, fontWeight: FontWeight.w700)),
            const SizedBox(height: 6),
            Row(children: [
              Expanded(child: DropdownButtonFormField<String>(
                value: asset, dropdownColor: kSurface2, style: const TextStyle(color: kTxt, fontSize: 13),
                decoration: const InputDecoration(labelText: 'Ativo', isDense: true),
                items: assetOptions.map((a) => DropdownMenuItem(value: a, child: Text(Fmt.asset(a)))).toList(),
                onChanged: (v) => ss(() => asset = v ?? asset),
              )),
              const SizedBox(width: 8),
              Expanded(child: TextField(
                controller: amtCtrl, style: const TextStyle(color: kTxt, fontSize: 13),
                decoration: const InputDecoration(labelText: 'Qtd (- debita)', isDense: true),
                keyboardType: const TextInputType.numberWithOptions(decimal: true, signed: true),
              )),
            ]),
            const SizedBox(height: 6),
            const Text('Use valor negativo para debitar.', style: TextStyle(fontSize: 10, color: kTxtMuted)),
          ])),
          actions: [
            TextButton(
              onPressed: busy ? null : () async {
                ss(() => busy = true);
                try {
                  await ctx.read<ApiClient>().adminPromote(username: username, isAdmin: !isAdmin);
                  if (ctx.mounted) Navigator.pop(ctx);
                  onChanged();
                } catch (_) { ss(() => busy = false); }
              },
              child: Text(isAdmin ? 'Rebaixar' : 'Promover a admin',
                  style: const TextStyle(color: kBrand)),
            ),
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Fechar', style: TextStyle(color: kTxtSub))),
            FilledButton(
              onPressed: busy ? null : () async {
                ss(() => busy = true);
                try {
                  await ctx.read<ApiClient>().adminCredit(
                      username: username, asset: asset, amount: amtCtrl.text.trim());
                  if (ctx.mounted) Navigator.pop(ctx);
                  onChanged();
                } catch (_) { ss(() => busy = false); }
              },
              child: const Text('Aplicar'),
            ),
          ],
        );
      }),
    );
  }

  void _showFaucet(BuildContext context, String username) {
    final amtCtrl = TextEditingController(text: '10000');
    String asset = 'USDT-sim';
    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, ss) => AlertDialog(
          backgroundColor: kSurface2,
          shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14),
              side: const BorderSide(color: kBorder)),
          title: Text('Faucet para @$username',
              style: const TextStyle(color: kTxt, fontSize: 16)),
          content: Column(mainAxisSize: MainAxisSize.min, children: [
            DropdownButtonFormField<String>(
              value: asset,
              dropdownColor: kSurface2,
              style: const TextStyle(color: kTxt),
              decoration: const InputDecoration(labelText: 'Ativo'),
              items: ['USDT-sim', 'VLT', 'BTC', 'ETH', 'BNB', 'SOL']
                  .map((a) => DropdownMenuItem(value: a, child: Text(a)))
                  .toList(),
              onChanged: (v) => ss(() => asset = v ?? asset),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: amtCtrl,
              style: const TextStyle(color: kTxt),
              decoration: const InputDecoration(labelText: 'Quantidade'),
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
            ),
          ]),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: const Text('Cancelar',
                    style: TextStyle(color: kTxtSub))),
            FilledButton(
              onPressed: () async {
                try {
                  await ctx.read<ApiClient>().faucet(
                      account: username,
                      asset: asset,
                      amount: amtCtrl.text.trim());
                } catch (_) {}
                if (ctx.mounted) Navigator.pop(ctx);
              },
              child: const Text('Emitir'),
            ),
          ],
        ),
      ),
    );
  }

  String _fmtDate(String iso) {
    try {
      final d = DateTime.parse(iso).toLocal();
      return '${d.day.toString().padLeft(2, '0')}/${d.month.toString().padLeft(2, '0')}/${d.year}';
    } catch (_) {
      return '—';
    }
  }
}

// ─── Faucet ───────────────────────────────────────────────────────────────────

class _FaucetTab extends StatefulWidget {
  const _FaucetTab();
  @override
  State<_FaucetTab> createState() => _FaucetTabState();
}

class _FaucetTabState extends State<_FaucetTab> {
  final _formKey = GlobalKey<FormState>();
  final _acctCtrl = TextEditingController();
  final _amtCtrl = TextEditingController(text: '10000');
  String _asset = 'USDT-sim';
  bool _busy = false;
  String? _result;
  bool _ok = false;

  @override
  void dispose() {
    _acctCtrl.dispose();
    _amtCtrl.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() { _busy = true; _result = null; });
    try {
      await context.read<ApiClient>().faucet(
          account: _acctCtrl.text.trim(),
          asset: _asset,
          amount: _amtCtrl.text.trim());
      if (mounted) {
        setState(() {
          _result = 'Crédito enviado para @${_acctCtrl.text.trim()}';
          _ok = true;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _result = 'Erro: $e'; _ok = false; });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520),
        child: Form(
          key: _formKey,
          child: _AdminCard(
            title: 'Emitir crédito (admin)',
            subtitle: 'Credita qualquer ativo para qualquer conta do sistema.',
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              TextFormField(
                controller: _acctCtrl,
                style: const TextStyle(color: kTxt),
                decoration: const InputDecoration(
                  labelText: 'Conta (username)',
                  prefixIcon: Icon(Icons.person_outline, size: 18, color: kTxtSub),
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? 'Obrigatório' : null,
              ),
              const SizedBox(height: 14),
              Row(children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    value: _asset,
                    dropdownColor: kSurface2,
                    style: const TextStyle(color: kTxt),
                    decoration: const InputDecoration(labelText: 'Ativo'),
                    items: ['USDT-sim', 'VLT', 'BTC', 'ETH', 'BNB', 'SOL', 'XRP', 'ADA']
                        .map((a) => DropdownMenuItem(value: a, child: Text(a)))
                        .toList(),
                    onChanged: (v) => setState(() => _asset = v ?? _asset),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: TextFormField(
                    controller: _amtCtrl,
                    style: const TextStyle(color: kTxt),
                    decoration: const InputDecoration(labelText: 'Valor'),
                    keyboardType:
                        const TextInputType.numberWithOptions(decimal: true),
                    validator: (v) {
                      final n = double.tryParse(
                          (v ?? '').replaceAll(',', '.'));
                      if (n == null || n <= 0) return 'Inválido';
                      return null;
                    },
                  ),
                ),
              ]),
              if (_result != null) ...[
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: (_ok ? kBuy : kSell).withOpacity(0.08),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                        color: (_ok ? kBuy : kSell).withOpacity(0.3)),
                  ),
                  child: Row(children: [
                    Icon(_ok ? Icons.check_circle_outline : Icons.error_outline,
                        size: 16, color: _ok ? kBuy : kSell),
                    const SizedBox(width: 8),
                    Expanded(child: Text(_result!,
                        style: TextStyle(fontSize: 12, color: _ok ? kBuy : kSell))),
                  ]),
                ),
              ],
              const SizedBox(height: 18),
              FilledButton.icon(
                onPressed: _busy ? null : _send,
                icon: _busy
                    ? const SizedBox(width: 16, height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2, color: kBg))
                    : const Icon(Icons.send_outlined, size: 16, color: kBg),
                label: const Text('Emitir crédito',
                    style: TextStyle(color: kBg)),
              ),
            ]),
          ),
        ),
      ),
    );
  }
}

// ─── Sistema ──────────────────────────────────────────────────────────────────

class _SystemTab extends StatelessWidget {
  const _SystemTab();

  String get _rabbit => 'http://${Uri.base.host}:15672';

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        _AdminCard(
          title: 'Status da rede',
          child: Wrap(spacing: 24, runSpacing: 12, children: [
            _Kv('WebSocket', app.wsConnected ? 'Conectado' : 'Offline',
                app.wsConnected ? kBuy : kSell),
            _Kv('Líder (Bully)',
                app.leader == -1 ? 'Sem líder' : 'Nó ${app.leader}',
                app.leader == -1 ? Colors.orangeAccent : Colors.lightBlueAccent),
            _Kv('Blocos', '${app.blocks.length}', kTxtSub),
            _Kv('Eventos', '${app.eventLog.length}', kTxtSub),
          ]),
        ),
        const SizedBox(height: 14),

        _AdminCard(
          title: 'RabbitMQ Management',
          subtitle: 'Login: admin / admin',
          child: Wrap(spacing: 8, runSpacing: 8, children: [
            FilledButton.icon(
              icon: const Icon(Icons.open_in_new, size: 16),
              label: const Text('Painel RabbitMQ'),
              onPressed: () => openUrl(_rabbit),
            ),
            OutlinedButton.icon(
              icon: const Icon(Icons.bug_report_outlined, size: 16),
              label: const Text('q.dlq'),
              onPressed: () => openUrl('$_rabbit/#/queues/%2Fblockchain/q.dlq'),
            ),
            OutlinedButton.icon(
              icon: const Icon(Icons.list_alt_outlined, size: 16),
              label: const Text('q.ledger.events'),
              onPressed: () => openUrl(
                  '$_rabbit/#/queues/%2Fblockchain/q.ledger.events'),
            ),
          ]),
        ),
        const SizedBox(height: 14),

        _AdminCard(
          title: 'Monitor de eventos',
          child: SizedBox(height: 300, child: _EventLog()),
        ),
      ]),
    );
  }
}

class _EventLog extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final entries = context.watch<AppState>().eventLog;
    if (entries.isEmpty) {
      return const Center(
          child: Text('Aguardando eventos…',
              style: TextStyle(color: kTxtMuted)));
    }
    return ListView.separated(
      itemCount: entries.length,
      separatorBuilder: (_, __) =>
          Container(height: 1, color: kBorder.withOpacity(0.4)),
      itemBuilder: (_, i) {
        final e = entries[i];
        Color c = kBrand;
        if (e.type.startsWith('block')) c = Colors.lightBlueAccent;
        else if (e.type.contains('faucet') || e.type.contains('credit')) c = kBuy;
        else if (e.type.contains('reject') || e.type.contains('error')) c = kSell;
        else if (e.type.contains('leader')) c = Colors.amberAccent;
        else if (e.type.startsWith('trade')) c = kBuy;
        final ts = e.when;
        final time =
            '${ts.hour.toString().padLeft(2, '0')}:${ts.minute.toString().padLeft(2, '0')}:${ts.second.toString().padLeft(2, '0')}';
        return ListTile(
          dense: true,
          leading: Container(width: 3, height: 28, color: c),
          title: Row(children: [
            Text(time,
                style: const TextStyle(
                    fontFamily: 'monospace', fontSize: 11, color: kTxtMuted)),
            const SizedBox(width: 10),
            Text(e.type,
                style: TextStyle(
                    fontSize: 12, color: c, fontWeight: FontWeight.w600)),
          ]),
        );
      },
    );
  }
}

// ─── Shared widgets ───────────────────────────────────────────────────────────

class _AdminCard extends StatelessWidget {
  final String title;
  final String? subtitle;
  final Widget child;
  const _AdminCard({required this.title, this.subtitle, required this.child});

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(
          color: kSurface,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: kBorder),
        ),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(title,
              style: const TextStyle(
                  fontSize: 15, fontWeight: FontWeight.w700, color: kTxt)),
          if (subtitle != null) ...[
            const SizedBox(height: 3),
            Text(subtitle!,
                style: const TextStyle(fontSize: 12, color: kTxtSub)),
          ],
          const SizedBox(height: 14),
          child,
        ]),
      );
}

class _Kv extends StatelessWidget {
  final String k, v;
  final Color c;
  const _Kv(this.k, this.v, this.c);

  @override
  Widget build(BuildContext context) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(k, style: const TextStyle(fontSize: 10, color: kTxtMuted)),
          const SizedBox(height: 2),
          Text(v,
              style: TextStyle(
                  fontWeight: FontWeight.w700, color: c, fontSize: 14)),
        ],
      );
}

class _H extends StatelessWidget {
  final String t;
  final bool right;
  const _H(this.t, {this.right = false});

  @override
  Widget build(BuildContext context) => Text(t,
      textAlign: right ? TextAlign.right : TextAlign.left,
      style: const TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w700,
          color: kTxtMuted));
}
