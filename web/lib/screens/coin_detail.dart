import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';

import '../market_state.dart';
import '../theme.dart';

final _fmtBRL  = NumberFormat.currency(locale: 'pt_BR', symbol: 'R\$');
final _fmtUSD  = NumberFormat.currency(locale: 'en_US', symbol: '\$');
final _fmtVol  = NumberFormat.compactCurrency(locale: 'en_US', symbol: '\$');
final _fmtMcap = NumberFormat.compactCurrency(locale: 'en_US', symbol: '\$');

class CoinDetailScreen extends StatefulWidget {
  final MarketCoin coin;
  const CoinDetailScreen({super.key, required this.coin});

  @override
  State<CoinDetailScreen> createState() => _CoinDetailScreenState();
}

class _CoinDetailScreenState extends State<CoinDetailScreen> {
  List<Candle> _all = [];
  bool _loading = true;
  int _timeframe = 50;

  static const _frames = [(10,'10'), (20,'20'), (50,'50'), (100,'100'), (200,'200')];

  @override
  void initState() { super.initState(); _load(); }

  Future<void> _load() async {
    final c = await context.read<MarketState>().loadCandles(widget.coin.symbol);
    if (mounted) setState(() { _all = c; _loading = false; });
  }

  List<Candle> get _visible {
    if (_all.isEmpty) return [];
    return _all.length > _timeframe ? _all.sublist(_all.length - _timeframe) : _all;
  }

  @override
  Widget build(BuildContext context) {
    final coin = widget.coin;
    final c    = coin.isUp ? kBuy : kSell;

    return Scaffold(
      backgroundColor: kBg,
      appBar: AppBar(
        backgroundColor: kSurface,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new, size: 16, color: kTxtSub),
          onPressed: () => Navigator.pop(context),
        ),
        title: Row(children: [
          _CoinAvatar(coin.symbol, size: 30),
          const SizedBox(width: 10),
          Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(coin.symbol, style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15, color: kTxt)),
            Text(coin.name,   style: const TextStyle(fontSize: 10, color: kTxtSub)),
          ]),
        ]),
        actions: [
          Container(
            margin: const EdgeInsets.only(right: 16),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(color: c.withOpacity(0.12), borderRadius: BorderRadius.circular(20)),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              Icon(coin.isUp ? Icons.arrow_upward : Icons.arrow_downward, size: 12, color: c),
              const SizedBox(width: 4),
              Text('${coin.change24h.abs().toStringAsFixed(2)}%',
                  style: TextStyle(fontSize: 12, color: c, fontWeight: FontWeight.w700)),
            ]),
          ),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _PriceHeader(coin: coin),
          const SizedBox(height: 16),
          _ChartContainer(
            candles: _visible, loading: _loading, isUp: coin.isUp,
            timeframe: _timeframe, frames: _frames,
            onFrameChange: (f) => setState(() => _timeframe = f),
          ),
          const SizedBox(height: 16),
          _StatsGrid(coin: coin),
        ]),
      ),
    );
  }
}

// ─── Price header ─────────────────────────────────────────────────────────────

class _PriceHeader extends StatelessWidget {
  final MarketCoin coin;
  const _PriceHeader({required this.coin});

  @override
  Widget build(BuildContext context) {
    final c = coin.isUp ? kBuy : kSell;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: kSurface, borderRadius: BorderRadius.circular(16),
        border: Border.all(color: c.withOpacity(0.2)),
        boxShadow: [BoxShadow(color: c.withOpacity(0.06), blurRadius: 16)],
      ),
      child: Row(children: [
        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text('Preço atual (BRL)', style: TextStyle(fontSize: 11, color: kTxtSub)),
          const SizedBox(height: 4),
          Text(_fmtBRL.format(coin.priceBRL),
              style: const TextStyle(fontSize: 30, fontWeight: FontWeight.w900, color: kTxt,
                  fontFeatures: [FontFeature.tabularFigures()])),
          const SizedBox(height: 4),
          Text(_fmtUSD.format(coin.priceUSD),
              style: const TextStyle(fontSize: 14, color: kTxtSub,
                  fontFeatures: [FontFeature.tabularFigures()])),
        ])),
        Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
          _SB('Volume 24h', _fmtVol.format(coin.volume24hUSD)),
          const SizedBox(height: 8),
          _SB('Market Cap', _fmtMcap.format(coin.marketCapUSD)),
        ]),
      ]),
    );
  }
}

