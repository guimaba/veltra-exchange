import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../auth_state.dart';
import '../market_state.dart';
import '../theme.dart';
import '../widgets/auth_widgets.dart';

class LoginScreen extends StatefulWidget {
  final VoidCallback onGoRegister;
  const LoginScreen({super.key, required this.onGoRegister});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _userCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  bool _obscure = true;

  @override
  void dispose() {
    _userCtrl.dispose();
    _passCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    await context.read<AuthState>().login(_userCtrl.text.trim(), _passCtrl.text);
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthState>();
    final wide = MediaQuery.of(context).size.width >= 900;

    return Scaffold(
      backgroundColor: kBg,
      body: Stack(children: [
        // Electric background
        Positioned.fill(child: CustomPaint(painter: const AuthBgPainter())),
        // Subtle grid overlay
        Positioned.fill(child: CustomPaint(painter: const _GridPainter())),

        if (wide)
          Row(children: [
            Expanded(flex: 6, child: _HeroPanel()),
            Container(width: 1, color: kBorder),
            SizedBox(width: 520, child: _FormPanel(
              formKey: _formKey, userCtrl: _userCtrl, passCtrl: _passCtrl,
              obscure: _obscure, auth: auth,
              onToggleObscure: () => setState(() => _obscure = !_obscure),
              onSubmit: _submit,
              onGoRegister: widget.onGoRegister,
            )),
          ])
        else
          _FormPanel(
            formKey: _formKey, userCtrl: _userCtrl, passCtrl: _passCtrl,
            obscure: _obscure, auth: auth,
            onToggleObscure: () => setState(() => _obscure = !_obscure),
            onSubmit: _submit,
            onGoRegister: widget.onGoRegister,
            scrollable: true,
          ),
      ]),
    );
  }
}

// ─── Hero panel (left side) ───────────────────────────────────────────────────

class _HeroPanel extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final market = context.watch<MarketState>();
    final coins  = market.coins;
    final btc    = coins.where((c) => c.symbol == 'BTC').firstOrNull;
    final eth    = coins.where((c) => c.symbol == 'ETH').firstOrNull;
    final vlt    = coins.where((c) => c.symbol == 'VLT').firstOrNull;
    final top5   = coins.take(5).toList();

    return Column(children: [
      // ── Top nav bar ──
      Padding(
        padding: const EdgeInsets.fromLTRB(40, 30, 40, 0),
        child: Row(children: [
          ShaderMask(
            shaderCallback: (b) => const LinearGradient(
                colors: [kBrand2, kBrand]).createShader(b),
            child: const Row(mainAxisSize: MainAxisSize.min, children: [
              Icon(Icons.hub_outlined, color: Colors.white, size: 22),
              SizedBox(width: 8),
              Text('VELTRA',
                  style: TextStyle(color: Colors.white, fontSize: 18,
                      fontWeight: FontWeight.w900, letterSpacing: 4)),
            ]),
          ),
          const Spacer(),
          // Live badge
          Row(mainAxisSize: MainAxisSize.min, children: [
            Container(width: 7, height: 7, decoration: BoxDecoration(
              color: kBrand, shape: BoxShape.circle,
              boxShadow: [BoxShadow(color: kBrand.withOpacity(0.9), blurRadius: 8)],
            )),
            const SizedBox(width: 7),
            const Text('AO VIVO', style: TextStyle(fontSize: 10, color: kBrand,
                fontWeight: FontWeight.w800, letterSpacing: 2)),
          ]),
        ]),
      ),

      // ── Central hero ──
      Expanded(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 40),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [

              // Big logo symbol with electric glow
              Container(
                width: 120, height: 120,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(32),
                  gradient: const LinearGradient(
                    begin: Alignment.topLeft, end: Alignment.bottomRight,
                    colors: [kBrand2, kBrand],
                  ),
                  boxShadow: [
                    BoxShadow(color: kBrand.withOpacity(0.5), blurRadius: 40, spreadRadius: 4),
                    BoxShadow(color: kBrand2.withOpacity(0.3), blurRadius: 60, spreadRadius: 0),
                  ],
                ),
                child: const Icon(Icons.hub_outlined, color: Colors.white, size: 62),
              ),
              const SizedBox(height: 28),

              // Big wordmark
              ShaderMask(
                shaderCallback: (b) => const LinearGradient(
                    begin: Alignment.topLeft, end: Alignment.bottomRight,
                    colors: [Colors.white, kBrand]).createShader(b),
                child: const Text('VELTRA EXCHANGE',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 50,
                        fontWeight: FontWeight.w900,
                        letterSpacing: 7,
                        height: 1.0)),
              ),
              const SizedBox(height: 12),

              // Tagline
              const Text('Negocie o futuro das criptomoedas.',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 20, color: kTxtSub,
                      fontWeight: FontWeight.w400, letterSpacing: 0.5)),
              const SizedBox(height: 36),

              // Live price cards — BTC, ETH, VLT
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                _LiveCard(symbol: 'BTC', coin: btc),
                const SizedBox(width: 12),
                _LiveCard(symbol: 'ETH', coin: eth),
                const SizedBox(width: 12),
                _LiveCard(symbol: 'VLT', coin: vlt, isVlt: true),
              ]),
              const SizedBox(height: 32),

              // Feature chips
              Wrap(alignment: WrapAlignment.center, spacing: 8, runSpacing: 8, children: [
                _FeatureChip(Icons.bolt, '33 Moedas'),
                _FeatureChip(Icons.security_outlined, 'JWT + bcrypt'),
                _FeatureChip(Icons.candlestick_chart_outlined, 'Order Book L2'),
                _FeatureChip(Icons.account_balance_outlined, 'Ledger Dupla Entrada'),
                _FeatureChip(Icons.cloud_sync_outlined, 'Preços Reais'),
              ]),
            ],
          ),
        ),
      ),

      // ── Bottom ticker strip ──
      Container(
        height: 1, color: kBorder.withOpacity(0.5),
      ),
      Container(
        color: kSurface.withOpacity(0.6),
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 14),
        child: top5.isEmpty
            ? const SizedBox.shrink()
            : Row(children: [
                const Text('MERCADO',
                    style: TextStyle(fontSize: 10, color: kTxtMuted,
                        letterSpacing: 2, fontWeight: FontWeight.w700)),
                const SizedBox(width: 16),
                Expanded(
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: top5.map((c) => _TickerPill(coin: c)).toList(),
                  ),
                ),
              ]),
      ),
    ]);
  }
}

