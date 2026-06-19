import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';

import '../market_state.dart';
import '../theme.dart';
import 'coin_detail.dart';

final _fmtBRL = NumberFormat.currency(locale: 'pt_BR', symbol: 'R\$');
final _fmtUSD = NumberFormat.currency(locale: 'en_US', symbol: '\$');
final _fmtVol = NumberFormat.compactCurrency(locale: 'en_US', symbol: '\$');

class MarketScreen extends StatefulWidget {
  const MarketScreen();

  @override
  State<MarketScreen> createState() => _MarketScreenState();
}

class _MarketScreenState extends State<MarketScreen> {
  int _filter = 0; // 0=All 1=Gainers 2=Losers

  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    var coins = market.coins;

    if (_filter == 1) coins = coins.where((c) => c.change24h >= 0).toList()
        ..sort((a, b) => b.change24h.compareTo(a.change24h));
    if (_filter == 2) coins = coins.where((c) => c.change24h < 0).toList()
        ..sort((a, b) => a.change24h.compareTo(b.change24h));

    return Column(children: [
      // ── Header ──
      _MarketHeader(market: market, filter: _filter, onFilter: (f) => setState(() => _filter = f)),

      // ── Column headers ──
      Container(
        color: kSurface,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        child: Row(children: [
          const SizedBox(width: 36),
          Expanded(flex: 4, child: _Hdr('Moeda')),
          Expanded(flex: 3, child: _Hdr('Preço BRL', right: true)),
          Expanded(flex: 3, child: _Hdr('Preço USD', right: true)),
          Expanded(flex: 2, child: _Hdr('24h', right: true)),
          Expanded(flex: 3, child: _Hdr('Volume', right: true)),
          const SizedBox(width: 64), // sparkline space
        ]),
      ),
      Container(height: 1, color: kBorder),

      // ── Coin list ──
      Expanded(
        child: coins.isEmpty
            ? _EmptyState(filter: _filter)
            : ListView.separated(
                itemCount: coins.length,
                separatorBuilder: (_, __) => Container(height: 1, color: kBorder.withOpacity(0.5)),
                itemBuilder: (_, i) => _CoinRow(coin: coins[i], market: market),
              ),
      ),

      // ── Footer timestamp ──
      if (market.updatedAt > 0)
        Container(
          color: kSurface,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 5),
          child: Row(mainAxisAlignment: MainAxisAlignment.end, children: [
            Container(
              width: 6, height: 6,
              decoration: BoxDecoration(
                color: kBrand,
                shape: BoxShape.circle,
                boxShadow: [BoxShadow(color: kBrand.withOpacity(0.8), blurRadius: 6)],
              ),
            ),
            const SizedBox(width: 6),
            Text(_fmtTime(market.updatedAt),
                style: const TextStyle(fontSize: 10, color: kTxtMuted)),
          ]),
        ),
    ]);
  }

  String _fmtTime(int ms) {
    final d = DateTime.fromMillisecondsSinceEpoch(ms);
    return 'Atualizado ${d.hour.toString().padLeft(2,'0')}:${d.minute.toString().padLeft(2,'0')}:${d.second.toString().padLeft(2,'0')}';
  }
}

// ─── Market header ────────────────────────────────────────────────────────────

class _MarketHeader extends StatelessWidget {
  final MarketState market;
  final int filter;
  final void Function(int) onFilter;
  const _MarketHeader({required this.market, required this.filter, required this.onFilter});

