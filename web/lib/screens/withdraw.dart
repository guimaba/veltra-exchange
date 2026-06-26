import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api.dart';
import '../balance_state.dart';
import '../fmt.dart';
import '../theme.dart';

/// Saque simulado. Só moedas FIAT que o usuário possui podem ser sacadas (1:1,
/// na própria moeda). Cripto/USDT não se saca — converte na aba de trading.
class WithdrawDialog extends StatefulWidget {
  const WithdrawDialog({super.key});

  @override
  State<WithdrawDialog> createState() => _WithdrawDialogState();
}

class _WithdrawDialogState extends State<WithdrawDialog> {
  final _amountCtrl = TextEditingController();
  String? _asset; // moeda fiat selecionada (ex.: BRL)
  int _step = 0; // 0=form 1=processing 2=success
  String? _error;
  double _lastAmount = 0;

  static bool _isFiat(String a) => kFiatCurrencies.any((c) => c.code == a);
  FiatCurrency get _fiat => fiatByCode(_asset ?? 'BRL');

  @override
  void dispose() { _amountCtrl.dispose(); super.dispose(); }

  Future<void> _confirm() async {
    final amount = double.tryParse(_amountCtrl.text.replaceAll(',', '.'));
    if (_asset == null) { setState(() => _error = 'Escolha uma moeda'); return; }
    if (amount == null || amount <= 0) { setState(() => _error = 'Informe um valor válido'); return; }
    _lastAmount = amount;
    setState(() { _step = 1; _error = null; });
    await Future.delayed(const Duration(milliseconds: 1600));
    try {
      // Saque 1:1: o ativo é a própria moeda.
      await context.read<ApiClient>().withdraw(
          asset: _asset!,
          amount: _amountCtrl.text.trim().replaceAll(',', '.'),
          currency: _asset!);
      context.read<BalanceState>().refresh();
      if (mounted) setState(() => _step = 2);
    } on ApiException catch (e) {
      if (mounted) setState(() { _step = 0; _error = e.message; });
    } catch (e) {
      if (mounted) setState(() { _step = 0; _error = e.toString(); });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: kSurface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
        side: BorderSide(color: kBrand.withOpacity(0.3)),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: AnimatedSwitcher(
          duration: const Duration(milliseconds: 300),
          child: _step == 0 ? _buildForm() : _step == 1 ? _buildProcessing() : _buildSuccess(),
        ),
      ),
    );
  }

  Widget _buildForm() {
    final bal = context.watch<BalanceState>();
    // Só moedas fiat que o usuário possui.
    final fiatHeld = bal.nonZero.where((b) => _isFiat(b.asset)).toList();
    if (_asset == null && fiatHeld.isNotEmpty) _asset = fiatHeld.first.asset;
    final available = _asset == null ? 0.0 : bal.balanceOf(_asset!);

    return Padding(
      key: const ValueKey('form'),
      padding: const EdgeInsets.all(28),
      child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [
          Container(
            width: 42, height: 42,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              gradient: const LinearGradient(colors: [kSell, Color(0xFFFF8A65)]),
              boxShadow: [BoxShadow(color: kSell.withOpacity(0.3), blurRadius: 10)],
            ),
            child: const Icon(Icons.arrow_upward_rounded, color: Colors.white, size: 20),
          ),
          const SizedBox(width: 12),
          const Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text('Sacar', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: kTxt)),
            Text('Saque em moeda fiat que você possui', style: TextStyle(fontSize: 11, color: kTxtSub)),
          ]),
          const Spacer(),
          IconButton(icon: const Icon(Icons.close, size: 18, color: kTxtSub), onPressed: () => Navigator.pop(context)),
        ]),

        const SizedBox(height: 20),

        if (fiatHeld.isEmpty)
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(color: kSurface2, borderRadius: BorderRadius.circular(10)),
            child: const Text(
              'Você não tem moeda fiat para sacar.\nConverta cripto/USDT em uma moeda (R\$, US\$, €, £) na aba de Trading primeiro.',
              textAlign: TextAlign.center, style: TextStyle(color: kTxtSub, fontSize: 12, height: 1.4)),
          )
        else ...[
          const Text('MOEDA', style: TextStyle(fontSize: 10, color: kTxtMuted, letterSpacing: 2, fontWeight: FontWeight.w700)),
          const SizedBox(height: 8),
          Row(children: [
            for (final b in fiatHeld)
              Expanded(child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 3),
                child: GestureDetector(
                  onTap: () => setState(() => _asset = b.asset),
                  child: AnimatedContainer(
                    duration: const Duration(milliseconds: 150),
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    decoration: BoxDecoration(
                      color: _asset == b.asset ? kBuy.withOpacity(0.15) : kSurface2,
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: _asset == b.asset ? kBuy : kBorder, width: _asset == b.asset ? 1.5 : 1),
                    ),
                    child: Column(mainAxisSize: MainAxisSize.min, children: [
                      Text(fiatByCode(b.asset).symbol, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w800, color: _asset == b.asset ? kBuy : kTxt)),
                      Text(b.asset, style: TextStyle(fontSize: 9, color: _asset == b.asset ? kBuy : kTxtSub)),
                    ]),
                  ),
                ),
              )),
          ]),
          const SizedBox(height: 4),
          Align(alignment: Alignment.centerRight, child: GestureDetector(
            onTap: () => setState(() => _amountCtrl.text = available.toString()),
            child: Text('Disponível: ${_fiat.symbol} ${Fmt.money(available)}  (máx)',
                style: const TextStyle(fontSize: 10, color: kBrand)),
          )),
          const SizedBox(height: 12),

          TextField(
            controller: _amountCtrl,
            onChanged: (_) => setState(() {}),
            style: const TextStyle(color: kTxt, fontSize: 20, fontWeight: FontWeight.w700),
            textAlign: TextAlign.center,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: InputDecoration(
              prefixText: '${_fiat.symbol} ',
              prefixStyle: const TextStyle(color: kTxtSub, fontSize: 16),
              hintText: '0,00', hintStyle: const TextStyle(color: kTxtMuted),
              filled: true, fillColor: kSurface2,
              border: OutlineInputBorder(borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: kBorder)),
              enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: kBorder)),
              focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(12), borderSide: const BorderSide(color: kSell, width: 1.5)),
              contentPadding: const EdgeInsets.symmetric(vertical: 16, horizontal: 16),
            ),
          ),
        ],

        if (_error != null) ...[
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(color: kSell.withOpacity(0.1), borderRadius: BorderRadius.circular(8), border: Border.all(color: kSell.withOpacity(0.3))),
            child: Text(_error!, style: const TextStyle(color: kSell, fontSize: 12)),
          ),
        ],

        const SizedBox(height: 18),

        if (fiatHeld.isNotEmpty)
          FilledButton(
            onPressed: _confirm,
            style: FilledButton.styleFrom(
              backgroundColor: kSell, foregroundColor: Colors.white,
              minimumSize: const Size(double.infinity, 50),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
            ),
            child: const Text('Confirmar saque', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15)),
          ),
      ]),
    );
  }

  Widget _buildProcessing() => const Padding(
    key: ValueKey('processing'),
    padding: EdgeInsets.symmetric(vertical: 60, horizontal: 28),
    child: Column(mainAxisSize: MainAxisSize.min, children: [
      SizedBox(width: 56, height: 56, child: CircularProgressIndicator(color: kSell, strokeWidth: 3)),
      SizedBox(height: 24),
      Text('Processando saque…', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: kTxt)),
    ]),
  );

  Widget _buildSuccess() => Padding(
    key: const ValueKey('success'),
    padding: const EdgeInsets.all(28),
    child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Center(child: Container(
        width: 72, height: 72,
        decoration: BoxDecoration(shape: BoxShape.circle, color: kBuy.withOpacity(0.15), border: Border.all(color: kBuy.withOpacity(0.4))),
        child: const Icon(Icons.check_rounded, color: kBuy, size: 40),
      )),
      const SizedBox(height: 20),
      const Text('Saque confirmado!', textAlign: TextAlign.center, style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: kTxt)),
      const SizedBox(height: 8),
      Text('${_fiat.symbol} ${Fmt.money(_lastAmount)} enviados (simulação).',
          textAlign: TextAlign.center, style: const TextStyle(fontSize: 14, color: kTxtSub)),
      const SizedBox(height: 28),
      FilledButton(
        onPressed: () => Navigator.pop(context),
        style: FilledButton.styleFrom(
          backgroundColor: kBuy, foregroundColor: Colors.black,
          minimumSize: const Size(double.infinity, 46),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        ),
        child: const Text('Fechar', style: TextStyle(fontWeight: FontWeight.w800)),
      ),
    ]),
  );
}
