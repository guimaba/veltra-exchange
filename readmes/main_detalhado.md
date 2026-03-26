# Explicação Detalhada: Orquestrador (Main)

Este arquivo detalha o funcionamento do `cmd/node/main.go`, que une todas as peças.

---

## `cmd/node/main.go`

O `main` é onde a mágica acontece e o processo "ganha vida".

| Linha | Código | O que faz? | Por que usamos? |
| :--- | :--- | :--- | :--- |
| 18-21 | `flag.Int, flag.Parse` | Lê o terminal. | Usamos para não precisar criar 3 arquivos de código diferentes. O mesmo código se comporta como Nó 1, 2 ou 3 dependendo do que digitamos. |
| 49-53 | `go func() { rpc.Accept }` | Ouvinte paralelo. | Usamos `go` para que o servidor fique esperando chamadas em "segundo plano" enquanto o programa principal continua rodando a lógica de líder. |
| 56 | `time.Sleep(5s)` | Pausa inicial. | **Crucial:** Usamos para dar tempo de todos os processos PowerShell abrirem e ligarem seus servidores antes de tentarmos a primeira conexão. |
| 59-86 | `go func() { for { ... } }` | Loop infinito de checagem. | Usamos para que a verificação de saúde do líder seja eterna. Se o nó perder o contato por 1 segundo, ele ignora, mas se persistir, ele age. |
| 61 | `time.Sleep(10s)` | Intervalo de pulso. | Usamos para não sobrecarregar a rede. Pregar o líder a cada milissegundo seria desnecessário e lento. |

---

### Resumão do Main
O `main` inicializa a blockchain vazia, liga o "telefone" (servidor RPC) para receber mensagens e cria o "vigia" (monitor de heartbeat) que fica garantindo que o líder sempre esteja vivo. Se o vigia falha, ele chama a cavalaria (Algoritmo Bully).