  @override
  Widget build(BuildContext context) {
    final coins = market.coins;
    final gainers = coins.where((c) => c.change24h >= 0).length;
    final losers  = coins.where((c) => c.change24h < 0).length;

    return Container(
      color: kSurface,
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 0),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        // Title row
        Row(children: [
          ShaderMask(
            shaderCallback: (b) => const LinearGradient(colors: [kBrand2, kBrand]).createShader(b),
            child: const Text('Mercado Cripto',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.w900, color: Colors.white)),
          ),
          const Spacer(),
          // Search
          SizedBox(
            width: 200,
            child: TextField(
              style: const TextStyle(color: kTxt, fontSize: 13),
              decoration: InputDecoration(
                hintText: 'Buscar...',
                prefixIcon: const Icon(Icons.search, size: 16, color: kTxtSub),
                contentPadding: const EdgeInsets.symmetric(vertical: 8),
                isDense: true,
                filled: true,
                fillColor: kBorder.withOpacity(0.3),
                border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBorder)),
                enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBorder)),
                focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8),
                    borderSide: const BorderSide(color: kBrand, width: 1.5)),
              ),
              onChanged: market.setSearch,
            ),
          ),
        ]),
        const SizedBox(height: 12),

        // Stats chips
        Row(children: [
          _StatChip('${coins.length}', 'Moedas', kBrand),
          const SizedBox(width: 8),
          _StatChip('$gainers', 'Em alta', kBuy),
          const SizedBox(width: 8),
          _StatChip('$losers', 'Em baixa', kSell),
        ]),
        const SizedBox(height: 12),

        // Filter tabs
        Row(children: [
          for (final f in [
            (0, 'Todos'),
            (1, '↑ Gainers'),
            (2, '↓ Losers'),
          ])
            Padding(
              padding: const EdgeInsets.only(right: 4),
              child: GestureDetector(
                onTap: () => onFilter(f.$1),
                child: AnimatedContainer(
                  duration: const Duration(milliseconds: 150),
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
                  decoration: BoxDecoration(
                    color: filter == f.$1 ? kBrand.withOpacity(0.12) : Colors.transparent,
                    borderRadius: BorderRadius.circular(20),
                    border: Border.all(
                        color: filter == f.$1 ? kBrand.withOpacity(0.5) : kBorder),
                  ),
                  child: Text(f.$2,
                      style: TextStyle(
                          fontSize: 12,
                          color: filter == f.$1 ? kBrand : kTxtSub,
                          fontWeight: filter == f.$1 ? FontWeight.w700 : FontWeight.normal)),
                ),
              ),
            ),
        ]),
        const SizedBox(height: 4),
      ]),
    );
  }
}

class _StatChip extends StatelessWidget {
  final String value, label;
  final Color color;
  const _StatChip(this.value, this.label, this.color);

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
        decoration: BoxDecoration(
          color: color.withOpacity(0.08),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: color.withOpacity(0.2)),
        ),
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          Text(value, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: color)),
          const SizedBox(width: 5),
          Text(label, style: TextStyle(fontSize: 11, color: color.withOpacity(0.7))),
        ]),
      );
}

// ─── Coin row ─────────────────────────────────────────────────────────────────

class _CoinRow extends StatelessWidget {
  final MarketCoin coin;
  final MarketState market;
  const _CoinRow({required this.coin, required this.market});

  @override
  Widget build(BuildContext context) {
    final up = coin.change24h >= 0;
    final c  = up ? kBuy : kSell;

    return InkWell(
      onTap: () => Navigator.push(context,
          MaterialPageRoute(builder: (_) => CoinDetailScreen(coin: coin))),
      hoverColor: kBrand.withOpacity(0.04),
      child: Container(
        color: Colors.transparent,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Row(children: [
          // Avatar
          _CoinAvatar(coin.symbol, size: 28),
          const SizedBox(width: 8),

          // Name
          Expanded(
            flex: 4,
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(coin.symbol,
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt)),
              Text(coin.name,
                  style: const TextStyle(fontSize: 10, color: kTxtSub),
                  overflow: TextOverflow.ellipsis),
            ]),
          ),

          // BRL
          Expanded(
            flex: 3,
            child: Text(_fmtBRL.format(coin.priceBRL),
                textAlign: TextAlign.right,
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt,
                    fontFeatures: [FontFeature.tabularFigures()])),
          ),

          // USD
          Expanded(
            flex: 3,
            child: Text(_fmtUSD.format(coin.priceUSD),
                textAlign: TextAlign.right,
                style: const TextStyle(fontSize: 11, color: kTxtSub,
                    fontFeatures: [FontFeature.tabularFigures()])),
          ),

          // 24h change
          Expanded(
            flex: 2,
            child: Align(
              alignment: Alignment.centerRight,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 3),
                decoration: BoxDecoration(
                  color: c.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Text(
                  '${up ? '+' : ''}${coin.change24h.toStringAsFixed(2)}%',
                  style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w700,
                      fontFeatures: const [FontFeature.tabularFigures()]),
                ),
              ),
            ),
          ),

          // Volume
          Expanded(
            flex: 3,
            child: Text(_fmtVol.format(coin.volume24hUSD),
                textAlign: TextAlign.right,
                style: const TextStyle(fontSize: 11, color: kTxtSub,
                    fontFeatures: [FontFeature.tabularFigures()])),
          ),

          // Sparkline
          SizedBox(
            width: 64,
            height: 28,
            child: _Sparkline(coin: coin, market: market, up: up),
          ),
        ]),
      ),
    );
  }
}

