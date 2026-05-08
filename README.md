# Blockchain Distribuída + Mensageria com RabbitMQ

Projeto da disciplina de **Sistemas Distribuídos** (FURB). Implementa uma rede de **blockchain** em **Go** com:

- Eleição de líder via algoritmo **Bully**;
- Mineração com **Prova de Trabalho (PoW)** simplificada;
- Persistência em **MariaDB**;
- **Camada de mensageria assíncrona com RabbitMQ** (etapa de mensageria);
- **Interface gráfica Flutter Web** servida pelo gateway.

> ⚠️ **Camada financeira simulada.** O sistema demonstra mensageria distribuída sobre uma blockchain didática. A operação "adicionar crédito" injeta saldo virtual — **não há integração com pagamento real**, gateway financeiro, Pix, cartão ou qualquer infraestrutura monetária. O foco do trabalho é a comunicação assíncrona via RabbitMQ.

---

## 🚀 Como executar

### Pré-requisito único

Apenas **[Docker Desktop](https://www.docker.com/products/docker-desktop/)** instalado e rodando. Tudo o mais (Go, Flutter, RabbitMQ, MariaDB) roda em containers — não precisa instalar nada localmente.

### Subir o sistema (um comando)

**Windows / PowerShell:**
```powershell
.\start.ps1
```

**Linux / macOS / WSL:**
```bash
./start.sh
```

**Equivalente direto (sem o script):**
```bash
docker compose up --build
```

Na primeira execução, o build leva ~3-5 minutos (compila Go, faz build do Flutter Web, baixa imagens). Execuções seguintes são imediatas (cache do Docker).

### Acessar a aplicação

| URL | Conteúdo |
|---|---|
| <http://localhost:8080> | **Aplicação Flutter Web** (interface principal) |
| <http://localhost:8080/api> | API REST do gateway |
| ws://localhost:8080/ws | Stream de eventos em tempo real (WebSocket) |
| <http://localhost:15672> | Painel de management do RabbitMQ — usuário `admin`, senha `admin` |

### Parar o sistema

```bash
docker compose down
```

Para apagar também os volumes (dados do MariaDB):
```bash
.\start.ps1 -Clean   # Windows
./start.sh --clean   # Linux/macOS
```

---

## 📦 O que sobe quando você roda o comando

| Container | Imagem | Porta exposta | Função |
|---|---|---|---|
| `blockchain-rabbitmq` | rabbitmq:3.12-management | 5672, 15672 | Mensageria + UI de management |
| `blockchain-mariadb` | mariadb:10.11 | 3306 | Persistência |
| `blockchain-node1` | (build local) | – | Nó da blockchain (ID 1) |
| `blockchain-node2` | (build local) | – | Nó da blockchain (ID 2) |
| `blockchain-node3` | (build local) | – | Nó da blockchain (ID 3) — geralmente o líder |
| `blockchain-gateway` | (build local) | 8080 | API REST + WebSocket + Flutter Web |
| `blockchain-audit` | (build local) | – | Consumidor de auditoria |

A topologia do RabbitMQ (vhost, usuários, exchanges, filas, bindings) é importada **automaticamente** do arquivo [docker/rabbitmq/definitions.json](docker/rabbitmq/definitions.json) no startup. Não há configuração manual.

---

## 📚 Documentação do trabalho de mensageria

Toda a documentação das etapas exigidas pelo enunciado está em [documentacoes/mensageria/](documentacoes/mensageria/):

| Etapa | Arquivo |
|---|---|
| 1. Descrição do Cenário | [01_cenario.md](documentacoes/mensageria/01_cenario.md) |
| 2. Arquitetura da Solução | [02_arquitetura.md](documentacoes/mensageria/02_arquitetura.md) |
| Diagramas (componentes, fluxo, deployment, DLQ) | [03_diagramas.md](documentacoes/mensageria/03_diagramas.md) |
| 3. Configuração do RabbitMQ | [04_configuracao_rabbitmq.md](documentacoes/mensageria/04_configuracao_rabbitmq.md) |
| 4. Exemplos de Uso | [05_exemplos_uso.md](documentacoes/mensageria/05_exemplos_uso.md) |
| 5. Considerações Técnicas | [06_consideracoes_tecnicas.md](documentacoes/mensageria/06_consideracoes_tecnicas.md) |

Documentação do projeto base de blockchain está em [documentacoes/](documentacoes/).

---

## 🏗️ Arquitetura resumida

```
Browser (Flutter Web)
     │  HTTP REST + WebSocket
     ▼
Gateway (Go)  ─── publish ───▶  RabbitMQ  ───▶  Nó Líder ──┐
     ▲                              │                       │ minera
     │  WS push ◀── consume ────────┤                       ▼
     │                              ├──▶ Auditoria        Bloco novo
     │                              │                       │
     │                              └──▶ DLQ ◀──────────────┘
     │                                                       publica
     └──────────── eventos em tempo real ─────────────────── evento
```

- **HTTP REST** entre browser e gateway (comandos one-shot).
- **WebSocket** entre gateway e browser (push de eventos em tempo real).
- **AMQP/RabbitMQ** entre gateway, nós e auditoria (assíncrono, com DLQ e retry).
- **RPC** entre os nós da blockchain (consenso Bully + sincronização de blocos).

Detalhes completos em [02_arquitetura.md](documentacoes/mensageria/02_arquitetura.md).

---

## 🛠️ Troubleshooting

### "Port already in use" ao subir

Algum serviço local já está usando 8080, 5672, 15672 ou 3306. Pare-o ou edite as portas no [docker-compose.yml](docker-compose.yml).

### Build do Flutter falha

Confirme conectividade com a internet (a imagem `cirruslabs/flutter` baixa pacotes pub no build). Em redes corporativas com proxy, configure `HTTP_PROXY`/`HTTPS_PROXY` no Docker Desktop.

### Nó líder cai e ninguém assume

Aguarde alguns segundos — o Bully precisa do timeout do heartbeat antes de iniciar a eleição. Veja os logs: `docker compose logs -f node1 node2 node3`.

### Mensagens parando na DLQ

Acesse o painel RabbitMQ em <http://localhost:15672>, vá em **Queues → q.dlq** para inspecionar payloads e a razão da morte (`x-first-death-reason`).

### Resetar tudo

```bash
.\start.ps1 -Clean   # Windows
./start.sh --clean   # Linux/macOS
```

Apaga containers, redes e volumes (incluindo dados do MariaDB e mensagens persistidas no Rabbit).

---

## 📂 Estrutura do repositório

```
.
├── cmd/
│   ├── node/        # binário do nó da blockchain
│   ├── gateway/     # binário do gateway HTTP/WS (a ser implementado)
│   └── audit/       # binário do serviço de auditoria (a ser implementado)
├── pkg/
│   ├── blockchain/  # estruturas e lógica da blockchain
│   ├── bully/       # estado do nó e eleição
│   ├── network/     # RPC entre nós
│   ├── database/    # MariaDB
│   └── messaging/   # RabbitMQ (publisher/consumer) — a ser implementado
├── web/             # projeto Flutter Web (a ser implementado)
├── docker/
│   ├── Dockerfile.node
│   ├── Dockerfile.gateway
│   ├── Dockerfile.audit
│   ├── rabbitmq/
│   │   ├── definitions.json   # topologia AMQP declarativa
│   │   └── rabbitmq.conf
│   └── mariadb/
│       └── init.sql           # schema inicial
├── documentacoes/
│   ├── GUIA_ARQUITETURA.md    # arquitetura do projeto base
│   └── mensageria/            # docs do trabalho de mensageria
├── docker-compose.yml         # orquestração de tudo
├── start.ps1 / start.sh       # scripts de inicialização única
└── README.md                  # este arquivo
```

---

## 🎯 Algoritmo de Eleição Bully (resumo)

O algoritmo garante que o nó com o **maior ID disponível** sempre seja líder:

1. Se um nó detecta que o líder caiu (timeout de heartbeat), inicia uma eleição enviando mensagens para todos os nós com ID superior.
2. Se nenhum nó superior responder, o nó emissor se torna líder e avisa a rede.
3. Se um nó superior responder "OK", ele assume o processo de eleição.

Detalhes em [documentacoes/GUIA_ARQUITETURA.md](documentacoes/GUIA_ARQUITETURA.md).

---

## ⚠️ Aviso final

Este sistema é **estritamente acadêmico**. Não use em produção. As senhas estão em texto plano no `docker-compose.yml`, o broker não usa TLS localmente, e a "carteira" não tem qualquer ligação com sistemas financeiros reais.
