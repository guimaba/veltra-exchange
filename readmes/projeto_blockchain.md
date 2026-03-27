# Documentação Completa: Blockchain Distribuída com Algoritmo Bully

## Introdução
Este projeto foi desenvolvido como um MVP para a matéria de Sistemas Distribuídos. Ele implementa uma blockchain básica onde a liderança do cluster é decidida pelo Algoritmo de Eleição Bully.

## 1. Arquitetura e Organização de Pastas

A estrutura segue as melhores práticas de Go (Standard Layout), separando o que é código executável da lógica interna:

- **`blockchain_sistemasDistribuidos/`**: O diretório raiz do repositório.
  - **`cmd/node/main.go`**: O orquestrador. Aqui inicializamos as conexões, registramos o servidor RPC e começamos as threads de verificação (Heartbeat).
  - **`pkg/`**: Contém a lógica de negócio modularizada.
    - **`pkg/blockchain/`**: Gerencia blocos e a integridade da corrente. Sabe como minerar um bloco e como validar se o elo anterior (`PrevHash`) está correto.
    - **`pkg/bully/`**: Gerencia o estado do nó (Líder, rodando ou em eleição). É uma abstração do estado necessária para o Bully funcionar.
    - **`pkg/network/`**: Implementa a comunicação RPC. É onde as mensagens `ELECT` e `COORDINATOR` são processadas e enviadas para os outros nós.
  - **`simulate.ps1`**: Script PowerShell para lançar automaticamente 3 instâncias (nós) do sistema com IDs e portas diferentes.

---

## 2. Por que esta arquitetura?

- **Desacoplamento**: Se o algoritmo Bully precisar ser trocado por outro (como Raft ou Paxos), você só precisa mexer na pasta `bully` e `network`, sem quebrar a lógica da Blockchain em si.
- **RPC (Remote Procedure Call)**: Optamos pelo `net/rpc` nativo do Go por ser extremamente eficiente e direto para comunicações entre processos. Ele permite que chamemos funções em outro computador como se estivessem rodando localmente.
- **Mutex e Concorrência**: O Go possui "Goroutines" para realizar tarefas em paralelo (ouvir a rede e checar o líder ao mesmo tempo). Usamos `sync.Mutex` para garantir que essas tarefas paralelas não tentem mudar o estado da blockchain simultaneamente, o que causaria erros.

---

## 3. Explicação dos Componentes

### Blockchain (`pkg/blockchain`)
A blockchain aqui é um "Livro Razão" (Ledger) de transações.
1. **Blocos**: Cada bloco tem seu Hash calculado a partir de seus dados + o Hash do bloco anterior. Isso cria a "corrente" (chain).
2. **Imutabilidade**: Se o Nó 1 tentar alterar uma transação antiga, o Hash desse bloco vai mudar. Como o bloco seguinte guarda o Hash antigo, a corrente "quebra", e todos os outros nós rejeitam a mudança.

### Algoritmo Bully (`pkg/bully` & `pkg/network`)
O Algoritmo Bully (Valentão) serve para decidir quem é o coordenador. Ele funciona assim:
1. **Sempre o maior vence**: O nó com o maior ID numérico tem prioridade.
2. **Eleição**: Quando um nó percebe que o líder caiu (falha no Heartbeat), ele manda uma mensagem `ELECT` para todos que têm ID maior que o dele.
3. **Bullying**: Se houver alguém maior, esse nó responde "OK" (eu sou maior, deixa que eu assumo). Se ninguém responder, o nó que iniciou a eleição se proclama líder e manda `COORDINATOR` para todos.

---

## 4. O Coração do Sistema (`main.go`)

O processo funciona em três loops principais:
1. **RPC Listener**: Fica ouvindo na porta (ex: 8001) para receber pedidos de outros nós.
2. **Heartbeat Loop**: O nó checa se o líder ainda responde. Se não responder em 10 segundos, o caos é detectado e uma eleição começa.
3. **Blockchain Manager**: Garante que o estado local da blockchain esteja sincronizado com o que o líder propõe.

---

## 5. Como Testar
No terminal, execute o `simulate.ps1`. Ele abrirá três janelas.
- **Cena 1**: O Nó 3 será eleito (maior ID).
- **Cena 2**: Feche o terminal do Nó 3. Observe o Nó 2 percebendo que o líder sumiu e iniciando uma nova eleição para se tornar o novo líder.
- **Cena 3**: Reabra o Nó 3. Ele notará que é o maior e "expulsará" (bully) o Nó 2 da liderança.