class _SB extends StatelessWidget {
  final String l, v;
  const _SB(this.l, this.v);
  @override
  Widget build(BuildContext context) => Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
    Text(l, style: const TextStyle(fontSize: 10, color: kTxtMuted)),
    Text(v, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt,
        fontFeatures: [FontFeature.tabularFigures()])),
  ]);
}

// ─── Chart container ──────────────────────────────────────────────────────────

class _ChartContainer extends StatelessWidget {
  final List<Candle> candles;
  final bool loading, isUp;
  final int timeframe;
  final List<(int, String)> frames;
  final void Function(int) onFrameChange;
  const _ChartContainer({required this.candles, required this.loading, required this.isUp,
      required this.timeframe, required this.frames, required this.onFrameChange});

  @override
  Widget build(BuildContext context) => Container(
    decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(16),
        border: Border.all(color: kBorder)),
    child: Column(children: [
      // Timeframe bar
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
        child: Row(children: [
          const Text('GRÁFICO', style: TextStyle(fontSize: 10, color: kTxtMuted,
              letterSpacing: 2, fontWeight: FontWeight.w700)),
          const Spacer(),
          Row(mainAxisSize: MainAxisSize.min, children: frames.map((f) {
            final sel = f.$1 == timeframe;
            return GestureDetector(
              onTap: () => onFrameChange(f.$1),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 150),
                margin: const EdgeInsets.only(left: 4),
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                decoration: BoxDecoration(
                  color: sel ? kBrand.withOpacity(0.15) : Colors.transparent,
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(color: sel ? kBrand.withOpacity(0.5) : Colors.transparent),
                ),
                child: Text(f.$2, style: TextStyle(fontSize: 11,
                    color: sel ? kBrand : kTxtSub,
                    fontWeight: sel ? FontWeight.w700 : FontWeight.normal)),
              ),
            );
          }).toList()),
        ]),
      ),
      // Chart
      SizedBox(
        height: 240,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(8, 0, 16, 8),
          child: loading
              ? _Shimmer()
              : candles.isEmpty
                  ? const Center(child: Text('Sem dados', style: TextStyle(color: kTxtSub)))
                  : _LineChart(candles: candles, isUp: isUp),
        ),
      ),
    ]),
  );
}

class _LineChart extends StatelessWidget {
  final List<Candle> candles;
  final bool isUp;
  const _LineChart({required this.candles, required this.isUp});

  @override
  Widget build(BuildContext context) {
    final c     = isUp ? kBuy : kSell;
    final spots = candles.asMap().entries.map((e) => FlSpot(e.key.toDouble(), e.value.c)).toList();
    final minY  = spots.map((s) => s.y).reduce((a, b) => a < b ? a : b);
    final maxY  = spots.map((s) => s.y).reduce((a, b) => a > b ? a : b);
    final pad   = (maxY - minY) * 0.12 + 1e-8;

    return LineChart(LineChartData(
      minX: 0, maxX: (candles.length - 1).toDouble(),
      minY: minY - pad, maxY: maxY + pad,
      gridData: FlGridData(show: true, drawVerticalLine: false,
          getDrawingHorizontalLine: (_) => FlLine(color: kBorder.withOpacity(0.6), strokeWidth: 0.5)),
      borderData: FlBorderData(show: false),
      titlesData: FlTitlesData(
        leftTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        bottomTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        rightTitles: AxisTitles(sideTitles: SideTitles(showTitles: true, reservedSize: 64,
          getTitlesWidget: (v, m) {
            if (v == m.min || v == m.max) return const SizedBox.shrink();
            return Padding(padding: const EdgeInsets.only(left: 4),
                child: Text(_fy(v), style: const TextStyle(fontSize: 9, color: kTxtMuted)));
          },
        )),
      ),
      lineTouchData: LineTouchData(touchTooltipData: LineTouchTooltipData(
        getTooltipItems: (s) => s.map((x) => LineTooltipItem(
          _fmtUSD.format(x.y),
          TextStyle(color: c, fontWeight: FontWeight.w700, fontSize: 11),
        )).toList(),
      )),
      lineBarsData: [LineChartBarData(
        spots: spots, isCurved: true, curveSmoothness: 0.25,
        color: c, barWidth: 2,
        dotData: const FlDotData(show: false),
        belowBarData: BarAreaData(show: true,
          gradient: LinearGradient(begin: Alignment.topCenter, end: Alignment.bottomCenter,
              colors: [c.withOpacity(0.2), c.withOpacity(0.0)])),
      )],
    ));
  }

