import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';

import '../auth_state.dart';
import '../balance_state.dart';
import '../market_state.dart';
import '../theme.dart';
import '../trading_state.dart';
import 'deposit.dart';

const kBuyColor  = kBuy;
const kSellColor = kSell;

// ─── Screen root ──────────────────────────────────────────────────────────────

class TradeScreen extends StatefulWidget {
  const TradeScreen({super.key});
  @override State<TradeScreen> createState() => _TradeScreenState();
}

class _TradeScreenState extends State<TradeScreen> {
  final _priceCtrl = TextEditingController();
  String _selectedSymbol = 'VLT';
  List<Candle> _candles = [];
  bool _candlesLoading = false;

  @override
  void initState() {
    super.initState();
    _loadCandles('VLT');
  }

  @override
  void dispose() { _priceCtrl.dispose(); super.dispose(); }

  Future<void> _selectPair(String symbol) async {
    setState(() { _selectedSymbol = symbol; _candles = []; _candlesLoading = true; });
    final market = context.read<MarketState>();
    final c = await market.loadCandles(symbol);
    if (mounted) setState(() { _candles = c; _candlesLoading = false; });
  }

  Future<void> _loadCandles(String symbol) async {
    setState(() => _candlesLoading = true);
    final c = await context.read<MarketState>().loadCandles(symbol);
    if (mounted) setState(() { _candles = c; _candlesLoading = false; });
  }

  // VLT usa matching engine real; todos os outros usam o simulador do gateway
  bool get _isVLT => _selectedSymbol == 'VLT';

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (ctx, box) {
      final wide = box.maxWidth >= 1100;
      if (wide) return _wideLayout();
      return _narrowLayout();
    });
  }

  Widget _wideLayout() {
    return Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      // Left sidebar: pair list
      SizedBox(width: 210, child: _PairSidebar(
        selected: _selectedSymbol,
        onSelect: _selectPair,
      )),
      Container(width: 1, color: kBorder),

      // Main content
      Expanded(
        child: Column(children: [
          _TopBar(symbol: _selectedSymbol, isVLT: _isVLT),
          Expanded(
            child: Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              // Chart + tape
              Expanded(
                child: Column(children: [
                  Expanded(
                    flex: 3,
                    child: _PriceChart(
                      symbol: _selectedSymbol,
                      candles: _candles,
                      loading: _candlesLoading,
                      isVLT: _isVLT,
                    ),
                  ),
                  Container(height: 1, color: kBorder),
                  Expanded(
                    flex: 2,
                    child: _TapePanel(symbol: _selectedSymbol),
                  ),
                ]),
              ),
              Container(width: 1, color: kBorder),

              // Right: book + form — habilitado para todos os pares
              SizedBox(
                width: 320,
                child: _TradingPanel(
                    priceCtrl: _priceCtrl,
                    symbol: _selectedSymbol,
                    isVLT: _isVLT),
              ),
            ]),
          ),
          Container(height: 1, color: kBorder),
          SizedBox(height: 200, child: _OrdersPanel(symbol: _selectedSymbol)),
        ]),
      ),
    ]);
  }

  Widget _narrowLayout() {
    return ListView(children: [
      _TopBar(symbol: _selectedSymbol, isVLT: _isVLT),
      SizedBox(height: 50, child: _PairScrollRow(selected: _selectedSymbol, onSelect: _selectPair)),
      Container(height: 1, color: kBorder),
      SizedBox(
        height: 240,
        child: _PriceChart(symbol: _selectedSymbol, candles: _candles, loading: _candlesLoading, isVLT: _isVLT),
      ),
      Container(height: 1, color: kBorder),
      SizedBox(height: 360, child: _TradingPanel(priceCtrl: _priceCtrl, symbol: _selectedSymbol, isVLT: _isVLT)),
      Container(height: 1, color: kBorder),
      SizedBox(height: 280, child: _TapePanel(symbol: _selectedSymbol)),
      Container(height: 1, color: kBorder),
      SizedBox(height: 220, child: _OrdersPanel(symbol: _selectedSymbol)),
    ]);
  }
}

// ─── Pair sidebar ─────────────────────────────────────────────────────────────

class _PairSidebar extends StatefulWidget {
  final String selected;
  final void Function(String) onSelect;
  const _PairSidebar({required this.selected, required this.onSelect});
  @override State<_PairSidebar> createState() => _PairSidebarState();
}

class _PairSidebarState extends State<_PairSidebar> {
  final _ctrl = TextEditingController();
  String _q = '';

  @override void dispose() { _ctrl.dispose(); super.dispose(); }

  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    final coins  = market.coins.where((c) {
      if (_q.isEmpty) return true;
      return c.symbol.toLowerCase().contains(_q) || c.name.toLowerCase().contains(_q);
    }).toList();

    return Container(
      color: kSurface,
      child: Column(children: [
        // Header
        Container(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 6),
          color: kSurface,
          child: TextField(
            controller: _ctrl,
            style: const TextStyle(color: kTxt, fontSize: 12),
            onChanged: (v) => setState(() => _q = v.toLowerCase()),
            decoration: InputDecoration(
              hintText: 'Buscar par...',
              prefixIcon: const Icon(Icons.search, size: 14, color: kTxtSub),
              contentPadding: const EdgeInsets.symmetric(vertical: 6),
              filled: true,
              fillColor: kSurface2,
              border: OutlineInputBorder(borderRadius: BorderRadius.circular(6),
                  borderSide: const BorderSide(color: kBorder)),
              enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6),
                  borderSide: const BorderSide(color: kBorder)),
              focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6),
                  borderSide: const BorderSide(color: kBrand, width: 1)),
            ),
          ),
        ),
        // "Live" VLT always first
        if (_q.isEmpty) _SidebarItem(
          symbol: 'VLT', name: 'Veltra Token',
          selected: widget.selected == 'VLT',
          isLive: true,
          coin: market.coins.where((c) => c.symbol == 'VLT').firstOrNull,
          onTap: () => widget.onSelect('VLT'),
        ),
        Container(height: 1, color: kBorder.withOpacity(0.5)),
        // All others
        Expanded(
          child: ListView.builder(
            itemCount: coins.length,
            itemBuilder: (_, i) {
              final c = coins[i];
              if (c.symbol == 'VLT' && _q.isEmpty) return const SizedBox.shrink();
              return _SidebarItem(
                symbol: c.symbol, name: c.name,
                selected: widget.selected == c.symbol,
                isLive: false,
                coin: c,
                onTap: () => widget.onSelect(c.symbol),
              );
            },
          ),
        ),
      ]),
    );
  }
}