// ─── Sparkline (mini chart) ───────────────────────────────────────────────────

class _Sparkline extends StatefulWidget {
  final MarketCoin coin;
  final MarketState market;
  final bool up;
  const _Sparkline({required this.coin, required this.market, required this.up});

  @override
  State<_Sparkline> createState() => _SparklineState();
}

class _SparklineState extends State<_Sparkline> {
  List<Candle> _candles = [];
  bool _loaded = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final candles = await widget.market.loadCandles(widget.coin.symbol);
    if (mounted) setState(() { _candles = candles; _loaded = true; });
  }

  @override
  Widget build(BuildContext context) {
    if (!_loaded || _candles.isEmpty) {
      return Container(
        decoration: BoxDecoration(
          color: kBorder.withOpacity(0.2),
          borderRadius: BorderRadius.circular(4),
        ),
      );
    }

    final last = _candles.length > 20 ? _candles.sublist(_candles.length - 20) : _candles;
    final spots = last.asMap().entries
        .map((e) => FlSpot(e.key.toDouble(), e.value.c))
        .toList();
    final c = widget.up ? kBuy : kSell;

    return LineChart(
      LineChartData(
        gridData: const FlGridData(show: false),
        titlesData: const FlTitlesData(show: false),
        borderData: FlBorderData(show: false),
        lineTouchData: const LineTouchData(enabled: false),
        lineBarsData: [
          LineChartBarData(
            spots: spots,
            isCurved: true,
            curveSmoothness: 0.4,
            color: c,
            barWidth: 1.5,
            dotData: const FlDotData(show: false),
            belowBarData: BarAreaData(
              show: true,
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [c.withOpacity(0.25), c.withOpacity(0.0)],
              ),
            ),
          ),
        ],
        minX: 0,
        maxX: (last.length - 1).toDouble(),
      ),
    );
  }
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

class _Hdr extends StatelessWidget {
  final String text;
  final bool right;
  const _Hdr(this.text, {this.right = false});

  @override
  Widget build(BuildContext context) => Text(text,
      textAlign: right ? TextAlign.right : TextAlign.left,
      style: const TextStyle(fontSize: 10, fontWeight: FontWeight.w600, color: kTxtMuted));
}

class _EmptyState extends StatelessWidget {
  final int filter;
  const _EmptyState({required this.filter});

  @override
  Widget build(BuildContext context) => Center(
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          Icon(
            filter == 0 ? Icons.cloud_off : Icons.search_off,
            size: 40, color: kTxtMuted,
          ),
          const SizedBox(height: 12),
          Text(
            filter == 0 ? 'Aguardando dados de mercado…' : 'Nenhuma moeda neste filtro',
            style: const TextStyle(color: kTxtSub, fontSize: 13),
          ),
        ]),
      );
}

class _CoinAvatar extends StatelessWidget {
  final String symbol;
  final double size;
  const _CoinAvatar(this.symbol, {this.size = 32});

  Color _color() {
    int h = 0;
    for (final c in symbol.codeUnits) h = (h * 31 + c) & 0xFFFFFF;
    const cols = [
      Color(0xFFF7931A), Color(0xFF627EEA), Color(0xFFF3BA2F), Color(0xFF9945FF),
      Color(0xFF00AAE4), Color(0xFFE84142), Color(0xFF8247E5), Color(0xFF375BD2),
      Color(0xFFFF007A), Color(0xFF00D4FF), Color(0xFF7B2FBE), Color(0xFF02C076),
    ];
    return cols[h % cols.length];
  }

  @override
  Widget build(BuildContext context) {
    final c = _color();
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(size * 0.28),
        color: c.withOpacity(0.15),
        border: Border.all(color: c.withOpacity(0.25)),
      ),
      child: Center(
        child: Text(
          symbol.isNotEmpty ? symbol[0] : '?',
          style: TextStyle(
              fontSize: size * 0.45,
              fontWeight: FontWeight.w800,
              color: c),
        ),
      ),
    );
  }
}
