import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../api.dart';
import '../balance_state.dart';
import '../theme.dart';

const _methods = [
  _PayMethod('pix',        'PIX',        Icons.qr_code_2,       Color(0xFF32BCAD)),
  _PayMethod('visa',       'Visa',        Icons.credit_card,     Color(0xFF1A1F71)),
  _PayMethod('mastercard', 'Mastercard', Icons.credit_card,      Color(0xFFEB001B)),
  _PayMethod('boleto',     'Boleto',     Icons.receipt_outlined,  kTxtSub),
];

class _PayMethod {
  final String id, label;
  final IconData icon;
  final Color color;
  const _PayMethod(this.id, this.label, this.icon, this.color);
}

class DepositDialog extends StatefulWidget {
  const DepositDialog({super.key});

  @override
  State<DepositDialog> createState() => _DepositDialogState();
}

class _DepositDialogState extends State<DepositDialog> {
  final _amountCtrl = TextEditingController(text: '1000');
  String _selectedMethod = 'pix';
  int _step = 0; // 0=form 1=processing 2=success
  String? _error;

  @override
  void dispose() { _amountCtrl.dispose(); super.dispose(); }

  Future<void> _confirm() async {
    final amount = double.tryParse(_amountCtrl.text.replaceAll(',', '.'));
    if (amount == null || amount <= 0) {
      setState(() => _error = 'Informe um valor válido');
      return;
    }
    setState(() { _step = 1; _error = null; });

    await Future.delayed(const Duration(milliseconds: 1800));

    try {
      await context.read<ApiClient>().deposit(
          amount: _amountCtrl.text.trim().replaceAll(',', '.'),
          method: _selectedMethod);
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
          child: _step == 0
              ? _buildForm()
              : _step == 1
                  ? _buildProcessing()
                  : _buildSuccess(),
        ),
      ),
    );
  }

  Widget _buildForm() => Padding(
    key: const ValueKey('form'),
    padding: const EdgeInsets.all(28),
    child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      // Header
      Row(children: [
        Container(
          width: 42, height: 42,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            gradient: const LinearGradient(colors: [kBrand2, kBrand]),
            boxShadow: [BoxShadow(color: kBrand.withOpacity(0.3), blurRadius: 10)],
          ),
          child: const Icon(Icons.account_balance_wallet_outlined, color: Colors.white, size: 20),
        ),
        const SizedBox(width: 12),
        Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text('Depositar', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: kTxt)),
          const Text('Saldo creditado como USDT-sim', style: TextStyle(fontSize: 11, color: kTxtSub)),
        ]),
        const Spacer(),
        IconButton(
          icon: const Icon(Icons.close, size: 18, color: kTxtSub),
          onPressed: () => Navigator.pop(context),
        ),
      ]),

      const SizedBox(height: 24),

      // Amount
      TextField(
        controller: _amountCtrl,
        style: const TextStyle(color: kTxt, fontSize: 22, fontWeight: FontWeight.w700),
        textAlign: TextAlign.center,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        decoration: InputDecoration(
          prefixText: 'R\$ ',
          prefixStyle: const TextStyle(color: kTxtSub, fontSize: 18),
          hintText: '0,00',
          hintStyle: const TextStyle(color: kTxtMuted),
          filled: true, fillColor: kSurface2,
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(color: kBorder)),
          enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(color: kBorder)),
          focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(12),
              borderSide: const BorderSide(color: kBrand, width: 1.5)),
          contentPadding: const EdgeInsets.symmetric(vertical: 18, horizontal: 16),
        ),
      ),

      const SizedBox(height: 8),

      // Quick amounts
      Row(children: [
        for (final v in ['100', '500', '1000', '5000'])
          Expanded(child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 3),
            child: GestureDetector(
              onTap: () => setState(() => _amountCtrl.text = v),
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: 6),
                decoration: BoxDecoration(
                  color: kBorder.withOpacity(0.3),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text('R\$ $v', textAlign: TextAlign.center,
                    style: const TextStyle(fontSize: 11, color: kTxtSub)),
              ),
            ),
          )),
      ]),

      const SizedBox(height: 20),

      // Payment method
      const Text('FORMA DE PAGAMENTO',
          style: TextStyle(fontSize: 10, color: kTxtMuted, letterSpacing: 2, fontWeight: FontWeight.w700)),
      const SizedBox(height: 10),
      Row(children: _methods.map((m) {
        final sel = m.id == _selectedMethod;
        return Expanded(child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 4),
          child: GestureDetector(
            onTap: () => setState(() => _selectedMethod = m.id),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              padding: const EdgeInsets.symmetric(vertical: 10),
              decoration: BoxDecoration(
                color: sel ? m.color.withOpacity(0.12) : kSurface2,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: sel ? m.color.withOpacity(0.6) : kBorder),
              ),
              child: Column(mainAxisSize: MainAxisSize.min, children: [
                Icon(m.icon, size: 20, color: sel ? m.color : kTxtSub),
                const SizedBox(height: 4),
                Text(m.label, style: TextStyle(fontSize: 10, color: sel ? m.color : kTxtSub,
                    fontWeight: sel ? FontWeight.w700 : FontWeight.normal)),
              ]),
            ),
          ),
        ));
      }).toList()),

      if (_error != null) ...[
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(color: kSell.withOpacity(0.1), borderRadius: BorderRadius.circular(8),
              border: Border.all(color: kSell.withOpacity(0.3))),
          child: Text(_error!, style: const TextStyle(color: kSell, fontSize: 12)),
        ),
      ],

      const SizedBox(height: 20),

      // Disclaimer
      Container(
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(color: kBrand.withOpacity(0.05),
            borderRadius: BorderRadius.circular(8), border: Border.all(color: kBrand.withOpacity(0.15))),
        child: const Row(children: [
          Icon(Icons.info_outline, size: 14, color: kBrand),
          SizedBox(width: 8),
          Expanded(child: Text('Simulação — nenhum valor real é cobrado. Saldo creditado como USDT-sim.',
              style: TextStyle(fontSize: 11, color: kTxtSub))),
        ]),
      ),
      const SizedBox(height: 20),

      Container(
        decoration: BoxDecoration(borderRadius: BorderRadius.circular(10),
            boxShadow: [BoxShadow(color: kBrand.withOpacity(0.35), blurRadius: 12)]),
        child: FilledButton(
          onPressed: _confirm,
          style: FilledButton.styleFrom(
            backgroundColor: kBrand, foregroundColor: kBg,
            minimumSize: const Size(double.infinity, 50),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          child: const Text('Confirmar depósito', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 15, color: kBg)),
        ),
      ),
    ]),
  );

  Widget _buildProcessing() => Padding(
    key: const ValueKey('processing'),
    padding: const EdgeInsets.symmetric(vertical: 60, horizontal: 28),
    child: Column(mainAxisSize: MainAxisSize.min, children: [
      const SizedBox(
        width: 56, height: 56,
        child: CircularProgressIndicator(color: kBrand, strokeWidth: 3),
      ),
      const SizedBox(height: 24),
      const Text('Processando pagamento…',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: kTxt)),
      const SizedBox(height: 8),
      Text('Aguarde enquanto confirmamos o ${_methods.firstWhere((m) => m.id == _selectedMethod, orElse: () => _methods.first).label}…',
          textAlign: TextAlign.center,
          style: const TextStyle(fontSize: 13, color: kTxtSub)),
    ]),
  );

  Widget _buildSuccess() => Padding(
    key: const ValueKey('success'),
    padding: const EdgeInsets.all(28),
    child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Center(child: Container(
        width: 72, height: 72,
        decoration: BoxDecoration(shape: BoxShape.circle,
            color: kBuy.withOpacity(0.15),
            border: Border.all(color: kBuy.withOpacity(0.4))),
        child: const Icon(Icons.check_rounded, color: kBuy, size: 40),
      )),
      const SizedBox(height: 20),
      const Text('Depósito confirmado!', textAlign: TextAlign.center,
          style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: kTxt)),
      const SizedBox(height: 8),
      Text('R\$ ${_amountCtrl.text} creditados como USDT-sim.',
          textAlign: TextAlign.center,
          style: const TextStyle(fontSize: 14, color: kTxtSub)),
      const SizedBox(height: 8),
      const Text('Seu saldo foi atualizado.', textAlign: TextAlign.center,
          style: TextStyle(fontSize: 12, color: kTxtMuted)),
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