  String _fy(double v) {
    if (v >= 1e6) return '\$${(v/1e6).toStringAsFixed(1)}M';
    if (v >= 1e3) return '\$${(v/1e3).toStringAsFixed(1)}K';
    if (v >= 1)   return '\$${v.toStringAsFixed(2)}';
    return '\$${v.toStringAsFixed(6)}';
  }
}

class _Shimmer extends StatefulWidget {
  @override State<_Shimmer> createState() => _ShimmerState();
}

class _ShimmerState extends State<_Shimmer> with SingleTickerProviderStateMixin {
  late final AnimationController _c;
  @override
  void initState() { super.initState(); _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 1200))..repeat(); }
  @override void dispose() { _c.dispose(); super.dispose(); }
  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: _c,
    builder: (_, __) => Container(
      decoration: BoxDecoration(borderRadius: BorderRadius.circular(8),
        gradient: LinearGradient(
          begin: Alignment(-1.0 + _c.value * 2.5, 0),
          end: Alignment(0 + _c.value * 2.5, 0),
          colors: [kSurface, kSurface2, kBorder.withOpacity(0.5), kSurface2, kSurface],
        )),
    ),
  );
}

// ─── Stats grid ───────────────────────────────────────────────────────────────

class _StatsGrid extends StatelessWidget {
  final MarketCoin coin;
  const _StatsGrid({required this.coin});

  @override
  Widget build(BuildContext context) {
    final c = coin.isUp ? kBuy : kSell;
    final items = [
      ('Preço BRL', _fmtBRL.format(coin.priceBRL), null),
      ('Preço USD', _fmtUSD.format(coin.priceUSD), null),
      ('Variação 24h', '${coin.isUp ? '+' : ''}${coin.change24h.toStringAsFixed(2)}%', c),
      ('Volume 24h', _fmtVol.format(coin.volume24hUSD), null),
      ('Market Cap', _fmtMcap.format(coin.marketCapUSD), null),
      ('Símbolo', coin.symbol, kBrand),
    ];
    return GridView.count(
      shrinkWrap: true, physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 2, mainAxisSpacing: 10, crossAxisSpacing: 10, childAspectRatio: 2.6,
      children: items.map((i) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(color: kSurface, borderRadius: BorderRadius.circular(12),
            border: Border.all(color: kBorder)),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisAlignment: MainAxisAlignment.center, children: [
          Text(i.$1, style: const TextStyle(fontSize: 10, color: kTxtMuted)),
          const SizedBox(height: 4),
          Text(i.$2, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: i.$3,
              fontFeatures: const [FontFeature.tabularFigures()]), overflow: TextOverflow.ellipsis),
        ]),
      )).toList(),
    );
  }
}

// ─── Coin avatar ─────────────────────────────────────────────────────────────

class _CoinAvatar extends StatelessWidget {
  final String symbol; final double size;
  const _CoinAvatar(this.symbol, {this.size = 32});

  Color _c() {
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
    final c = _c();
    return Container(
      width: size, height: size,
      decoration: BoxDecoration(borderRadius: BorderRadius.circular(size * 0.28),
          color: c.withOpacity(0.15), border: Border.all(color: c.withOpacity(0.3))),
      child: Center(child: Text(symbol.isNotEmpty ? symbol[0] : '?',
          style: TextStyle(fontSize: size * 0.45, fontWeight: FontWeight.w800, color: c))),
    );
  }
}
