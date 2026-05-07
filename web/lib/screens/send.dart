import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';

import '../api.dart';
import '../state.dart';

class SendScreen extends StatefulWidget {
  const SendScreen();

  @override
  State<SendScreen> createState() => _SendScreenState();
}

class _SendScreenState extends State<SendScreen> {
  final _formKey = GlobalKey<FormState>();
  final _senderCtrl = TextEditingController();
  final _receiverCtrl = TextEditingController();
  final _amountCtrl = TextEditingController();
  bool _busy = false;
  String? _lastResult;

  @override
  void dispose() {
    _senderCtrl.dispose();
    _receiverCtrl.dispose();
    _amountCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _busy = true;
      _lastResult = null;
    });
    final state = context.read<AppState>();
    try {
      final txId = await state.api.postTransaction(
        _senderCtrl.text.trim(),
        _receiverCtrl.text.trim(),
        double.parse(_amountCtrl.text.replaceAll(',', '.')),
      );
      setState(() {
        _lastResult = 'Transacao enfileirada. tx_id=$txId';
      });
    } on ApiException catch (e) {
      setState(() {
        _lastResult = 'Erro ${e.status}: ${e.message}';
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final fmt = NumberFormat.simpleCurrency(locale: 'pt_BR');
    final balance = state.balances[_senderCtrl.text.trim()];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  'Enviar transacao',
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                const SizedBox(height: 8),
                Text(
                  'O lider valida o saldo do remetente antes de aceitar a transferencia. Se nao houver saldo suficiente, sera publicado um evento transaction.rejected.',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 16),
                TextFormField(
                  controller: _senderCtrl,
                  decoration: InputDecoration(
                    labelText: 'De',
                    hintText: 'ex: alice',
                    helperText: balance == null
                        ? null
                        : 'Saldo disponivel: ${fmt.format(balance)}',
                  ),
                  validator: (v) => (v == null || v.trim().isEmpty)
                      ? 'Informe o remetente'
                      : null,
                  onChanged: (_) => setState(() {}),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _receiverCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Para',
                    hintText: 'ex: bob',
                  ),
                  validator: (v) {
                    if (v == null || v.trim().isEmpty) return 'Informe o destino';
                    if (v.trim() == _senderCtrl.text.trim()) {
                      return 'Remetente e destino nao podem ser iguais';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _amountCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Valor',
                    prefixText: 'R\$ ',
                  ),
                  keyboardType: const TextInputType.numberWithOptions(
                    decimal: true,
                  ),
                  validator: (v) {
                    final n = double.tryParse(
                      (v ?? '').replaceAll(',', '.'),
                    );
                    if (n == null || n <= 0) return 'Valor invalido';
                    return null;
                  },
                ),
                const SizedBox(height: 16),
                FilledButton.icon(
                  onPressed: _busy ? null : _submit,
                  icon: const Icon(Icons.send),
                  label: Text(_busy ? 'Enfileirando...' : 'Enviar transacao'),
                ),
                if (_lastResult != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.surfaceContainerHigh,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Text(
                      _lastResult!,
                      style: const TextStyle(fontFamily: 'monospace'),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