class _LiveCard extends StatelessWidget {
  final String symbol;
  final MarketCoin? coin;
  final bool isVlt;
  const _LiveCard({required this.symbol, required this.coin, this.isVlt = false});

  @override
  Widget build(BuildContext context) {
    if (coin == null) {
      return Container(
        width: 160, height: 80,
        decoration: BoxDecoration(color: kSurface2, borderRadius: BorderRadius.circular(12),
            border: Border.all(color: kBorder)),
        child: Center(child: Text(symbol, style: const TextStyle(color: kTxtMuted, fontSize: 14, fontWeight: FontWeight.w700))),
      );
    }
    final up = coin!.change24h >= 0;
    final c  = up ? kBuy : kSell;
    return Container(
      width: 180,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kSurface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: isVlt ? kBrand.withOpacity(0.5) : kBorder),
        boxShadow: isVlt ? [BoxShadow(color: kBrand.withOpacity(0.15), blurRadius: 16)] : null,
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Text(symbol,
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800,
                  color: isVlt ? kBrand : kTxt)),
          if (isVlt) ...[
            const SizedBox(width: 6),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
              decoration: BoxDecoration(color: kBrand.withOpacity(0.15), borderRadius: BorderRadius.circular(4)),
              child: const Text('Veltra', style: TextStyle(fontSize: 8, color: kBrand, fontWeight: FontWeight.w800)),
            ),
          ],
          const Spacer(),
          Icon(up ? Icons.arrow_upward : Icons.arrow_downward, size: 12, color: c),
        ]),
        const SizedBox(height: 6),
        Text(_fmtBRL(coin!.priceBRL),
            style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w900, color: kTxt,
                fontFeatures: [FontFeature.tabularFigures()])),
        const SizedBox(height: 2),
        Text('${up ? '+' : ''}${coin!.change24h.toStringAsFixed(2)}%',
            style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w600)),
      ]),
    );
  }

  String _fmtBRL(double v) {
    if (v >= 100000) return 'R\$${(v/1000).toStringAsFixed(0)}K';
    if (v >= 1000) return 'R\$${v.toStringAsFixed(0)}';
    if (v >= 1) return 'R\$${v.toStringAsFixed(2)}';
    return 'R\$${v.toStringAsFixed(4)}';
  }
}

class _TickerPill extends StatelessWidget {
  final MarketCoin coin;
  const _TickerPill({required this.coin});