class _SidebarItem extends StatelessWidget {
  final String symbol, name;
  final bool selected, isLive;
  final MarketCoin? coin;
  final VoidCallback onTap;
  const _SidebarItem({required this.symbol, required this.name, required this.selected,
      required this.isLive, required this.coin, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final up = coin?.change24h != null ? coin!.change24h >= 0 : true;
    final c  = up ? kBuy : kSell;
    return InkWell(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        color: selected ? kBrand.withOpacity(0.08) : Colors.transparent,
        child: Row(children: [
          if (selected)
            Container(width: 2, height: 28, margin: const EdgeInsets.only(right: 8),
                decoration: BoxDecoration(color: kBrand, borderRadius: BorderRadius.circular(1),
                    boxShadow: [BoxShadow(color: kBrand.withOpacity(0.8), blurRadius: 4)])),
          // Coin initial
          Container(
            width: 24, height: 24,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(6),
              color: _coinColor(symbol).withOpacity(0.15),
            ),
            child: Center(child: Text(symbol.isNotEmpty ? symbol[0] : '?',
                style: TextStyle(fontSize: 11, fontWeight: FontWeight.w800,
                    color: _coinColor(symbol)))),
          ),
          const SizedBox(width: 7),
          Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Text(symbol, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700,
                  color: selected ? kBrand : kTxt)),
              if (isLive) ...[
                const SizedBox(width: 4),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
                  decoration: BoxDecoration(color: kBrand.withOpacity(0.2),
                      borderRadius: BorderRadius.circular(4)),
                  child: const Text('LIVE', style: TextStyle(fontSize: 8, color: kBrand,
                      fontWeight: FontWeight.w800, letterSpacing: 1)),
                ),
              ],
            ]),
            if (coin != null)
              Text('\$${_fmtShort(coin!.priceUSD)}',
                  style: const TextStyle(fontSize: 10, color: kTxtSub,
                      fontFeatures: [FontFeature.tabularFigures()])),
          ])),
          if (coin != null)
            Text('${up ? '+' : ''}${coin!.change24h.toStringAsFixed(2)}%',
                style: TextStyle(fontSize: 10, color: c, fontWeight: FontWeight.w700)),
        ]),
      ),
    );
  }

  Color _coinColor(String s) {
    int h = 0; for (final c in s.codeUnits) h = (h * 31 + c) & 0xFFFFFF;
    const cols = [Color(0xFFF7931A), Color(0xFF627EEA), Color(0xFFF3BA2F), Color(0xFF9945FF),
      Color(0xFF00AAE4), Color(0xFFE84142), Color(0xFF00D4FF), Color(0xFF02C076)];
    return cols[h % cols.length];
  }

  String _fmtShort(double v) {
    if (v >= 1000) return '${(v/1000).toStringAsFixed(1)}K';
    if (v >= 1)    return v.toStringAsFixed(2);
    if (v >= 0.01) return v.toStringAsFixed(4);
    return v.toStringAsExponential(2);
  }
}

// ─── Horizontal scroll row (mobile) ──────────────────────────────────────────

class _PairScrollRow extends StatelessWidget {
  final String selected;
  final void Function(String) onSelect;
  const _PairScrollRow({required this.selected, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    final coins = context.watch<MarketState>().coins;
    return Container(
      color: kSurface,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 8),
        itemCount: coins.length,
        separatorBuilder: (_, __) => const SizedBox(width: 4),
        itemBuilder: (_, i) {
          final c = coins[i];
          final sel = selected == c.symbol;
          return GestureDetector(
            onTap: () => onSelect(c.symbol),
            child: Container(
              alignment: Alignment.center,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: BoxDecoration(
                border: Border(bottom: BorderSide(
                    color: sel ? kBrand : Colors.transparent, width: 2)),
              ),
              child: Text(c.symbol,
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700,
                      color: sel ? kBrand : kTxtSub)),
            ),
          );
        },
      ),
    );
  }
}

// ─── Top bar ──────────────────────────────────────────────────────────────────

class _TopBar extends StatelessWidget {
  final String symbol;
  final bool isVLT;
  const _TopBar({required this.symbol, required this.isVLT});

  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    final trading = context.watch<TradingState>();
    final coin = market.coins.where((c) => c.symbol == symbol).firstOrNull;

    final priceUSD  = isVLT && trading.lastPrice > 0
        ? trading.lastPrice / 1e8
        : (coin?.priceUSD ?? 0);
    final priceBRL  = coin?.priceBRL ?? priceUSD * 5.0;
    final change24h = coin?.change24h ?? 0.0;
    final isUp      = change24h >= 0;
    final dirColor  = isVLT
        ? (trading.priceDirection > 0 ? kBuy : trading.priceDirection < 0 ? kSell : kTxtSub)
        : (isUp ? kBuy : kSell);

    return Container(
      height: 52,
      color: kSurface,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(children: [
        // Symbol + live badge
        Row(mainAxisSize: MainAxisSize.min, children: [
          _CoinDot(symbol),
          const SizedBox(width: 8),
          Text('$symbol/USDT-sim',
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: kTxt)),
          if (isVLT) ...[
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(color: kBrand.withOpacity(0.12),
                  borderRadius: BorderRadius.circular(4),
                  border: Border.all(color: kBrand.withOpacity(0.3))),
              child: const Text('LIVE', style: TextStyle(fontSize: 9, color: kBrand,
                  fontWeight: FontWeight.w900, letterSpacing: 1)),
            ),
          ],
        ]),
        const SizedBox(width: 20),

        // Price BRL
        Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('R\$ ${_fmtBRL(priceBRL)}',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w900, color: dirColor,
                  fontFeatures: const [FontFeature.tabularFigures()])),
          Text('\$${_fmtUSD(priceUSD)}',
              style: const TextStyle(fontSize: 11, color: kTxtSub,
                  fontFeatures: [FontFeature.tabularFigures()])),
        ]),
        const SizedBox(width: 24),

        // Stats — VLT mostra bid/ask do motor; demais mostram 24h + volume
        if (isVLT) ...[
          _Stat('Bid', trading.bestBid != null ? fmtAmount(trading.bestBid!) : '—', kBuy),
          const SizedBox(width: 16),
          _Stat('Ask', trading.bestAsk != null ? fmtAmount(trading.bestAsk!) : '—', kSell),
          const SizedBox(width: 16),
          _Stat('Spread', trading.spread != null ? fmtAmount(trading.spread!) : '—', kTxtSub),
        ] else ...[
          _Stat('24h', '${isUp ? '+' : ''}${change24h.toStringAsFixed(2)}%', isUp ? kBuy : kSell),
          if (coin != null) ...[
            const SizedBox(width: 16),
            _Stat('Vol', _fmtVolume(coin.volume24hUSD), kTxtSub),
            const SizedBox(width: 16),
            _Stat('MCap', _fmtVolume(coin.marketCapUSD), kTxtMuted),
          ],
        ],
        const Spacer(),

        // Saldo USDT-sim + botão depositar
        _BalanceChip(),
        const SizedBox(width: 12),

        // Account
        Row(mainAxisSize: MainAxisSize.min, children: [
          const Icon(Icons.person_outline, size: 13, color: kTxtSub),
          const SizedBox(width: 4),
          Text(context.watch<AuthState>().user?.username ?? '—',
              style: const TextStyle(fontSize: 11, color: kTxtSub)),
        ]),
      ]),
    );
  }

  String _fmtBRL(double v) {
    if (v >= 1000) return v.toStringAsFixed(2).replaceAllMapped(RegExp(r'\B(?=(\d{3})+(?!\d))'), (_) => '.');
    if (v >= 1) return v.toStringAsFixed(2);
    if (v >= 0.01) return v.toStringAsFixed(4);
    return v.toStringAsExponential(4);
  }
  String _fmtUSD(double v) {
    if (v >= 1000) return '\$${(v/1000).toStringAsFixed(1)}K';
    if (v >= 1) return v.toStringAsFixed(2);
    if (v >= 0.001) return v.toStringAsFixed(5);
    return v.toStringAsExponential(3);
  }
  String _fmtVolume(double v) {
    if (v >= 1e9) return '${(v/1e9).toStringAsFixed(1)}B';
    if (v >= 1e6) return '${(v/1e6).toStringAsFixed(1)}M';
    if (v >= 1e3) return '${(v/1e3).toStringAsFixed(1)}K';
    return v.toStringAsFixed(0);
  }
}

