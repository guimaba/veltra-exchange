# Explicação Detalhada: Algoritmo Bully e Rede (RPC)

Este arquivo detalha o funcionamento dos arquivos `pkg/bully/node.go` e `pkg/network/rpc.go`.

---

## 1. `pkg/bully/node.go`

Este arquivo gerencia o "cérebro" de cada nó individualmente.

| Linha | Código | O que faz? | Por que usamos? |
| :--- | :--- | :--- | :--- |
| 10-14 | `StateRunning, Election, Leader` | Estados do nó. | Usamos para que o nó saiba como agir. Um nó em `ELECTION` ignora pedidos de transação para focar em encontrar o novo chefe. |
| 22 | `RWMutex` | Trava de Read/Write. | Permite que vários processos leiam quem é o líder ao mesmo tempo, mas trava tudo se um deles precisar trocar o líder. Melhora a performance. |
| 64-72 | `GetPeersHigherThan` | Filtra IDs maiores. | O coração do Bully. Usamos porque o algoritmo dita que você só pode ser "mandado" por quem é maior. É a regra de hierarquia. |

---

## 2. `pkg/network/rpc.go`

| Linha | Código | O que faz? | Por que usamos? |
| :--- | :--- | :--- | :--- |
| 24-30 | `Elect` (RPC) | Recebe desafio. | É o canal de entrada dos desafios. Usamos para que o nó saiba que existem outros competidores ativos na rede. |
| 28 | `go StartElection` | Inicia eleição em paralelo. | Usamos `go` (goroutine) para que o nó responda ao desafiante imediatamente, sem ficar travado esperando o fim da sua própria votação. |
| 61 | `Heartbeat` | Responde "estou aqui". | Usamos para que os outros nós não iniciem eleições desnecessárias. É a prova de vida do líder. |
| 77 | `for _, port := range` | Loop de chamadas. | Usamos o `_` para ignorar o ID (que não é necessário para o comando de rede), focando apenas na porta. Isso evita erros de compilação por "variável não utilizada". |
| 78 | `rpc.Dial` | Telefona para o par. | Abre o canal de comunicação. Usamos o protocolo TCP por ser confiável (garante que a mensagem chegue ou avise o erro). |

---

### Resumão da Rede
O sistema usa RPC para que um código possa chamar uma função em outro computador como se fosse local. O Bully é uma disputa de "quem tem o ID maior" que acontece via essas chamadas de rede sempre que o líder atual desaparece.