  @override
  Widget build(BuildContext context) {
    final up = coin.change24h >= 0;
    final c  = up ? kBuy : kSell;
    return Row(mainAxisSize: MainAxisSize.min, children: [
      Text(coin.symbol, style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt)),
      const SizedBox(width: 7),
      Text('R\$${_fmt(coin.priceBRL)}', style: const TextStyle(fontSize: 13, color: kTxtSub,
          fontFeatures: [FontFeature.tabularFigures()])),
      const SizedBox(width: 4),
      Text('${up ? '+' : ''}${coin.change24h.toStringAsFixed(2)}%',
          style: TextStyle(fontSize: 12, color: c, fontWeight: FontWeight.w600)),
    ]);
  }

  String _fmt(double v) {
    if (v >= 100000) return '${(v/1000).toStringAsFixed(0)}K';
    if (v >= 1) return v.toStringAsFixed(2);
    return v.toStringAsFixed(4);
  }
}

class _FeatureChip extends StatelessWidget {
  final IconData icon;
  final String label;
  const _FeatureChip(this.icon, this.label);

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: kSurface2,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: kBorder),
        ),
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          Icon(icon, size: 14, color: kBrand),
          const SizedBox(width: 6),
          Text(label, style: const TextStyle(fontSize: 12, color: kTxtSub)),
        ]),
      );
}

class _TickerRow extends StatelessWidget {
  final MarketCoin coin;
  const _TickerRow({required this.coin});

  @override
  Widget build(BuildContext context) {
    final up = coin.change24h >= 0;
    final c = up ? kBuy : kSell;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(children: [
        // Avatar
        Container(
          width: 28, height: 28,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(8),
            color: c.withOpacity(0.1),
          ),
          child: Center(
            child: Text(
              coin.symbol.isNotEmpty ? coin.symbol[0] : '?',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w800, color: c),
            ),
          ),
        ),
        const SizedBox(width: 10),
        SizedBox(
          width: 80,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(coin.symbol,
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt)),
            Text(coin.name,
                style: const TextStyle(fontSize: 10, color: kTxtMuted),
                overflow: TextOverflow.ellipsis),
          ]),
        ),
        const Spacer(),
        Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
          Text(
            'R\$ ${_fmtPrice(coin.priceBRL)}',
            style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: kTxt,
                fontFeatures: [FontFeature.tabularFigures()]),
          ),
          Text(
            '${up ? '+' : ''}${coin.change24h.toStringAsFixed(2)}%',
            style: TextStyle(fontSize: 11, color: c, fontWeight: FontWeight.w600),
          ),
        ]),
        const SizedBox(width: 8),
        // Mini trend indicator
        Container(
          width: 40, height: 20,
          decoration: BoxDecoration(
            color: c.withOpacity(0.08),
            borderRadius: BorderRadius.circular(4),
          ),
          child: Center(
            child: Icon(
              up ? Icons.trending_up : Icons.trending_down,
              size: 14, color: c,
            ),
          ),
        ),
      ]),
    );
  }

  String _fmtPrice(double p) {
    if (p >= 10000) return p.toStringAsFixed(0).replaceAllMapped(
      RegExp(r'\B(?=(\d{3})+(?!\d))'), (m) => '.');
    if (p >= 1) return p.toStringAsFixed(2);
    if (p >= 0.01) return p.toStringAsFixed(4);
    return p.toStringAsExponential(2);
  }
}

class _TickerSkeleton extends StatelessWidget {
  @override
  Widget build(BuildContext context) => Column(
        children: List.generate(
          3,
          (_) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 5),
            child: Container(
              height: 30,
              decoration: BoxDecoration(
                color: kSurface2,
                borderRadius: BorderRadius.circular(8),
              ),
            ),
          ),
        ),
      );
}

// ─── Form panel (right side) ──────────────────────────────────────────────────