class _Stat extends StatelessWidget {
  final String l, v; final Color c;
  const _Stat(this.l, this.v, this.c);
  @override
  Widget build(BuildContext context) => Column(
    mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(l, style: const TextStyle(fontSize: 10, color: kTxtMuted)),
      Text(v, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: c,
          fontFeatures: const [FontFeature.tabularFigures()])),
    ],
  );
}

class _CoinDot extends StatelessWidget {
  final String s;
  const _CoinDot(this.s);
  Color _c() {
    int h = 0; for (final c in s.codeUnits) h = (h * 31 + c) & 0xFFFFFF;
    const cols = [Color(0xFFF7931A), Color(0xFF627EEA), Color(0xFFF3BA2F), Color(0xFF9945FF),
      Color(0xFF00AAE4), Color(0xFFE84142), Color(0xFF00D4FF), Color(0xFF02C076)];
    return cols[h % cols.length];
  }
  @override
  Widget build(BuildContext context) {
    final c = _c();
    return Container(
      width: 28, height: 28,
      decoration: BoxDecoration(borderRadius: BorderRadius.circular(8), color: c.withOpacity(0.15)),
      child: Center(child: Text(s.isNotEmpty ? s[0] : '?',
          style: TextStyle(fontSize: 13, fontWeight: FontWeight.w800, color: c))),
    );
  }
}

// ─── Price chart ──────────────────────────────────────────────────────────────

class _PriceChart extends StatefulWidget {
  final String symbol;
  final List<Candle> candles;
  final bool loading, isVLT;
  const _PriceChart({required this.symbol, required this.candles,
      required this.loading, required this.isVLT});
  @override State<_PriceChart> createState() => _PriceChartState();
}

class _PriceChartState extends State<_PriceChart> {
  int _tf = 50;
  static const _frames = [(10,'10'), (20,'20'), (50,'50'), (100,'100'), (200,'200')];

  List<Candle> get _visible {
    if (widget.candles.isEmpty) return [];
    return widget.candles.length > _tf
        ? widget.candles.sublist(widget.candles.length - _tf)
        : widget.candles;
  }

  @override
  Widget build(BuildContext context) {
    final market  = context.watch<MarketState>();
    final coin    = market.coins.where((c) => c.symbol == widget.symbol).firstOrNull;
    final trading = context.watch<TradingState>();
    final isUp    = widget.isVLT
        ? trading.priceDirection >= 0
        : (coin?.change24h ?? 0) >= 0;
    final c = isUp ? kBuy : kSell;

    return Container(
      color: kBg,
      child: Column(children: [
        // Toolbar
        Container(
          color: kSurface,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
          child: Row(children: [
            const Text('Gráfico de Preço',
                style: TextStyle(fontSize: 11, color: kTxtMuted, fontWeight: FontWeight.w600)),
            const Spacer(),
            Row(mainAxisSize: MainAxisSize.min,
                children: _frames.map((f) => GestureDetector(
                  onTap: () => setState(() => _tf = f.$1),
                  child: AnimatedContainer(
                    duration: const Duration(milliseconds: 120),
                    margin: const EdgeInsets.only(left: 4),
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: f.$1 == _tf ? c.withOpacity(0.12) : Colors.transparent,
                      borderRadius: BorderRadius.circular(5),
                      border: Border.all(color: f.$1 == _tf ? c.withOpacity(0.4) : Colors.transparent),
                    ),
                    child: Text(f.$2, style: TextStyle(fontSize: 11,
                        color: f.$1 == _tf ? c : kTxtSub,
                        fontWeight: f.$1 == _tf ? FontWeight.w700 : FontWeight.normal)),
                  ),
                )).toList()),
          ]),
        ),
        Container(height: 1, color: kBorder),

        // Chart
        Expanded(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(8, 8, 16, 8),
            child: widget.loading
                ? _buildShimmer()
                : _visible.isEmpty
                    ? _buildEmpty()
                    : _buildChart(_visible, c),
          ),
        ),
      ]),
    );
  }

  Widget _buildShimmer() => Container(
    decoration: BoxDecoration(color: kSurface2, borderRadius: BorderRadius.circular(8)),
    child: const Center(child: CircularProgressIndicator(color: kBrand, strokeWidth: 2)),
  );

  Widget _buildEmpty() => Center(
    child: Column(mainAxisSize: MainAxisSize.min, children: [
      Icon(Icons.candlestick_chart_outlined, size: 36, color: kTxtMuted),
      const SizedBox(height: 8),
      Text('Aguardando dados de ${widget.symbol}…',
          style: const TextStyle(color: kTxtSub, fontSize: 12)),
    ]),
  );

  Widget _buildChart(List<Candle> candles, Color c) {
    final spots  = candles.asMap().entries.map((e) => FlSpot(e.key.toDouble(), e.value.c)).toList();
    final minY   = spots.map((s) => s.y).reduce((a, b) => a < b ? a : b);
    final maxY   = spots.map((s) => s.y).reduce((a, b) => a > b ? a : b);
    final pad    = (maxY - minY) * 0.1 + 1e-10;

    return LineChart(LineChartData(
      minX: 0, maxX: (candles.length - 1).toDouble(),
      minY: minY - pad, maxY: maxY + pad,
      gridData: FlGridData(
        show: true,
        drawVerticalLine: true,
        horizontalInterval: (maxY - minY) / 4,
        verticalInterval: (candles.length / 4).ceilToDouble(),
        getDrawingHorizontalLine: (_) => FlLine(color: kBorder.withOpacity(0.5), strokeWidth: 0.5),
        getDrawingVerticalLine: (_) => FlLine(color: kBorder.withOpacity(0.3), strokeWidth: 0.5),
      ),
      borderData: FlBorderData(show: false),
      titlesData: FlTitlesData(
        leftTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        bottomTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
        rightTitles: AxisTitles(sideTitles: SideTitles(
          showTitles: true, reservedSize: 68,
          getTitlesWidget: (v, m) {
            if (v == m.min || v == m.max) return const SizedBox.shrink();
            return Padding(padding: const EdgeInsets.only(left: 6),
                child: Text(_fmtY(v), style: const TextStyle(fontSize: 9, color: kTxtMuted)));
          },
        )),
      ),
      lineTouchData: LineTouchData(
        touchTooltipData: LineTouchTooltipData(
          getTooltipColor: (_) => kSurface2,
          getTooltipItems: (spots) => spots.map((s) => LineTooltipItem(
            _fmtY(s.y),
            TextStyle(color: c, fontWeight: FontWeight.w700, fontSize: 12),
          )).toList(),
        ),
      ),
      lineBarsData: [LineChartBarData(
        spots: spots, isCurved: true, curveSmoothness: 0.25,
        color: c, barWidth: 2,
        dotData: const FlDotData(show: false),
        belowBarData: BarAreaData(show: true,
          gradient: LinearGradient(begin: Alignment.topCenter, end: Alignment.bottomCenter,
              colors: [c.withOpacity(0.18), c.withOpacity(0.0)])),
      )],
    ));
  }

  String _fmtY(double v) {
    if (v >= 1e6) return '\$${(v/1e6).toStringAsFixed(2)}M';
    if (v >= 1e3) return '\$${(v/1e3).toStringAsFixed(2)}K';
    if (v >= 1)   return '\$${v.toStringAsFixed(2)}';
    if (v >= 0.001) return '\$${v.toStringAsFixed(5)}';
    return '\$${v.toStringAsExponential(2)}';
  }
}

