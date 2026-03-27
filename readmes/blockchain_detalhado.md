# Explicação Detalhada: Core da Blockchain

Este arquivo detalha o funcionamento dos arquivos `pkg/blockchain/block.go` e `pkg/blockchain/blockchain.go`.

---

## 1. `pkg/blockchain/block.go`

Este arquivo define a estrutura básica do dado que compõe a nossa rede.

| Linha | Código | O que faz? | Por que usamos? |
| :--- | :--- | :--- | :--- |
| 10-14 | `Transaction` | Define a estrutura da transferência. | É a nossa "Moeda". Sem isso, a blockchain não teria utilidade prática. |
| 16-23 | `Block` | Define o container de dados. | Precisamos agrupar transações para que o Hash seja calculado sobre um conjunto, garantindo a integridade do lote. |
| 36-42 | `CalculateHash` | Gera o SHA-256 do bloco. | O SHA-256 é um padrão de segurança. Usamos para que qualquer alteração mínima invalide o bloco, mantendo a rede honesta. |
| 44-56 | `Mine` | Loop de Nonce e Zeros. | Usamos para evitar que um atacante crie milhões de blocos falsos por segundo. Isso dá "custo" à criação de blocos (Consenso). |

---

## 2. `pkg/blockchain/blockchain.go`

| Linha | Código | O que faz? | Por que usamos? |
| :--- | :--- | :--- | :--- |
| 10 | `Mutex` | Trava de segurança. | Essencial em sistemas distribuídos. Sem o Mutex, dois processos tentando escrever no arquivo ao mesmo tempo causariam "Race Conditions" e corromperiam a corrente. |
| 14 | `GenesisBlock` | Cria o primeiro bloco. | Toda blockchain precisa de um ponto de partida fixo e conhecido por todos os nós para que todos concordem com o histórico. |
| 37-50 | `IsValid` | Validação de histórico. | É o que dá segurança à rede. Usamos para detectar se algum nó está tentando enviar uma versão alterada ou falsa da blockchain. |

---

### Resumão do Core
A blockchain é basicamente uma lista onde cada item aponta para o anterior via um código matemático (Hash). Se alguém mexer em um bloco antigo, todos os hashes seguintes "quebram", denunciando a fraude.