class _FormPanel extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final TextEditingController userCtrl, passCtrl;
  final bool obscure;
  final AuthState auth;
  final VoidCallback onToggleObscure, onSubmit, onGoRegister;
  final bool scrollable;

  const _FormPanel({
    required this.formKey,
    required this.userCtrl,
    required this.passCtrl,
    required this.obscure,
    required this.auth,
    required this.onToggleObscure,
    required this.onSubmit,
    required this.onGoRegister,
    this.scrollable = false,
  });

  @override
  Widget build(BuildContext context) {
    Widget content = Column(
      mainAxisAlignment:
          scrollable ? MainAxisAlignment.start : MainAxisAlignment.center,
      children: [
        if (scrollable) ...[const SizedBox(height: 48), const VeltraLogo(), const SizedBox(height: 36)],
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 52),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            if (!scrollable)
              Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                ShaderMask(
                  shaderCallback: (b) => const LinearGradient(
                      colors: [kBrand2, kBrand]).createShader(b),
                  child: const Text('VELTRA',
                      style: TextStyle(
                          color: Colors.white,
                          fontSize: 13,
                          fontWeight: FontWeight.w900,
                          letterSpacing: 4)),
                ),
                const SizedBox(height: 6),
                const Text('Bem-vindo de volta',
                    style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: kTxt)),
                const SizedBox(height: 4),
                const Text('Entre na sua conta',
                    style: TextStyle(fontSize: 13, color: kTxtSub)),
                const SizedBox(height: 32),
              ]),

            Form(
              key: formKey,
              child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                TextFormField(
                  controller: userCtrl,
                  style: const TextStyle(color: kTxt, fontSize: 14),
                  decoration: const InputDecoration(
                    labelText: 'Usuário',
                    prefixIcon: Icon(Icons.person_outline, size: 18, color: kTxtSub),
                  ),
                  autofillHints: const [AutofillHints.username],
                  textInputAction: TextInputAction.next,
                  validator: (v) => (v == null || v.trim().isEmpty) ? 'Obrigatório' : null,
                ),
                const SizedBox(height: 14),
                TextFormField(
                  controller: passCtrl,
                  obscureText: obscure,
                  style: const TextStyle(color: kTxt, fontSize: 14),
                  decoration: InputDecoration(
                    labelText: 'Senha',
                    prefixIcon: const Icon(Icons.lock_outline, size: 18, color: kTxtSub),
                    suffixIcon: IconButton(
                      icon: Icon(
                          obscure ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                          size: 18, color: kTxtSub),
                      onPressed: onToggleObscure,
                    ),
                  ),
                  autofillHints: const [AutofillHints.password],
                  textInputAction: TextInputAction.done,
                  onFieldSubmitted: (_) => onSubmit(),
                  validator: (v) => (v == null || v.isEmpty) ? 'Obrigatório' : null,
                ),
                if (auth.error != null) ...[
                  const SizedBox(height: 12),
                  AuthErrorBanner(auth.error!),
                ],
                const SizedBox(height: 20),
                GlowButton(
                    onPressed: auth.loading ? null : onSubmit,
                    loading: auth.loading,
                    label: 'Entrar'),
              ]),
            ),

            const SizedBox(height: 24),
            Row(mainAxisAlignment: MainAxisAlignment.center, children: [
              const Text('Não tem conta? ', style: TextStyle(color: kTxtSub, fontSize: 13)),
              GestureDetector(
                onTap: onGoRegister,
                child: const Text('Criar conta',
                    style: TextStyle(color: kBrand, fontSize: 13, fontWeight: FontWeight.w700)),
              ),
            ]),

            if (!scrollable) ...[
              const SizedBox(height: 40),
              // Stats row
              Row(children: [
                _AuthStat('33', 'Moedas'),
                _Divider(),
                _AuthStat('L2', 'Order Book'),
                _Divider(),
                _AuthStat('JWT', 'Seguro'),
              ]),
            ],
            if (scrollable) const SizedBox(height: 48),
          ]),
        ),
      ],
    );

    if (scrollable) return SingleChildScrollView(child: content);
    return content;
  }
}

class _AuthStat extends StatelessWidget {
  final String value, label;
  const _AuthStat(this.value, this.label);

  @override
  Widget build(BuildContext context) => Expanded(
        child: Column(children: [
          ShaderMask(
            shaderCallback: (b) =>
                const LinearGradient(colors: [kBrand2, kBrand]).createShader(b),
            child: Text(value,
                style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w900,
                    color: Colors.white)),
          ),
          const SizedBox(height: 2),
          Text(label, style: const TextStyle(fontSize: 10, color: kTxtMuted)),
        ]),
      );
}

class _Divider extends StatelessWidget {
  @override
  Widget build(BuildContext context) =>
      Container(width: 1, height: 28, color: kBorder);
}

// ─── Grid painter (background decoration) ────────────────────────────────────

class _GridPainter extends CustomPainter {
  const _GridPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = kBrand.withOpacity(0.025)
      ..strokeWidth = 0.5;
    const step = 60.0;
    for (double x = 0; x < size.width; x += step) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), paint);
    }
    for (double y = 0; y < size.height; y += step) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), paint);
    }
  }

  @override
  bool shouldRepaint(_) => false;
}
