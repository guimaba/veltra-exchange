# Trabalho Prático — Mensageria com RabbitMQ

Documentação completa do trabalho prático de mensageria, construído sobre o projeto base de blockchain distribuída.

## Índice

| Documento | Conteúdo |
|---|---|
| **[RELATORIO_FINAL.md](./RELATORIO_FINAL.md)** | **Relatório final consolidado — leitura recomendada** |
| [01_cenario.md](./01_cenario.md) | Etapa 1 — Descrição do Cenário |
| [02_arquitetura.md](./02_arquitetura.md) | Etapa 2 — Arquitetura da Solução |
| [03_diagramas.md](./03_diagramas.md) | Diagramas (componentes, sequência, deployment, DLQ) |
| [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) | Etapa 3 — Configuração do RabbitMQ |
| [05_exemplos_uso.md](./05_exemplos_uso.md) | Etapa 4 — Exemplos de Uso |
| [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) | Etapa 5 — Considerações Técnicas |
| [07_roteiro_testes.md](./07_roteiro_testes.md) | Roteiro de testes ponta-a-ponta |

## Escopo

> ⚠️ **Camada financeira simulada.** O sistema demonstra a arquitetura de mensageria sobre uma blockchain didática. A operação "adicionar crédito" é um endpoint que injeta saldo virtual — **não há integração com pagamento real, gateway financeiro, Pix, cartão ou qualquer infraestrutura monetária**. O foco do trabalho é a comunicação assíncrona via RabbitMQ.

## Como executar

Ver instruções no [README principal do projeto](../../README.md).
