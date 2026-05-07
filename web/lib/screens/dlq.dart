import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

Future<void> openUrl(String url) async {
  final uri = Uri.parse(url);
  await launchUrl(uri, mode: LaunchMode.externalApplication);
}

class DlqScreen extends StatelessWidget {
  const DlqScreen();

  String _rabbitHost() {
    final host = Uri.base.host;
    return 'http://$host:15672';
  }

  @override
  Widget build(BuildContext context) {
    final rabbit = _rabbitHost();
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const Icon(Icons.report_outlined, color: Colors.orangeAccent),
                      const SizedBox(width: 8),
                      Text(
                        'Dead Letter Queue (q.dlq)',
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  const Text(
                    'Mensagens que esgotaram o pipeline de retry (3 tentativas em '
                    'blockchain.retry, com backoff de 5s) ou que sao permanentemente '
                    'invalidas (JSON mal-formado, schema desconhecido) sao roteadas '
                    'pelo blockchain.dlx (fanout) ate q.dlq.',
                  ),
                  const SizedBox(height: 12),
                  const Text(
                    'A inspecao detalhada (browse de payload, headers x-first-death-* '
                    'com a causa da morte) fica no painel de management do RabbitMQ.',
                  ),
                  const SizedBox(height: 16),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      FilledButton.icon(
                        icon: const Icon(Icons.open_in_new),
                        label: const Text('Abrir painel RabbitMQ'),
                        onPressed: () => openUrl(rabbit),
                      ),
                      OutlinedButton.icon(
                        icon: const Icon(Icons.dataset_outlined),
                        label: const Text('Ir direto para q.dlq'),
                        onPressed: () =>
                            openUrl('$rabbit/#/queues/%2Fblockchain/q.dlq'),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Login padrao: admin / admin',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Como uma mensagem cai aqui',
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  const SizedBox(height: 8),
                  const _Step(
                    n: 1,
                    text: 'Consumer (no lider) recebe a mensagem.',
                  ),
                  const _Step(
                    n: 2,
                    text:
                        'Erro transitorio (DB fora, rede): consumer republica em '
                        'blockchain.retry incrementando x-retry-count.',
                  ),
                  const _Step(
                    n: 3,
                    text:
                        'Apos 3 tentativas, o consumer publica direto em '
                        'blockchain.dlx -> q.dlq.',
                  ),
                  const _Step(
                    n: 4,
                    text:
                        'Erro permanente (JSON invalido, routing desconhecida) '
                        'vai direto para a DLQ, sem passar por retry.',
                  ),
                  const _Step(
                    n: 5,
                    text:
                        'Erros de negocio (saldo insuficiente, conta invalida) NAO '
                        'vao para a DLQ - viram um evento *.rejected normal.',
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Step extends StatelessWidget {
  final int n;
  final String text;
  const _Step({required this.n, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          CircleAvatar(radius: 12, child: Text('$n', style: const TextStyle(fontSize: 12))),
          const SizedBox(width: 8),
          Expanded(child: Text(text)),
        ],
      ),
    );
  }
}
