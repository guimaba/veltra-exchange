import 'package:flutter/material.dart';

import '../theme.dart';

class VeltraLogo extends StatelessWidget {
  const VeltraLogo({super.key});

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Container(
        width: 64,
        height: 64,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(18),
          gradient: const LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [kBrand2, kBrand],
          ),
          boxShadow: [
            BoxShadow(color: kBrand.withOpacity(0.35), blurRadius: 24),
          ],
        ),
        child: const Icon(Icons.hub_outlined, color: Colors.white, size: 32),
      ),
      const SizedBox(height: 16),
      ShaderMask(
        shaderCallback: (bounds) => const LinearGradient(
          colors: [kBrand2, kBrand],
        ).createShader(bounds),
        child: const Text('VELTRA',
            style: TextStyle(
                fontSize: 26,
                fontWeight: FontWeight.w900,
                color: Colors.white,
                letterSpacing: 6)),
      ),
      const Text('EXCHANGE',
          style: TextStyle(
              fontSize: 11,
              color: kTxtMuted,
              letterSpacing: 5,
              fontWeight: FontWeight.w500)),
    ]);
  }
}

class GlowButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String label;
  const GlowButton({super.key, required this.onPressed, required this.loading, required this.label});

  @override
  Widget build(BuildContext context) => Container(
    decoration: BoxDecoration(
      borderRadius: BorderRadius.circular(8),
      boxShadow: onPressed != null
          ? [BoxShadow(color: kBrand.withOpacity(0.4), blurRadius: 16, spreadRadius: 0)]
          : null,
    ),
    child: FilledButton(
      onPressed: onPressed,
      style: FilledButton.styleFrom(
        backgroundColor: kBrand,
        foregroundColor: kBg,
        minimumSize: const Size(double.infinity, 50),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      child: loading
          ? const SizedBox(
              width: 20, height: 20,
              child: CircularProgressIndicator(strokeWidth: 2, color: kBg))
          : Text(label,
              style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15, color: kBg)),
    ),
  );
}

class AuthErrorBanner extends StatelessWidget {
  final String message;
  const AuthErrorBanner(this.message, {super.key});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
    decoration: BoxDecoration(
      color: kSell.withOpacity(0.08),
      borderRadius: BorderRadius.circular(8),
      border: Border.all(color: kSell.withOpacity(0.3)),
    ),
    child: Row(children: [
      const Icon(Icons.error_outline, size: 16, color: kSell),
      const SizedBox(width: 8),
      Expanded(child: Text(message, style: const TextStyle(color: kSell, fontSize: 12))),
    ]),
  );
}

class AuthBgPainter extends CustomPainter {
  const AuthBgPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final p1 = Paint()
      ..shader = RadialGradient(
        colors: [kBrand2.withOpacity(0.25), Colors.transparent],
        radius: 0.7,
      ).createShader(Rect.fromLTWH(-size.width * 0.2, -size.height * 0.1,
          size.width * 1.2, size.height * 1.2));
    canvas.drawRect(Offset.zero & size, p1);

    final p2 = Paint()
      ..shader = RadialGradient(
        colors: [kBrand.withOpacity(0.12), Colors.transparent],
        radius: 0.6,
      ).createShader(Rect.fromLTWH(size.width * 0.3, size.height * 0.4,
          size.width * 0.9, size.height * 0.9));
    canvas.drawRect(Offset.zero & size, p2);
  }

  @override
  bool shouldRepaint(_) => false;
}
