import 'package:flutter/material.dart';

// ─── Veltra Design Tokens ───────────────────────────────────────────────────
const kBg       = Color(0xFF050E1A); // deep-space background
const kSurface  = Color(0xFF091726); // cards / panels
const kSurface2 = Color(0xFF0D2138); // elevated surfaces / modals
const kBorder   = Color(0xFF162D4A); // default border
const kBrand    = Color(0xFF00D4FF); // electric cyan — Veltra blue
const kBrand2   = Color(0xFF7B2FBE); // electric violet
const kBuy      = Color(0xFF02C076); // buy / up
const kSell     = Color(0xFFF6465D); // sell / down
const kTxt      = Color(0xFFE2EFFF); // primary text
const kTxtSub   = Color(0xFF6B8EAE); // secondary text
const kTxtMuted = Color(0xFF304D66); // muted / placeholder

ThemeData appTheme() {
  final scheme = ColorScheme(
    brightness: Brightness.dark,
    primary: kBrand,
    onPrimary: kBg,
    secondary: kBrand2,
    onSecondary: kTxt,
    error: kSell,
    onError: kTxt,
    surface: kSurface,
    onSurface: kTxt,
    surfaceContainerHighest: kSurface2,
    outline: kBorder,
  );

  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: scheme,
    scaffoldBackgroundColor: kBg,

    appBarTheme: const AppBarTheme(
      backgroundColor: kBg,
      surfaceTintColor: Colors.transparent,
      shadowColor: Colors.transparent,
      elevation: 0,
      centerTitle: false,
      titleTextStyle: TextStyle(color: kTxt, fontSize: 16, fontWeight: FontWeight.w700),
      iconTheme: IconThemeData(color: kTxtSub),
    ),

    cardTheme: const CardTheme(
      elevation: 0,
      color: kSurface,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(12)),
        side: BorderSide(color: kBorder),
      ),
    ),

    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: kSurface,
      surfaceTintColor: Colors.transparent,
      indicatorColor: kBrand.withOpacity(0.18),
      iconTheme: WidgetStatePropertyAll(IconThemeData(color: kTxtSub)),
      labelTextStyle: const WidgetStatePropertyAll(
          TextStyle(fontSize: 11, color: kTxtSub)),
    ),

    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: kBorder.withOpacity(0.35),
      isDense: true,
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: kBorder)),
      enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: kBorder)),
      focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: kBrand, width: 1.5)),
      labelStyle: const TextStyle(color: kTxtSub, fontSize: 13),
      hintStyle: const TextStyle(color: kTxtMuted, fontSize: 13),
    ),

    dividerTheme: const DividerThemeData(color: kBorder, thickness: 1),

    tabBarTheme: TabBarTheme(
      dividerColor: Colors.transparent,
      labelColor: kBrand,
      unselectedLabelColor: kTxtSub,
      labelStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
      unselectedLabelStyle: const TextStyle(fontSize: 13),
      indicator: UnderlineTabIndicator(
        borderSide: const BorderSide(color: kBrand, width: 2),
        borderRadius: BorderRadius.circular(2),
      ),
    ),

    snackBarTheme: SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: kSurface2,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(10),
        side: const BorderSide(color: kBorder),
      ),
      contentTextStyle: const TextStyle(color: kTxt),
    ),

    filledButtonTheme: FilledButtonThemeData(
      style: ButtonStyle(
        backgroundColor: const WidgetStatePropertyAll(kBrand),
        foregroundColor: const WidgetStatePropertyAll(kBg),
        textStyle: const WidgetStatePropertyAll(
            TextStyle(fontWeight: FontWeight.w700, fontSize: 14)),
        shape: WidgetStatePropertyAll(
            RoundedRectangleBorder(borderRadius: BorderRadius.circular(8))),
        padding: const WidgetStatePropertyAll(
            EdgeInsets.symmetric(vertical: 14, horizontal: 20)),
      ),
    ),

    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        foregroundColor: const WidgetStatePropertyAll(kTxtSub),
        side: const WidgetStatePropertyAll(BorderSide(color: kBorder)),
        shape: WidgetStatePropertyAll(
            RoundedRectangleBorder(borderRadius: BorderRadius.circular(8))),
      ),
    ),

    popupMenuTheme: const PopupMenuThemeData(
      color: kSurface2,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(10)),
        side: BorderSide(color: kBorder),
      ),
    ),

    textTheme: const TextTheme(
      headlineLarge: TextStyle(color: kTxt, fontWeight: FontWeight.w800),
      headlineMedium: TextStyle(color: kTxt, fontWeight: FontWeight.w700),
      headlineSmall: TextStyle(color: kTxt, fontWeight: FontWeight.w600),
      titleLarge: TextStyle(color: kTxt, fontWeight: FontWeight.w600),
      titleMedium: TextStyle(color: kTxt, fontWeight: FontWeight.w600),
      titleSmall: TextStyle(color: kTxt, fontWeight: FontWeight.w500),
      bodyLarge: TextStyle(color: kTxt),
      bodyMedium: TextStyle(color: kTxt),
      bodySmall: TextStyle(color: kTxtSub, fontSize: 12),
      labelLarge: TextStyle(color: kTxt, fontWeight: FontWeight.w500),
      labelSmall: TextStyle(color: kTxtMuted, fontSize: 11),
    ),
  );
}

// ─── Shared UI helpers ────────────────────────────────────────────────────────

/// Container com borda Veltra (opcional glow elétrico).
class VBox extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final double radius;
  final bool glow;
  final Color? glowColor;
  const VBox({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.radius = 12,
    this.glow = false,
    this.glowColor,
  });

  @override
  Widget build(BuildContext context) {
    final c = glowColor ?? kBrand;
    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(radius),
        border: Border.all(color: glow ? c.withOpacity(0.55) : kBorder),
        boxShadow: glow
            ? [BoxShadow(color: c.withOpacity(0.15), blurRadius: 20, spreadRadius: 0)]
            : null,
      ),
      child: child,
    );
  }
}

/// Badge de variação percentual.
class ChangeBadge extends StatelessWidget {
  final double pct;
  const ChangeBadge(this.pct, {super.key});

  @override
  Widget build(BuildContext context) {
    final up = pct >= 0;
    final c = up ? kBuy : kSell;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: c.withOpacity(0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(up ? Icons.arrow_upward : Icons.arrow_downward, size: 11, color: c),
        const SizedBox(width: 3),
        Text('${pct.abs().toStringAsFixed(2)}%',
            style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w700)),
      ]),
    );
  }
}
