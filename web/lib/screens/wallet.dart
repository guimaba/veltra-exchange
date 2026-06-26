import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../auth_state.dart';
import '../balance_state.dart';
import '../fmt.dart';
import '../market_state.dart';
import '../theme.dart';
import '../trading_state.dart';
import 'deposit.dart';
import 'withdraw.dart';

class WalletScreen extends StatelessWidget {
  const WalletScreen();

  @override
  Widget build(BuildContext context) {
    final auth    = context.watch<AuthState>();
    final bal     = context.watch<BalanceState>();
    final market  = context.watch<MarketState>();
    final trading = context.watch<TradingState>();
    final username = auth.user?.username ?? '';

    // Calcula portfolio total em BRL
    double totalBRL = 0;
    for (final b in bal.nonZero) {
      final coin = market.coins.where((c) => c.symbol == b.asset).firstOrNull;
      if (coin != null) {
        totalBRL += b.balanceDecimal * coin.priceBRL;
      } else if (b.asset == 'USDT-sim') {
        totalBRL += market.usdToBRL(b.balanceDecimal);
      }
    }

    return RefreshIndicator(
      color: kBrand,
      backgroundColor: kSurface2,
      onRefresh: () => bal.refresh(),
      child: SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _PortfolioHeader(username: username, totalBRL: totalBRL, balances: bal.nonZero, market: market),
          const SizedBox(height: 14),

          // Botão Depositar
          _DepositButton(),
          const SizedBox(height: 20),

          // Assets
          if (bal.loading && bal.nonZero.isEmpty)
            const Center(child: Padding(
              padding: EdgeInsets.symmetric(vertical: 40),
              child: CircularProgressIndicator(color: kBrand, strokeWidth: 2),
            ))
          else if (bal.nonZero.isEmpty)
            _EmptyWallet()
          else ...[
            const _SectionHeader(title: 'Seus ativos'),
            const SizedBox(height: 8),
            _AssetGrid(balances: bal.nonZero, market: market),
          ],

          const SizedBox(height: 24),
          _SectionHeader(title: 'Ordens abertas', badge: '${trading.openOrders.length}'),
          const SizedBox(height: 8),
          _OpenOrdersCard(),

          const SizedBox(height: 24),
          const _SectionHeader(title: 'Últimos trades'),
          const SizedBox(height: 8),
          _TradesCard(),
        ]),
      ),
    );
  }
}

// ─── Portfolio header ─────────────────────────────────────────────────────────

class _PortfolioHeader extends StatelessWidget {
  final String username;
  final double totalBRL;
  final List<AssetBalance> balances;
  final MarketState market;
  const _PortfolioHeader({required this.username, required this.totalBRL,
      required this.balances, required this.market});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(20),
    decoration: BoxDecoration(
      borderRadius: BorderRadius.circular(16),
      gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight,
          colors: [kBrand2.withOpacity(0.3), kBrand.withOpacity(0.15)]),
      border: Border.all(color: kBrand.withOpacity(0.3)),
      boxShadow: [BoxShadow(color: kBrand.withOpacity(0.08), blurRadius: 20)],
    ),
    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Row(children: [
        Text('@$username', style: const TextStyle(fontSize: 12, color: kTxtSub)),
        const Spacer(),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(color: kBrand.withOpacity(0.15), borderRadius: BorderRadius.circular(20)),
          child: const Text('Portfólio', style: TextStyle(fontSize: 11, color: kBrand, fontWeight: FontWeight.w700)),
        ),
      ]),
      const SizedBox(height: 10),
      const Text('Saldo total estimado', style: TextStyle(fontSize: 12, color: kTxtSub)),
      const SizedBox(height: 4),
      Text(Fmt.brl(totalBRL),
          style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w900, color: kTxt,
              fontFeatures: [FontFeature.tabularFigures()])),
      const SizedBox(height: 14),
      // Breakdown pills
      Wrap(spacing: 12, runSpacing: 6, children: balances.take(4).map((b) {
        final coin = market.coins.where((c) => c.symbol == b.asset).firstOrNull;
        final brl = coin != null ? b.balanceDecimal * coin.priceBRL
            : b.asset == 'USDT-sim' ? market.usdToBRL(b.balanceDecimal) : 0.0;
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(color: Colors.white.withOpacity(0.06),
              borderRadius: BorderRadius.circular(20)),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Text(Fmt.asset(b.asset), style: const TextStyle(fontSize: 11, color: kTxt, fontWeight: FontWeight.w700)),
            const SizedBox(width: 6),
            Text(Fmt.brl(brl),
                style: const TextStyle(fontSize: 11, color: kTxtSub,
                    fontFeatures: [FontFeature.tabularFigures()])),
          ]),
        );
      }).toList()),
    ]),
  );
}

// ─── Deposit button ───────────────────────────────────────────────────────────

