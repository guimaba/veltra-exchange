# Guia de Arquitetura do Sistema

Este documento fornece um panorama técnico do funcionamento interno da Blockchain Distribuída. Ele complementa o `README.md` principal, focando em "Como as peças se encaixam" em nível de código (evitando detalhamentos linha a linha que ficam obsoletos rapidamente).

## 1. O Ponto de Entrada (`cmd/node/main.go`)
O `main` orquestra a subida do Nó na rede:
- **MariaDB:** Inicializa a conexão com o banco de dados. Se falhar, o nó funciona em memória (ideal para o MVP, porém perde a persistência real dos blocos).
- **Servidor RPC:** Mantém a porta TCP aberta em uma `Goroutine` (background) para receber blocos, transações e mensagens de eleição de outros nós sem travar o processamento local.
- **Heartbeat (Pulso de Vida):** Um loop infinito que a cada 10 segundos dispara um `Ping` para o Líder atual. Se o líder não responder, a eleição é iniciada.

## 2. O Core da Blockchain e Mempool (`pkg/blockchain`)
A blockchain é um Livro Razão (Ledger) imutável voltado para transações.
- **Transações e MemPool:** As transações (`Transaction`) chegam pela rede e são alocadas localmente na fila pendente apontada na estrutura da `Blockchain` (conhecida como MemPool). O uso de um `Mutex` garante que duas transações não sejam escritas ao mesmo tempo corrompendo o slice.
- **Mineração e PoW:** O sistema usa o método `MinePendingTransactions(difficulty)` para aplicar a **Prova de Trabalho (PoW)**. O Líder realiza um ataque de força bruta no próprio bloco alterando a variável `Nonce` até criar um Hash (`SHA-256`) que inicie com certa quantidade de zeros (Dificuldade).
  > ⚠️ **Aviso de Desempenho (Mineração):**
  > O processo de mineração (Prova de Trabalho) normalmente consome 100% da CPU em redes reais como o Bitcoin. Para que **não haja nenhum risco à sua máquina** de superaquecimento ou travamentos, o código foi calibrado com uma *Dificuldade Fixa Baixíssima* de nível `3`. Isso significa que a mineração ocorre em frações de segundo e não sobrecarrega seu processador, sendo totalmente segura para rodar localmente no seu notebook/PC durante a simulação!
- **Imutabilidade:** Qualquer byte adulterado em uma transação passada irá quebrar a função `IsValid()` em cascata de todos os hashes dos blocos subsequentes, denunciando a falsificação para a rede.

## 3. Algoritmo de Consenso Bully (`pkg/bully`)
Esse algoritmo de nome divertido ("Valentão") dita as regras de como a rede escolhe o Líder (Coordenador) que terá o direito exclusivo de minerar os blocos oficiais.
- **A Regra Primordial:** O nó ativo que tiver o maior ID numérico sempre prevalece e ganha.
- **Processo de Eleição:** 
  1. Quando um nó comum não acha o líder no `Heartbeat`, ele grita `ELECT` na rede.
  2. Ele só envia essa mensagem para IDs *Maiores* que o dele.
  3. Se algum nó maior estiver online, ele responde "OK, eu assumo" e instaura uma nova eleição a partir dele.
  4. Se o "valentão supremo" (maior ID) cair, o segundo maior não receberá respostas e, portanto, se autoproclamará Líder através da mensagem global de `COORDINATOR`.
- **Controle de Estado:** O nó mantém seu estado controlado por `RWMutex` para que todos esses processos de rede não misturem os ciclos de vida.

## 4. Comunicação em Malha Direta (`pkg/network`)
Em vez de depender de servidores HTTP densos e lentos (REST), nossa blockchain confia no Go RPC (`net/rpc`).
- **Comunicação Nativa:** Permite que o Nó 1 execute o método exportado `ReceiveTransaction` ou `Elect` nativamente no Node 2.
- **Retransmissão de Blocos:** Assim que o Coordenador finaliza o processo intenso do `MinePendingTransactions`, ele dispara em loop a goroutine iterando sobre seu `h.Node.Peers` para forçar que todo o resto da rede receba (`ReceiveBlock`) e atualize seu histórico imutável com o novo Bloco Válido.
