package matching

// Write-Ahead Log + snapshots para o matching engine (plano tecnico secao
// 4.2.2): "cada evento e gravado no WAL antes de confirmado; snapshots
// periodicos do book limitam o replay necessario apos uma falha. Recuperacao
// sem perda de trades confirmados."
//
// Fluxo de uso no servico:
//  1. OpenJournal -> reconstroi o Engine do ultimo snapshot + replay do WAL.
//  2. Para cada comando sequenciado: Journal.Append(cmd) ANTES de Engine.Apply.
//  3. Periodicamente: Journal.Snapshot(engine, lastSeq) -> trunca o WAL.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
)

// CmdKind distingue os comandos registrados no WAL.
type CmdKind string

const (
	CmdPlace  CmdKind = "place"
	CmdCancel CmdKind = "cancel"
)

// Command e um registro do WAL: uma operacao sequenciada a ser aplicada ao
// motor. Seq e a sequencia global atribuida pelo sequenciador (monotonica).
type Command struct {
	Seq     uint64          `json:"seq"`
	Kind    CmdKind         `json:"kind"`
	Order   *exchange.Order `json:"order,omitempty"`    // para CmdPlace
	OrderID string          `json:"order_id,omitempty"` // para CmdCancel
}

// snapshotFile e o formato persistido de um snapshot do motor.
type snapshotFile struct {
	Pair     exchange.Pair     `json:"pair"`
	LastSeq  uint64            `json:"last_seq"`  // ultima Seq de comando aplicada
	TradeSeq uint64            `json:"trade_seq"` // contador de trades do par
	Orders   []*exchange.Order `json:"orders"`    // ordens em repouso, em ordem de prioridade
}

// Journal gerencia o par de arquivos (WAL + snapshot) de um motor em um diretorio.
type Journal struct {
	pair     exchange.Pair
	walPath  string
	snapPath string

	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

// OpenJournal abre (criando se preciso) o journal do par em dir, reconstroi o
// Engine a partir do ultimo snapshot e do replay do WAL, e deixa o WAL pronto
// para append. Retorna o motor recuperado e a ultima Seq aplicada.
func OpenJournal(dir string, pair exchange.Pair) (*Journal, *Engine, uint64, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, 0, err
	}
	safe := safeName(pair)
	j := &Journal{
		pair:     pair,
		walPath:  filepath.Join(dir, safe+".wal"),
		snapPath: filepath.Join(dir, safe+".snapshot"),
	}

	engine := NewEngine(pair)
	var lastSeq uint64

	// 1) Carrega snapshot, se houver.
	snap, ok, err := readSnapshot(j.snapPath)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("ler snapshot: %w", err)
	}
	if ok {
		engine.Restore(snap.Orders, snap.TradeSeq)
		lastSeq = snap.LastSeq
	}

	// 2) Replay do WAL: aplica apenas comandos com Seq > lastSeq.
	cmds, err := readWAL(j.walPath)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("ler WAL: %w", err)
	}
	for _, c := range cmds {
		if c.Seq <= lastSeq {
			continue
		}
		engine.Apply(c)
		lastSeq = c.Seq
	}

	// 3) Abre o WAL para append.
	f, err := os.OpenFile(j.walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, 0, err
	}
	j.f = f
	j.enc = json.NewEncoder(f)
	return j, engine, lastSeq, nil
}

// Append grava o comando no WAL e forca o flush para disco (fsync), garantindo
// durabilidade antes de aplicar ao motor.
func (j *Journal) Append(cmd Command) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.enc.Encode(cmd); err != nil {
		return err
	}
	return j.f.Sync()
}

// Snapshot persiste o estado atual do motor e trunca o WAL (compactacao).
func (j *Journal) Snapshot(e *Engine, lastSeq uint64) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	snap := snapshotFile{
		Pair:     j.pair,
		LastSeq:  lastSeq,
		TradeSeq: e.TradeSeq(),
		Orders:   e.RestingOrders(),
	}
	// Escreve em arquivo temporario e renomeia (atomico).
	tmp := j.snapPath + ".tmp"
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, j.snapPath); err != nil {
		return err
	}

	// Trunca o WAL: tudo ate lastSeq ja esta no snapshot. No Windows nao se pode
	// truncar um handle aberto em modo append, entao fechamos, truncamos pelo
	// path e reabrimos.
	if err := j.f.Close(); err != nil {
		return err
	}
	if err := os.Truncate(j.walPath, 0); err != nil {
		return err
	}
	f, err := os.OpenFile(j.walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	j.f = f
	j.enc = json.NewEncoder(f)
	return j.f.Sync()
}

// Close fecha o WAL.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f != nil {
		return j.f.Close()
	}
	return nil
}

// Apply aplica um comando do WAL ao motor (usado no replay). Diferente de
// Process, descarta o Result: o replay so reconstroi estado.
func (e *Engine) Apply(cmd Command) Result {
	switch cmd.Kind {
	case CmdPlace:
		if cmd.Order != nil {
			// Reinicia campos voláteis para reprocessar do zero de forma identica.
			cmd.Order.Filled = 0
			cmd.Order.Status = exchange.StatusNew
			return e.Process(cmd.Order)
		}
	case CmdCancel:
		e.Cancel(cmd.OrderID)
	}
	return Result{}
}

// ----- I/O de baixo nivel -----

func readWAL(path string) ([]Command, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var cmds []Command
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var c Command
		if err := json.Unmarshal(line, &c); err != nil {
			// Linha corrompida (provavel crash no meio do append): para o replay
			// aqui. Tudo antes ja e durabilidade confirmada.
			break
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func readSnapshot(path string) (snapshotFile, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshotFile{}, false, nil
		}
		return snapshotFile{}, false, err
	}
	var s snapshotFile
	if err := json.Unmarshal(data, &s); err != nil {
		return snapshotFile{}, false, err
	}
	return s, true, nil
}

func safeName(p exchange.Pair) string {
	return strings.NewReplacer("/", "_", " ", "_").Replace(p.String())
}