class _DepositButton extends StatelessWidget {
  @override
  Widget build(BuildContext context) => Row(children: [
    Expanded(child: FilledButton.icon(
      onPressed: () => showDialog(context: context, builder: (_) => const DepositDialog()),
      icon: const Icon(Icons.add_rounded, size: 20, color: kBg),
      label: const Text('Depositar', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15, color: kBg)),
      style: FilledButton.styleFrom(
        backgroundColor: kBrand,
        minimumSize: const Size(double.infinity, 52),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    )),
    const SizedBox(width: 10),
    Expanded(child: OutlinedButton.icon(
      onPressed: () => showDialog(context: context, builder: (_) => const WithdrawDialog()),
      icon: const Icon(Icons.arrow_upward_rounded, size: 18, color: kTxt),
      label: const Text('Sacar', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15, color: kTxt)),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(double.infinity, 52),
        side: const BorderSide(color: kBorder),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    )),
  ]);
}

// ─── Empty wallet ─────────────────────────────────────────────────────────────

class _EmptyWallet extends StatelessWidget {
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(vertical: 40),
    decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(14), border: Border.all(color: kBorder)),
    child: Column(children: [
      const Icon(Icons.account_balance_wallet_outlined, size: 40, color: kTxtMuted),
      const SizedBox(height: 12),
      const Text('Carteira vazia', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: kTxtSub)),
      const SizedBox(height: 6),
      const Text('Faça um depósito para começar a negociar.',
          style: TextStyle(fontSize: 12, color: kTxtMuted)),
    ]),
  );
}

// ─── Asset grid ───────────────────────────────────────────────────────────────

class _AssetGrid extends StatelessWidget {
  final List<AssetBalance> balances;
  final MarketState market;
  const _AssetGrid({required this.balances, required this.market});

  @override
  Widget build(BuildContext context) => Column(
    children: balances.map((b) {
      final coin = market.coins.where((c) => c.symbol == b.asset).firstOrNull;
      final brl  = coin != null ? b.balanceDecimal * coin.priceBRL
          : b.asset == 'USDT-sim' ? b.balanceDecimal * 5.0 : 0.0;
      return _AssetRow(bal: b, brl: brl, coin: coin);
    }).toList(),
  );
}

class _AssetRow extends StatelessWidget {
  final AssetBalance bal;
  final double brl;
  final MarketCoin? coin;
  const _AssetRow({required this.bal, required this.brl, required this.coin});

  Color _color() {
    int h = 0; for (final c in bal.asset.codeUnits) h = (h * 31 + c) & 0xFFFF;
    const cols = [Color(0xFFF7931A), Color(0xFF627EEA), Color(0xFFF3BA2F), Color(0xFF9945FF),
      Color(0xFF00AAE4), Color(0xFFE84142), Color(0xFF00D4FF), Color(0xFF02C076)];
    return cols[h % cols.length];
  }

  @override
  Widget build(BuildContext context) {
    final c = _color();
    final pctChange = coin?.change24h ?? 0;
    final isUp = pctChange >= 0;

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(12),
          border: Border.all(color: kBorder)),
      child: Row(children: [
        // Avatar
        Container(width: 38, height: 38,
          decoration: BoxDecoration(borderRadius: BorderRadius.circular(10),
              color: c.withOpacity(0.15), border: Border.all(color: c.withOpacity(0.3))),
          child: Center(child: Text(bal.asset.isNotEmpty ? bal.asset[0] : '?',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: c))),
        ),
        const SizedBox(width: 12),

        // Name + change
        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(Fmt.asset(bal.asset), style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: kTxt)),
          if (coin != null)
            Row(children: [
              Text(Fmt.usd(coin!.priceUSD),
                  style: const TextStyle(fontSize: 11, color: kTxtSub,
                      fontFeatures: [FontFeature.tabularFigures()])),
              const SizedBox(width: 6),
              Text(Fmt.pct(pctChange),
                  style: TextStyle(fontSize: 10, color: isUp ? kBuy : kSell, fontWeight: FontWeight.w600)),
            ]),
        ])),

        // Balance + BRL
        Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
          Text(Fmt.qty(bal.balanceDecimal),
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: kTxt,
                  fontFeatures: [FontFeature.tabularFigures()])),
          Text(Fmt.brl(brl),
              style: const TextStyle(fontSize: 11, color: kTxtSub,
                  fontFeatures: [FontFeature.tabularFigures()])),
        ]),
      ]),
    );
  }
}

// ─── Section header ───────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  final String title; final String? badge;
  const _SectionHeader({required this.title, this.badge});
  @override
  Widget build(BuildContext context) => Row(children: [
    Container(width: 3, height: 16,
        decoration: BoxDecoration(color: kBrand, borderRadius: BorderRadius.circular(2),
            boxShadow: [BoxShadow(color: kBrand.withOpacity(0.6), blurRadius: 6)])),
    const SizedBox(width: 8),
    Text(title, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: kTxt)),
    if (badge != null) ...[
      const SizedBox(width: 8),
      Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(color: kBrand.withOpacity(0.12), borderRadius: BorderRadius.circular(20)),
        child: Text(badge!, style: const TextStyle(fontSize: 11, color: kBrand, fontWeight: FontWeight.w700)),
      ),
    ],
  ]);
}

