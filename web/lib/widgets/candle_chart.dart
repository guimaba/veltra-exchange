import 'package:flutter/material.dart';

import '../fmt.dart';
import '../market_state.dart';
import '../theme.dart';

/// Gráfico de velas (candlestick) com grade, eixo de preço à direita e wicks.
/// Desenhado via CustomPaint (fl_chart 0.69 não tem candlestick nativo).
class CandleChart extends StatelessWidget {
  final List<Candle> candles;
  final Color up;
  final Color down;
  const CandleChart({
    super.key,
    required this.candles,
    this.up = kBuy,
    this.down = kSell,
  });

  @override
  Widget build(BuildContext context) {
    if (candles.isEmpty) {
      return const Center(
        child: Text('Sem dados', style: TextStyle(color: kTxtSub, fontSize: 12)),
      );
    }
    return CustomPaint(
      painter: _CandlePainter(candles: candles, up: up, down: down),
      size: Size.infinite,
    );
  }
}

class _CandlePainter extends CustomPainter {
  final List<Candle> candles;
  final Color up;
  final Color down;
  _CandlePainter({required this.candles, required this.up, required this.down});

  static const double _rightPad = 58; // espaço p/ rótulos de preço
  static const double _topPad = 8;
  static const double _bottomPad = 18; // espaço p/ rótulos de tempo
  static const int _hLines = 5;

  @override
  void paint(Canvas canvas, Size size) {
    final chartW = size.width - _rightPad;
    final chartH = size.height - _topPad - _bottomPad;
    if (chartW <= 0 || chartH <= 0) return;

    double minLow = candles.first.l, maxHigh = candles.first.h;
    for (final c in candles) {
      if (c.l < minLow) minLow = c.l;
      if (c.h > maxHigh) maxHigh = c.h;
    }
    final pad = (maxHigh - minLow) * 0.08 + 1e-9;
    minLow -= pad;
    maxHigh += pad;
    final range = (maxHigh - minLow).abs() < 1e-12 ? 1.0 : (maxHigh - minLow);

    double yOf(double price) =>
        _topPad + chartH - ((price - minLow) / range) * chartH;

    final gridPaint = Paint()
      ..color = kBorder.withOpacity(0.45)
      ..strokeWidth = 0.5;
    final textStyle = const TextStyle(color: kTxtMuted, fontSize: 9);

    // --- Grade horizontal + rótulos de preço ---
    for (int i = 0; i <= _hLines; i++) {
      final t = i / _hLines;
      final y = _topPad + chartH * t;
      canvas.drawLine(Offset(0, y), Offset(chartW, y), gridPaint);
      final price = maxHigh - range * t;
      final tp = TextPainter(
        text: TextSpan(text: Fmt.price(price), style: textStyle),
        textDirection: TextDirection.ltr,
      )..layout(maxWidth: _rightPad);
      tp.paint(canvas, Offset(chartW + 4, y - tp.height / 2));
    }

    // --- Grade vertical (até ~6 divisões) ---
    final vDiv = candles.length <= 1 ? 1 : (candles.length / 6).ceil();
    for (int i = 0; i < candles.length; i += vDiv) {
      final x = chartW * (i / (candles.length - 1).clamp(1, 1 << 30));
      canvas.drawLine(
          Offset(x, _topPad), Offset(x, _topPad + chartH), gridPaint);
    }

    // --- Velas ---
    final slot = chartW / candles.length;
    final bodyW = (slot * 0.62).clamp(1.0, 14.0);
    for (int i = 0; i < candles.length; i++) {
      final c = candles[i];
      final cx = slot * i + slot / 2;
      final isUp = c.c >= c.o;
      final color = isUp ? up : down;
      final paint = Paint()
        ..color = color
        ..strokeWidth = 1.0;

      // Wick (máx-mín)
      canvas.drawLine(Offset(cx, yOf(c.h)), Offset(cx, yOf(c.l)), paint);

      // Corpo (abertura-fechamento)
      final yo = yOf(c.o), yc = yOf(c.c);
      final top = yo < yc ? yo : yc;
      final bot = yo < yc ? yc : yo;
      final h = (bot - top).clamp(1.0, double.infinity);
      final rect = Rect.fromLTWH(cx - bodyW / 2, top, bodyW, h);
      canvas.drawRRect(
        RRect.fromRectAndRadius(rect, const Radius.circular(1)),
        Paint()..color = color,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _CandlePainter old) =>
      old.candles != candles || old.up != up || old.down != down;
}