// ─── Live trading panel (only for VLT) ───────────────────────────────────────

/// Painel de trading unificado: book real (VLT) ou sintético (demais pares) + form.
class _TradingPanel extends StatelessWidget {
  final TextEditingController priceCtrl;
  final String symbol;
  final bool isVLT;
  const _TradingPanel({required this.priceCtrl, required this.symbol, required this.isVLT});

  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    final coin = market.coins.where((c) => c.symbol == symbol).firstOrNull;

    return Column(children: [
      // Header
      Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        color: kSurface,
        child: Row(children: [
          Text('Order Book', style: const TextStyle(fontSize: 11, color: kTxtSub, fontWeight: FontWeight.w600)),
          const Spacer(),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
            decoration: BoxDecoration(color: kBrand.withOpacity(0.1), borderRadius: BorderRadius.circular(4)),
            child: Text('$symbol/USDT-sim', style: const TextStyle(fontSize: 9, color: kBrand, fontWeight: FontWeight.w700)),
          ),
        ]),
      ),
      Container(height: 1, color: kBorder),
      Expanded(
        flex: 5,
        child: isVLT
            ? _OrderBookView(onPriceTap: (p) => priceCtrl.text = fmtAmount(p, minDecimals: 0))
            : _SyntheticBookView(symbol: symbol, coin: coin,
                onPriceTap: (p) => priceCtrl.text = p.toStringAsFixed(p < 1 ? 6 : 2)),
      ),
      Container(height: 1, color: kBorder),
      Expanded(flex: 7, child: _OrderForm(priceCtrl: priceCtrl, symbol: symbol, isVLT: isVLT)),
    ]);
  }
}

// ─── Synthetic book (non-VLT pairs) ──────────────────────────────────────────

class _SyntheticBookView extends StatelessWidget {
  final String symbol;
  final MarketCoin? coin;
  final void Function(double) onPriceTap;
  const _SyntheticBookView({required this.symbol, required this.coin, required this.onPriceTap});

  @override
  Widget build(BuildContext context) {
    final price = coin?.priceUSD ?? 0.0;
    if (price == 0) {
      return const Center(child: Text('Aguardando preço…', style: TextStyle(color: kTxtMuted, fontSize: 11)));
    }

    // Gera 5 níveis de asks (acima do preço) e 5 bids (abaixo)
    final factors = [0.001, 0.003, 0.006, 0.010, 0.015];
    final asks = factors.map((f) => (price * (1 + f), _qty(symbol, f, false))).toList();
    final bids = factors.map((f) => (price * (1 - f), _qty(symbol, f, true))).toList();
    final maxQ = [...asks, ...bids].map((e) => e.$2).reduce((a, b) => a > b ? a : b);

    return Column(children: [
      // Header
      Padding(padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          child: Row(children: [
            Expanded(child: _BH('USDT-sim')),
            Expanded(child: _BH(symbol, right: true)),
          ])),
      const Divider(height: 1),
      // Asks reversed
      Expanded(child: ListView(reverse: true,
        children: asks.map((e) => _SynRow(
          price: e.$1, qty: e.$2, maxQty: maxQ,
          color: kSell, onTap: onPriceTap,
        )).toList(),
      )),
      // Mid price
      Container(
        padding: const EdgeInsets.symmetric(vertical: 4),
        color: kSurface2,
        child: Center(
          child: Text(_fmtP(price),
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w900, color: kTxtSub,
                  fontFeatures: [FontFeature.tabularFigures()])),
        ),
      ),
      // Bids
      Expanded(child: ListView(
        children: bids.map((e) => _SynRow(
          price: e.$1, qty: e.$2, maxQty: maxQ,
          color: kBuy, onTap: onPriceTap,
        )).toList(),
      )),
    ]);
  }

  double _qty(String sym, double factor, bool isBid) {
    int h = 0; for (final c in sym.codeUnits) h = (h * 31 + c) & 0xFFFF;
    final base = 0.5 + (h % 100) / 100.0;
    final mult = isBid ? 1.2 : 0.9;
    return base * mult * (1.0 - factor * 5);
  }

  String _fmtP(double v) {
    if (v >= 1000) return '\$${v.toStringAsFixed(2)}';
    if (v >= 1) return '\$${v.toStringAsFixed(4)}';
    return '\$${v.toStringAsExponential(4)}';
  }
}