// ─── Open orders ──────────────────────────────────────────────────────────────

class _OpenOrdersCard extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final orders = context.watch<TradingState>().openOrders;
    if (orders.isEmpty) return _Empty(Icons.receipt_long_outlined, 'Nenhuma ordem aberta');
    return Container(
      decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(12), border: Border.all(color: kBorder)),
      child: Column(children: orders.take(5).map((o) {
        final c = o.side == 'buy' ? kBuy : kSell;
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          decoration: BoxDecoration(border: Border(bottom: BorderSide(color: kBorder.withOpacity(0.5)))),
          child: Row(children: [
            Container(width: 3, height: 36, decoration: BoxDecoration(color: c, borderRadius: BorderRadius.circular(2))),
            const SizedBox(width: 10),
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text('${o.side == 'buy' ? 'COMPRA' : 'VENDA'} ${o.type.toUpperCase()} · ${o.pair}',
                  style: TextStyle(fontSize: 12, color: c, fontWeight: FontWeight.w800)),
              Text(o.type == 'market' ? Fmt.qty(o.quantity / 1e8)
                  : '${Fmt.qty(o.quantity / 1e8)} @ ${Fmt.price(o.price / 1e8)}',
                  style: const TextStyle(fontSize: 11, color: kTxtSub)),
            ])),
            _Pill(o.status),
          ]),
        );
      }).toList()),
    );
  }
}

class _Pill extends StatelessWidget {
  final String s; const _Pill(this.s);
  @override
  Widget build(BuildContext context) {
    final c = switch (s) {
      'filled' => kBuy, 'partially_filled' => const Color(0xFFF0B90B),
      'canceled' => kTxtSub, 'rejected' => kSell, _ => kBrand,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(color: c.withOpacity(0.1), borderRadius: BorderRadius.circular(20),
          border: Border.all(color: c.withOpacity(0.3))),
      child: Text(s, style: TextStyle(fontSize: 10, color: c, fontWeight: FontWeight.w700)),
    );
  }
}

// ─── Trades card ──────────────────────────────────────────────────────────────

class _TradesCard extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final trades = context.watch<TradingState>().allTrades.take(8).toList();
    if (trades.isEmpty) return _Empty(Icons.swap_horiz_outlined, 'Nenhuma movimentação ainda');
    return Container(
      decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(12), border: Border.all(color: kBorder)),
      child: Column(children: [
        Padding(padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
            child: Row(children: [
              Expanded(child: _H('Par')),
              Expanded(child: _H('Preço', right: true)),
              Expanded(child: _H('Qtd', right: true)),
              Expanded(child: _H('Hora', right: true)),
            ])),
        Container(height: 1, color: kBorder),
        ...trades.map((t) {
          final up = t.takerSide == 'buy'; final c = up ? kBuy : kSell;
          final ts = DateTime.fromMillisecondsSinceEpoch(t.timestampMs);
          return Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
            child: Row(children: [
              Expanded(child: Text(t.pair, style: const TextStyle(fontSize: 11, color: kTxt))),
              Expanded(child: Text(Fmt.price(t.price / 1e8), textAlign: TextAlign.right,
                  style: TextStyle(fontSize: 11, color: c, fontFeatures: const [FontFeature.tabularFigures()]))),
              Expanded(child: Text(Fmt.qty(t.quantity / 1e8), textAlign: TextAlign.right,
                  style: const TextStyle(fontSize: 11, color: kTxt, fontFeatures: [FontFeature.tabularFigures()]))),
              Expanded(child: Text(
                  '${ts.hour.toString().padLeft(2,'0')}:${ts.minute.toString().padLeft(2,'0')}',
                  textAlign: TextAlign.right, style: const TextStyle(fontSize: 11, color: kTxtMuted))),
            ]),
          );
        }),
      ]),
    );
  }
}

class _H extends StatelessWidget {
  final String t; final bool right;
  const _H(this.t, {this.right = false});
  @override
  Widget build(BuildContext context) => Text(t,
      textAlign: right ? TextAlign.right : TextAlign.left,
      style: const TextStyle(fontSize: 10, color: kTxtMuted, fontWeight: FontWeight.w600));
}

class _Empty extends StatelessWidget {
  final IconData icon; final String label;
  const _Empty(this.icon, this.label);
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(vertical: 32),
    decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(12), border: Border.all(color: kBorder)),
    child: Column(children: [
      Icon(icon, size: 32, color: kTxtMuted),
      const SizedBox(height: 8),
      Text(label, style: const TextStyle(fontSize: 13, color: kTxtSub)),
    ]),
  );
}
