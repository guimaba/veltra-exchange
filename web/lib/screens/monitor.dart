import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';

import '../state.dart';

class MonitorScreen extends StatelessWidget {
  const MonitorScreen();

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final entries = state.eventLog;
    final fmt = DateFormat('HH:mm:ss');

    return Column(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          color: Theme.of(context).colorScheme.surfaceContainerHigh,
          child: Row(
            children: [
              const Icon(Icons.bolt_outlined, size: 18),
              const SizedBox(width: 6),
              Text(
                'Monitor de eventos em tempo real (WebSocket)',
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const Spacer(),
              Text(
                '${entries.length} evento${entries.length == 1 ? '' : 's'}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
          ),
        ),
        Expanded(
          child: entries.isEmpty
              ? const Center(
                  child: Text(
                    'Aguardando eventos...',
                    style: TextStyle(fontStyle: FontStyle.italic),
                  ),
                )
              : ListView.separated(
                  itemCount: entries.length,
                  separatorBuilder: (_, __) => const Divider(height: 1),
                  itemBuilder: (context, i) {
                    final e = entries[i];
                    return _EventTile(
                      time: fmt.format(e.when),
                      type: e.type,
                      data: e.data,
                    );
                  },
                ),
        ),
      ],
    );
  }
}

class _EventTile extends StatelessWidget {
  final String time;
  final String type;
  final Map<String, dynamic> data;
  const _EventTile({
    required this.time,
    required this.type,
    required this.data,
  });

  Color _colorFor(BuildContext ctx) {
    final cs = Theme.of(ctx).colorScheme;
    if (type.startsWith('block')) return Colors.lightBlueAccent;
    if (type.startsWith('credit')) return Colors.greenAccent;
    if (type.contains('rejected')) return Colors.redAccent;
    if (type.contains('leader')) return Colors.amberAccent;
    return cs.primary;
  }

  @override
  Widget build(BuildContext context) {
    final color = _colorFor(context);
    return ExpansionTile(
      leading: Container(
        width: 8,
        height: double.infinity,
        color: color,
      ),
      title: Row(
        children: [
          Text(
            time,
            style: const TextStyle(
              fontFamily: 'monospace',
              fontSize: 12,
            ),
          ),
          const SizedBox(width: 12),
          Text(
            type,
            style: TextStyle(
              fontWeight: FontWeight.bold,
              color: color,
            ),
          ),
        ],
      ),
      childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHigh,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            const JsonEncoder.withIndent('  ').convert(data),
            style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
          ),
        ),
      ],
    );
  }
}