class _SynRow extends StatelessWidget {
  final double price, qty, maxQty;
  final Color color;
  final void Function(double) onTap;
  const _SynRow({required this.price, required this.qty, required this.maxQty, required this.color, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final frac = (qty / maxQty).clamp(0.02, 1.0);
    final pStr = price >= 1000 ? price.toStringAsFixed(2)
        : price >= 1 ? price.toStringAsFixed(4)
        : price.toStringAsExponential(3);
    return InkWell(
      onTap: () => onTap(price),
      child: SizedBox(height: 17, child: Stack(children: [
        Positioned.fill(child: Align(alignment: Alignment.centerRight,
            child: FractionallySizedBox(widthFactor: frac,
                child: Container(color: color.withOpacity(0.08))))),
        Padding(padding: const EdgeInsets.symmetric(horizontal: 10),
            child: Row(children: [
              Expanded(child: Text(pStr,
                  style: TextStyle(fontSize: 11, color: color,
                      fontFeatures: const [FontFeature.tabularFigures()]))),
              Expanded(child: Text(qty.toStringAsFixed(4),
                  textAlign: TextAlign.right,
                  style: const TextStyle(fontSize: 11, color: kTxt,
                      fontFeatures: [FontFeature.tabularFigures()]))),
            ])),
      ])),
    );
  }
}

// ─── Order book view ──────────────────────────────────────────────────────────

class _OrderBookView extends StatelessWidget {
  final void Function(int) onPriceTap;
  const _OrderBookView({required this.onPriceTap});

  @override
  Widget build(BuildContext context) {
    final t = context.watch<TradingState>();
    num maxCum = 1, cum = 0;
    final askRows = <(BookLevel, num)>[];
    for (final l in t.asks) { cum += l.quantity; askRows.add((l, cum)); }
    maxCum = cum > maxCum ? cum : maxCum; cum = 0;
    final bidRows = <(BookLevel, num)>[];
    for (final l in t.bids) { cum += l.quantity; bidRows.add((l, cum)); }
    maxCum = cum > maxCum ? cum : maxCum;

    return Container(
      color: kSurface,
      child: Column(children: [
        Padding(padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            child: Row(children: [
              Expanded(child: _BH(t.quoteAsset)),
              Expanded(child: _BH(t.baseAsset, right: true)),
            ])),
        const Divider(height: 1),
        Expanded(child: ListView.builder(
          reverse: true, itemCount: askRows.length,
          itemBuilder: (_, i) => _BookRow(level: askRows[i].$1,
              cum: askRows[i].$2 / maxCum, color: kSell, onTap: onPriceTap),
        )),
        // Mid price
        Container(
          padding: const EdgeInsets.symmetric(vertical: 4),
          color: kSurface2,
          child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            Text(
              t.lastPrice == 0 ? '—' : fmtAmount(t.lastPrice),
              style: TextStyle(
                fontSize: 14, fontWeight: FontWeight.w900,
                color: t.priceDirection >= 0 ? kBuy : kSell,
                fontFeatures: const [FontFeature.tabularFigures()],
              ),
            ),
            if (t.priceDirection != 0)
              Icon(t.priceDirection > 0 ? Icons.north : Icons.south, size: 11,
                  color: t.priceDirection > 0 ? kBuy : kSell),
          ]),
        ),
        Expanded(child: ListView.builder(
          itemCount: bidRows.length,
          itemBuilder: (_, i) => _BookRow(level: bidRows[i].$1,
              cum: bidRows[i].$2 / maxCum, color: kBuy, onTap: onPriceTap),
        )),
      ]),
    );
  }
}

class _BH extends StatelessWidget {
  final String t; final bool right;
  const _BH(this.t, {this.right = false});
  @override
  Widget build(BuildContext context) => Text(t,
      textAlign: right ? TextAlign.right : TextAlign.left,
      style: const TextStyle(fontSize: 9, color: kTxtMuted, fontWeight: FontWeight.w600));
}

class _BookRow extends StatelessWidget {
  final BookLevel level; final num cum; final Color color; final void Function(int) onTap;
  const _BookRow({required this.level, required this.cum, required this.color, required this.onTap});
  @override
  Widget build(BuildContext context) => InkWell(
    onTap: () => onTap(level.price),
    child: SizedBox(height: 17, child: Stack(children: [
      Positioned.fill(child: Align(
        alignment: Alignment.centerRight,
        child: FractionallySizedBox(
          widthFactor: cum.clamp(0.02, 1.0).toDouble(),
          child: Container(color: color.withOpacity(0.08)),
        ),
      )),
      Padding(padding: const EdgeInsets.symmetric(horizontal: 10), child: Row(children: [
        Expanded(child: Text(fmtAmount(level.price),
            style: TextStyle(fontSize: 11, color: color,
                fontFeatures: const [FontFeature.tabularFigures()]))),
        Expanded(child: Text(fmtAmount(level.quantity, minDecimals: 0, maxDecimals: 4),
            textAlign: TextAlign.right,
            style: const TextStyle(fontSize: 11, color: kTxt,
                fontFeatures: [FontFeature.tabularFigures()]))),
      ])),
    ])),
  );
}

// ─── Order form ───────────────────────────────────────────────────────────────

class _OrderForm extends StatefulWidget {
  final TextEditingController priceCtrl;
  final String symbol;
  final bool isVLT;
  const _OrderForm({required this.priceCtrl, required this.symbol, required this.isVLT});
  @override State<_OrderForm> createState() => _OrderFormState();
}

