# Trabalho Prático — Mensageria com RabbitMQ

Documentação do trabalho prático de mensageria, construído sobre o projeto base de blockchain distribuída.

## Índice

| Etapa | Documento | Pontuação | Prazo |
|---|---|---|---|
| 1. Descrição do Cenário | [01_cenario.md](./01_cenario.md) | parte dos 2,5 pts | 08/05 |
| 2. Arquitetura da Solução | [02_arquitetura.md](./02_arquitetura.md) | parte dos 2,5 pts | 08/05 |
| Diagramas (1 e 2) | [03_diagramas.md](./03_diagramas.md) | parte dos 2,5 pts | 08/05 |
| 3. Configuração RabbitMQ | [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) | parte dos 3,5 pts | 08/05 |
| 4. Exemplos de Uso | [05_exemplos_uso.md](./05_exemplos_uso.md) | parte dos 3,5 pts | 08/05 |
| 5. Considerações Técnicas | [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) | parte dos 3,5 pts | 08/05 |
| Relatório Final | _(a fazer)_ | 2,5 pts | 08/05 |

## Escopo

> ⚠️ **Camada financeira simulada.** O sistema demonstra a arquitetura de mensageria sobre uma blockchain didática. A operação "adicionar crédito" é um endpoint que injeta saldo virtual — **não há integração com pagamento real, gateway financeiro, Pix, cartão ou qualquer infraestrutura monetária**. O foco do trabalho é a comunicação assíncrona via RabbitMQ.

## Como executar

Instruções de execução única (`docker-compose up`) estarão no [README principal do projeto](../../README.md) após a implementação das Etapas 3-5.
