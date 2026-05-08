import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../state.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen();

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final blocks = state.blocks.reversed.toList();

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                _Stat(
                  label: 'Blocos minerados',
                  value: '${state.blocks.length}',
                  icon: Icons.layers_outlined,
                ),
                _Stat(
                  label: 'Lider atual',
                  value: state.leader == -1 ? '—' : 'No ${state.leader}',
                  icon: Icons.flag_outlined,
                ),
                _Stat(
                  label: 'Contas',
                  value: '${state.balances.length}',
                  icon: Icons.people_outline,
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          'Ultimos blocos',
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 8),
        if (blocks.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: Text(
                'Sem blocos minerados ainda. Envie uma transacao para iniciar.',
                style: TextStyle(fontStyle: FontStyle.italic),
              ),
            ),
          )
        else
          ...blocks.map((b) => _BlockCard(block: b)),
      ],
    );
  }
}

class _Stat extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  const _Stat({required this.label, required this.value, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        children: [
          Icon(icon, size: 28),
          const SizedBox(height: 4),
          Text(value, style: Theme.of(context).textTheme.titleLarge),
          Text(label, style: Theme.of(context).textTheme.bodySmall),
        ],
      ),
    );
  }
}

class _BlockCard extends StatelessWidget {
  final BlockSummary block;
  const _BlockCard({required this.block});

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 6),
      child: ExpansionTile(
        leading: CircleAvatar(child: Text('#${block.index}')),
        title: Text('Bloco ${block.index}'),
        subtitle: Text(
          'Hash: ${_short(block.hash)} • Nonce ${block.nonce} • Minerador No ${block.minerNodeId}',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _kv('Hash', block.hash),
                _kv('PrevHash', block.prevHash),
                const Divider(),
                Text(
                  'Transacoes (${block.transactions.length})',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 4),
                if (block.transactions.isEmpty)
                  const Text('— sem transacoes —')
                else
                  ...block.transactions.map(_txRow),
              ],
            ),
          ),
        ],
      ),
    );
  }

  static Widget _kv(String k, String v) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 2),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: 90,
              child: Text(k, style: const TextStyle(fontWeight: FontWeight.bold)),
            ),
            Expanded(
              child: Text(v, style: const TextStyle(fontFamily: 'monospace')),
            ),
          ],
        ),
      );

  static Widget _txRow(Map<String, dynamic> tx) {
    final kind = tx['kind'] ?? '';
    final sender = (tx['sender'] ?? '') as String;
    final receiver = (tx['receiver'] ?? '') as String;
    final amount = tx['amount'];
    final desc = kind == 'credit'
        ? 'CREDITO -> $receiver: R\$ $amount'
        : '$sender -> $receiver: R\$ $amount';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Text('• $desc', style: const TextStyle(fontFamily: 'monospace')),
    );
  }

  static String _short(String h) =>
      h.length > 16 ? '${h.substring(0, 8)}...${h.substring(h.length - 8)}' : h;
}