class _OrderFormState extends State<_OrderForm> with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  String _type = 'limit', _tif = 'gtc';
  final _qtyCtrl = TextEditingController();
  bool _sending = false;

  @override void initState() { super.initState(); _tabs = TabController(length: 2, vsync: this); }
  @override void dispose() { _tabs.dispose(); _qtyCtrl.dispose(); super.dispose(); }

  String get _side => _tabs.index == 0 ? 'buy' : 'sell';
  bool get _isBuy => _side == 'buy';

  double? get _total {
    final p = double.tryParse(widget.priceCtrl.text.replaceAll(',', '.'));
    final q = double.tryParse(_qtyCtrl.text.replaceAll(',', '.'));
    if (p == null || q == null) return null;
    return p * q;
  }

  Future<void> _submit() async {
    final t = context.read<TradingState>();
    setState(() => _sending = true);
    // Para pares não-VLT: o gateway usa o preço de mercado real como fill price
    final ok = await t.placeOrder(
      pair: '${widget.symbol}/USDT-sim',
      side: _side, type: _type,
      quantity: _qtyCtrl.text.trim().replaceAll(',', '.'),
      price: _type == 'limit' ? widget.priceCtrl.text.trim().replaceAll(',', '.') : '',
      timeInForce: _type == 'market' ? 'ioc' : _tif,
    );
    setState(() => _sending = false);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(ok ? 'Ordem enviada' : (t.lastError ?? 'Erro')),
      backgroundColor: ok ? kBuy.withOpacity(0.15) : kSell.withOpacity(0.15),
      duration: const Duration(seconds: 2),
    ));
    t.clearMessages();
  }

  @override
  Widget build(BuildContext context) {
    final t = context.watch<TradingState>();
    final actionColor = _isBuy ? kBuy : kSell;
    // Para pares não-VLT, baseAsset = symbol, quoteAsset = USDT-sim
    final baseAsset  = widget.isVLT ? t.baseAsset  : widget.symbol;
    final quoteAsset = widget.isVLT ? t.quoteAsset : 'USDT-sim';

    return Container(
      color: kSurface,
      child: Column(children: [
        // Buy/Sell tabs
        Container(
          height: 36,
          margin: const EdgeInsets.fromLTRB(10, 8, 10, 0),
          decoration: BoxDecoration(color: kBorder.withOpacity(0.4), borderRadius: BorderRadius.circular(7)),
          child: TabBar(
            controller: _tabs,
            onTap: (_) => setState(() {}),
            indicator: BoxDecoration(color: actionColor, borderRadius: BorderRadius.circular(6),
                boxShadow: [BoxShadow(color: actionColor.withOpacity(0.4), blurRadius: 6)]),
            indicatorSize: TabBarIndicatorSize.tab,
            dividerColor: Colors.transparent,
            labelColor: Colors.black,
            unselectedLabelColor: kTxtSub,
            labelStyle: const TextStyle(fontWeight: FontWeight.w800, fontSize: 12),
            tabs: const [Tab(text: 'Comprar'), Tab(text: 'Vender')],
          ),
        ),

        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              // Balances — simulados para pares não-VLT (usa projeção do VeltraState)
              Row(children: [
                _Bal(baseAsset, t.balanceOf(baseAsset)),
                const SizedBox(width: 12),
                _Bal(quoteAsset, t.balanceOf(quoteAsset)),
                const Spacer(),
                _FaucetInline(symbol: widget.symbol, isVLT: widget.isVLT),
              ]),
              const SizedBox(height: 10),

              // Type + TIF
              Row(children: [
                Expanded(child: DropdownButtonFormField<String>(
                  value: _type,
                  dropdownColor: kSurface2,
                  style: const TextStyle(color: kTxt, fontSize: 12),
                  decoration: const InputDecoration(labelText: 'Tipo', isDense: true),
                  items: const [
                    DropdownMenuItem(value: 'limit', child: Text('Limit')),
                    DropdownMenuItem(value: 'market', child: Text('Market')),
                  ],
                  onChanged: (v) => setState(() => _type = v ?? 'limit'),
                )),
                const SizedBox(width: 8),
                Expanded(child: DropdownButtonFormField<String>(
                  value: _type == 'market' ? 'ioc' : _tif,
                  dropdownColor: kSurface2,
                  style: const TextStyle(color: kTxt, fontSize: 12),
                  decoration: const InputDecoration(labelText: 'Validade', isDense: true),
                  items: const [
                    DropdownMenuItem(value: 'gtc', child: Text('GTC')),
                    DropdownMenuItem(value: 'ioc', child: Text('IOC')),
                    DropdownMenuItem(value: 'fok', child: Text('FOK')),
                  ],
                  onChanged: _type == 'market' ? null : (v) => setState(() => _tif = v ?? 'gtc'),
                )),
              ]),
              const SizedBox(height: 10),

              if (_type == 'limit') ...[
                TextField(
                  controller: widget.priceCtrl,
                  style: const TextStyle(color: kTxt, fontSize: 13),
                  decoration: InputDecoration(
                    labelText: 'Preço',
                    suffixText: quoteAsset,
                    suffixStyle: const TextStyle(color: kTxtSub, fontSize: 11),
                  ),
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                ),
                const SizedBox(height: 8),
              ],

              TextField(
                controller: _qtyCtrl,
                style: const TextStyle(color: kTxt, fontSize: 13),
                decoration: InputDecoration(
                  labelText: 'Quantidade',
                  suffixText: baseAsset,
                  suffixStyle: const TextStyle(color: kTxtSub, fontSize: 11),
                ),
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                onChanged: (_) => setState(() {}),
              ),
              const SizedBox(height: 8),

              if (_type == 'limit')
                Row(children: [
                  const Text('Total ≈', style: TextStyle(fontSize: 11, color: kTxtMuted)),
                  const Spacer(),
                  Text(_total == null ? '—' : '${_total!.toStringAsFixed(2)} $quoteAsset',
                      style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: kTxt,
                          fontFeatures: [FontFeature.tabularFigures()])),
                ]),
              const SizedBox(height: 10),

              Container(
                decoration: BoxDecoration(borderRadius: BorderRadius.circular(7),
                    boxShadow: [BoxShadow(color: actionColor.withOpacity(0.3), blurRadius: 10)]),
                child: FilledButton(
                  onPressed: _sending ? null : _submit,
                  style: FilledButton.styleFrom(
                    backgroundColor: actionColor, foregroundColor: Colors.black,
                    minimumSize: const Size(double.infinity, 42),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(7)),
                  ),
                  child: Text(_sending ? 'Enviando…' : '${_isBuy ? 'Comprar' : 'Vender'} $baseAsset',
                      style: const TextStyle(fontWeight: FontWeight.w900, fontSize: 13)),
                ),
              ),
            ]),
          ),
        ),
      ]),
    );
  }
}

class _Bal extends StatelessWidget {
  final String asset; final num amount;
  const _Bal(this.asset, this.amount);
  @override
  Widget build(BuildContext context) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
    Text(asset, style: const TextStyle(fontSize: 9, color: kTxtMuted)),
    Text(fmtAmount(amount, minDecimals: 0, maxDecimals: 4),
        style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: kTxt,
            fontFeatures: [FontFeature.tabularFigures()])),
  ]);
}

class _FaucetInline extends StatelessWidget {
  final String symbol;
  final bool isVLT;
  const _FaucetInline({required this.symbol, required this.isVLT});

  @override
  Widget build(BuildContext context) => GestureDetector(
    onTap: () => _show(context),
    child: Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(color: kBrand.withOpacity(0.1), borderRadius: BorderRadius.circular(6),
          border: Border.all(color: kBrand.withOpacity(0.3))),
      child: const Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(Icons.water_drop_outlined, size: 11, color: kBrand),
        SizedBox(width: 4),
        Text('Faucet', style: TextStyle(fontSize: 10, color: kBrand, fontWeight: FontWeight.w700)),
      ]),
    ),
  );

  void _show(BuildContext context) {
    final t = context.read<TradingState>();
    final ac = TextEditingController(text: '1000');
    // Para pares não-VLT: oferecer USDT-sim e o ativo em questão
    final assets = isVLT ? [t.quoteAsset, t.baseAsset] : ['USDT-sim', symbol];
    String asset = assets.first;
    showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(builder: (ctx, ss) => AlertDialog(
        backgroundColor: kSurface2,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12), side: const BorderSide(color: kBorder)),
        title: const Text('Faucet — emitir saldo', style: TextStyle(color: kTxt, fontSize: 16)),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          DropdownButtonFormField<String>(
            value: asset, dropdownColor: kSurface2, style: const TextStyle(color: kTxt),
            decoration: const InputDecoration(labelText: 'Ativo'),
            items: assets.map((a) => DropdownMenuItem(value: a, child: Text(a))).toList(),
            onChanged: (v) => ss(() => asset = v ?? asset),
          ),
          const SizedBox(height: 12),
          TextField(controller: ac, style: const TextStyle(color: kTxt),
              decoration: const InputDecoration(labelText: 'Quantidade'),
              keyboardType: const TextInputType.numberWithOptions(decimal: true)),
        ]),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancelar', style: TextStyle(color: kTxtSub))),
          FilledButton(
            onPressed: () async {
              final ok = await t.requestFaucet(asset, ac.text.trim());
              if (ok && ctx.mounted) Navigator.pop(ctx);
            },
            child: const Text('Emitir'),
          ),
        ],
      )),
    );
  }
}

// ─── Trade tape ───────────────────────────────────────────────────────────────

class _TapePanel extends StatelessWidget {
  final String symbol;
  const _TapePanel({required this.symbol});

  @override
  Widget build(BuildContext context) {
    final t = context.watch<TradingState>();
    // Filtra trades do par atual
    final pair = '$symbol/USDT-sim';
    final trades = t.trades.where((tr) => tr.pair == pair || symbol == 'VLT').toList();

    return Container(
      color: kSurface,
      child: Column(children: [
        Padding(padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
            child: Row(children: [
              Expanded(child: _H('Preço')),
              Expanded(child: _H('Qtd', right: true)),
              Expanded(child: _H('Hora', right: true)),
            ])),
        const Divider(height: 1),
        Expanded(
          child: trades.isEmpty
              ? const Center(child: Text('Aguardando trades…', style: TextStyle(color: kTxtMuted, fontSize: 11)))
              : ListView.builder(
                  itemCount: trades.length,
                  itemBuilder: (_, i) {
                    final tr = trades[i];
                    final c  = tr.takerSide == 'buy' ? kBuy : kSell;
                    final ts = DateTime.fromMillisecondsSinceEpoch(tr.timestampMs);
                    final priceStr = tr.price > 1e10
                        ? (tr.price / 1e8).toStringAsFixed(2)
                        : fmtAmount(tr.price);
                    return Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
                      child: Row(children: [
                        Expanded(child: Text(priceStr,
                            style: TextStyle(fontSize: 11, color: c,
                                fontFeatures: const [FontFeature.tabularFigures()]))),
                        Expanded(child: Text(fmtAmount(tr.quantity, minDecimals: 0, maxDecimals: 6),
                            textAlign: TextAlign.right,
                            style: const TextStyle(fontSize: 11, color: kTxt,
                                fontFeatures: [FontFeature.tabularFigures()]))),
                        Expanded(child: Text(
                            '${ts.hour.toString().padLeft(2,'0')}:${ts.minute.toString().padLeft(2,'0')}:${ts.second.toString().padLeft(2,'0')}',
                            textAlign: TextAlign.right,
                            style: const TextStyle(fontSize: 10, color: kTxtMuted))),
                      ]),
                    );
                  }),
        ),
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
      style: const TextStyle(fontSize: 9, color: kTxtMuted, fontWeight: FontWeight.w600));
}

// ─── Read-only panel (non-VLT pairs) ─────────────────────────────────────────

class _ReadOnlyPanel extends StatelessWidget {
  final String symbol;
  const _ReadOnlyPanel({required this.symbol});

  @override
  Widget build(BuildContext context) {
    final coin = context.watch<MarketState>().coins.where((c) => c.symbol == symbol).firstOrNull;

    return Container(
      color: kSurface,
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: kBrand.withOpacity(0.05),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: kBrand.withOpacity(0.2)),
          ),
          child: Row(children: [
            const Icon(Icons.info_outline, size: 16, color: kBrand),
            const SizedBox(width: 8),
            Expanded(child: Text(
              '$symbol não possui trading habilitado.\nApenas VLT/USDT-sim tem matching engine.',
              style: const TextStyle(fontSize: 12, color: kTxtSub, height: 1.4),
            )),
          ]),
        ),
        const SizedBox(height: 16),
        if (coin != null) ...[
          _InfoRow('Preço USD', '\$${_fmt(coin.priceUSD)}'),
          _InfoRow('Preço BRL', 'R\$ ${_fmtBRL(coin.priceBRL)}'),
          _InfoRow('Variação 24h', '${coin.isUp ? '+' : ''}${coin.change24h.toStringAsFixed(2)}%',
              valueColor: coin.isUp ? kBuy : kSell),
          _InfoRow('Volume 24h', '\$${_fmtVol(coin.volume24hUSD)}'),
          _InfoRow('Market Cap', '\$${_fmtVol(coin.marketCapUSD)}'),
        ],
      ]),
    );
  }

  String _fmt(double v) {
    if (v >= 1000) return '${(v/1000).toStringAsFixed(2)}K';
    if (v >= 1) return v.toStringAsFixed(4);
    if (v >= 0.0001) return v.toStringAsFixed(6);
    return v.toStringAsExponential(3);
  }
  String _fmtBRL(double v) {
    if (v >= 1000) return v.toStringAsFixed(2).replaceAllMapped(RegExp(r'\B(?=(\d{3})+(?!\d))'), (_) => '.');
    if (v >= 1) return v.toStringAsFixed(2);
    return v.toStringAsFixed(6);
  }
  String _fmtVol(double v) {
    if (v >= 1e12) return '${(v/1e12).toStringAsFixed(2)}T';
    if (v >= 1e9) return '${(v/1e9).toStringAsFixed(2)}B';
    if (v >= 1e6) return '${(v/1e6).toStringAsFixed(2)}M';
    return '${(v/1e3).toStringAsFixed(1)}K';
  }
}

class _InfoRow extends StatelessWidget {
  final String label, value; final Color? valueColor;
  const _InfoRow(this.label, this.value, {this.valueColor});
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 6),
    child: Row(children: [
      Text(label, style: const TextStyle(fontSize: 12, color: kTxtSub)),
      const Spacer(),
      Text(value, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700,
          color: valueColor ?? kTxt, fontFeatures: const [FontFeature.tabularFigures()])),
    ]),
  );
}

// ─── Market info panel (bottom, non-VLT) ──────────────────────────────────────

class _MarketInfoPanel extends StatelessWidget {
  final String symbol;
  const _MarketInfoPanel({required this.symbol});
  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    // Show top movers
    final coins = market.coins.where((c) => c.symbol != symbol).take(6).toList();
    return Container(
      color: kSurface,
      child: Column(children: [
        const Padding(
          padding: EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          child: Row(children: [
            Text('Outros mercados', style: TextStyle(fontSize: 11, color: kTxtSub, fontWeight: FontWeight.w600)),
          ]),
        ),
        const Divider(height: 1),
        Expanded(child: ListView.builder(
          itemCount: coins.length,
          itemBuilder: (_, i) {
            final c = coins[i]; final up = c.change24h >= 0;
            return Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
              child: Row(children: [
                Text(c.symbol, style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: kTxt)),
                const Spacer(),
                Text('\$${c.priceUSD >= 1 ? c.priceUSD.toStringAsFixed(2) : c.priceUSD.toStringAsExponential(3)}',
                    style: const TextStyle(fontSize: 11, color: kTxtSub,
                        fontFeatures: [FontFeature.tabularFigures()])),
                const SizedBox(width: 8),
                Text('${up ? '+' : ''}${c.change24h.toStringAsFixed(2)}%',
                    style: TextStyle(fontSize: 11, color: up ? kBuy : kSell, fontWeight: FontWeight.w700)),
              ]),
            );
          },
        )),
      ]),
    );
  }
}

// ─── Orders panel ─────────────────────────────────────────────────────────────

class _OrdersPanel extends StatelessWidget {
  final String symbol;
  const _OrdersPanel({required this.symbol});

  @override
  Widget build(BuildContext context) {
    final t = context.watch<TradingState>();
    final pair = '$symbol/USDT-sim';
    // Filtra por par (VLT usa o getter padrão do matching engine)
    final open    = symbol == 'VLT' ? t.openOrders
        : t.openOrders.where((o) => o.pair == pair).toList();
    final history = symbol == 'VLT' ? t.orderHistory
        : t.orderHistory.where((o) => o.pair == pair).toList();

    return Container(
      color: kSurface,
      child: DefaultTabController(length: 2, child: Column(children: [
        TabBar(tabs: [
          Tab(text: 'Ordens abertas (${open.length})'),
          const Tab(text: 'Histórico'),
        ]),
        Expanded(child: TabBarView(children: [
          _OTab(orders: open, canCancel: symbol == 'VLT'),
          _OTab(orders: history, canCancel: false),
        ])),
      ])),
    );
  }
}

class _OTab extends StatelessWidget {
  final List<OrderInfo> orders; final bool canCancel;
  const _OTab({required this.orders, required this.canCancel});
  @override
  Widget build(BuildContext context) {
    if (orders.isEmpty) {
      return const Center(child: Text('Nenhuma ordem', style: TextStyle(color: kTxtMuted, fontSize: 12)));
    }
    return ListView.separated(
      itemCount: orders.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, i) {
        final o = orders[i]; final c = o.side == 'buy' ? kBuy : kSell;
        return Container(
          color: kSurface,
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          child: Row(children: [
            Container(width: 3, height: 30, decoration: BoxDecoration(color: c, borderRadius: BorderRadius.circular(2))),
            const SizedBox(width: 10),
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text('${o.side == 'buy' ? 'COMPRA' : 'VENDA'} ${o.type.toUpperCase()}',
                  style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w800)),
              Text(o.type == 'market'
                  ? '${(o.quantity/1e8).toStringAsFixed(4)} VLT'
                  : '${(o.quantity/1e8).toStringAsFixed(4)} @ ${(o.price/1e8).toStringAsFixed(2)}',
                  style: const TextStyle(fontSize: 10, color: kTxtSub)),
            ])),
            _SPill(o.status),
            if (canCancel) ...[
              const SizedBox(width: 6),
              GestureDetector(
                onTap: () => context.read<TradingState>().cancelOrder(o),
                child: Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(color: kSell.withOpacity(0.1), borderRadius: BorderRadius.circular(5)),
                  child: const Icon(Icons.close, size: 12, color: kSell),
                ),
              ),
            ],
          ]),
        );
      },
    );
  }
}

class _SPill extends StatelessWidget {
  final String s; const _SPill(this.s);
  @override
  Widget build(BuildContext context) {
    final c = switch (s) {
      'filled' => kBuy, 'partially_filled' => const Color(0xFFF0B90B),
      'canceled' => kTxtSub, 'rejected' => kSell, _ => kBrand,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
      decoration: BoxDecoration(color: c.withOpacity(0.1), borderRadius: BorderRadius.circular(20)),
      child: Text(s, style: TextStyle(fontSize: 9, color: c, fontWeight: FontWeight.w700)),
    );
  }
}


// ─── Balance chip (header) ────────────────────────────────────────────────────

class _BalanceChip extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final bal = context.watch<BalanceState>();
    final usdt = bal.balanceOf('USDT-sim');
    final fmtUsdt = usdt >= 1000
        ? 'R\$ ${(usdt * 5 / 1000).toStringAsFixed(1)}K'
        : 'R\$ ${(usdt * 5).toStringAsFixed(2)}';

    return GestureDetector(
      onTap: () => showDialog(context: context, builder: (_) => const DepositDialog()),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: kSurface2,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: kBorder),
        ),
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          const Icon(Icons.account_balance_wallet_outlined, size: 13, color: kTxtSub),
          const SizedBox(width: 5),
          Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [
            const Text('USDT-sim', style: TextStyle(fontSize: 9, color: kTxtMuted)),
            Text(usdt == 0 ? '—' : fmtUsdt,
                style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: kTxt,
                    fontFeatures: [FontFeature.tabularFigures()])),
          ]),
          const SizedBox(width: 6),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            decoration: BoxDecoration(color: kBrand.withOpacity(0.12), borderRadius: BorderRadius.circular(6)),
            child: const Text('+ Depositar', style: TextStyle(fontSize: 9, color: kBrand, fontWeight: FontWeight.w700)),
          ),
        ]),
      ),
    );
  }
}
